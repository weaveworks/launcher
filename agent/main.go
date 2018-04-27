package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	raven "github.com/getsentry/raven-go"
	"github.com/oklog/run"
	log "github.com/sirupsen/logrus"

	apiv1 "k8s.io/api/core/v1"
	kubeclient "k8s.io/client-go/kubernetes"

	"github.com/weaveworks/launcher/pkg/k8s"
	"github.com/weaveworks/launcher/pkg/kubectl"
	"github.com/weaveworks/launcher/pkg/sentry"
	"github.com/weaveworks/launcher/pkg/text"
	"github.com/weaveworks/launcher/pkg/weavecloud"
)

const (
	defaultAgentPollURL      = "https://get.weave.works/k8s/agent.yaml?instanceID={{.InstanceID}}"
	defaultAgentRecoveryWait = 5 * time.Minute
	defaultWCHostname        = "cloud.weave.works"
	defaultWCPollURL         = "https://{{.WCHostname}}/k8s.yaml" +
		"?k8s-version={{.KubernetesVersion}}&t={{.Token}}&omit-support-info=true" +
		"{{if .FluxConfig}}" +
		"&git-label={{.FluxConfig.GitLabel}}&git-url={{.FluxConfig.GitURL}}" +
		"&git-path={{.FluxConfig.GitPath}}&git-branch={{.FluxConfig.GitBranch}}" +
		"{{end}}"
)

type agentConfig struct {
	KubernetesVersion    string
	WCHostname           string
	Token                string
	InstanceID           string
	AgentPollURLTemplate string
	AgentRecoveryWait    time.Duration
	WCPollURLTemplate    string
	KubeClient           *kubeclient.Clientset
	KubectlClient        kubectl.Client
	FluxConfig           *FluxConfig
}

func init() {
	// https://sentry.io/weaveworks/launcher-agent/
	raven.SetDSN("https://a31e98421db8457a8c85fb42afcfc6fa:ec43815dbf4e440ca69f53b683bb81da@sentry.io/278297")
}

func setLogLevel(logLevel string) error {
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("error parsing log level: %v", err)
	}
	log.SetLevel(level)
	return nil
}

func logError(msg string, err error, cfg *agentConfig) {
	formatted := fmt.Sprintf("%s: %s", msg, err)
	log.Error(formatted)

	sentry.CaptureAndWait(1, formatted, nil)
}

func updateAgents(cfg *agentConfig, cancel <-chan interface{}) error {
	// Self-update
	agentPollURL, err := text.ResolveString(cfg.AgentPollURLTemplate, cfg)
	if err != nil {
		log.Fatal("invalid URL template:", err)
	}
	log.Info("Updating self from ", agentPollURL)

	initialRevision, err := k8s.GetLatestDeploymentReplicaSetRevision(cfg.KubeClient, "weave", "weave-agent")
	if err != nil {
		logError("Failed to fetch latest deployment replicateset revision", err, cfg)
		return nil
	}
	log.Info("Revision before self-update: ", initialRevision)
	err = kubectl.Apply(cfg.KubectlClient, agentPollURL)
	if err != nil {
		// We exit so that kubernetes can take care of restarting the pod and with that retrying to apply the file.
		logError("Failed to execute kubectl apply", err, cfg)
		return err
	}
	updatedRevision, err := k8s.GetLatestDeploymentReplicaSetRevision(cfg.KubeClient, "weave", "weave-agent")
	if err != nil {
		logError("Failed to fetch latest deployment replicateset revision", err, cfg)
		return nil
	}
	log.Info("Revision after self-update: ", updatedRevision)

	// If the agent replica set is updating, we will be killed via SIGTERM.
	// The agent uses a RollingUpdate strategy, so we are only killed when the
	// new agent is ready. If we are not killed after 5 minutes we assume that
	// the new agent did not become ready and so we recover by rolling back.
	if updatedRevision > initialRevision {
		log.Infof("The agent replica set updated. Rollback if we are not killed within %s...", cfg.AgentRecoveryWait)

		select {
		case <-time.After(cfg.AgentRecoveryWait):
		case <-cancel:
			return nil
		}

		logError("Deployment of the new agent failed. Rolling back.", errors.New("Deployment failed"), cfg)
		_, err := cfg.KubectlClient.Execute("rollout", "undo", "--namespace=weave", "deployment/weave-agent")
		if err != nil {
			logError("Failed rolling back agent. Will continue to check for updates.", err, cfg)
			return nil
		}

		// Return so we continue updating the agent until success
		logError("The new agent was rolled back.", errors.New("Rollback success"), cfg)
		return nil
	}

	// Get existing flux config
	fluxCfg, err := getFluxConfig(cfg.KubectlClient, "weave")
	if err != nil {
		logError("Failed getting existing flux config", err, cfg)
	}
	if fluxCfg != nil {
		cfg.FluxConfig = fluxCfg
	}

	// Update Weave Cloud agents
	wcPollURL, err := text.ResolveString(cfg.WCPollURLTemplate, cfg)
	if err != nil {
		log.Fatal("invalid URL template:", err)
	}
	log.Info("Updating WC from ", wcPollURL)
	err = kubectl.Apply(cfg.KubectlClient, wcPollURL)
	if err != nil {
		logError("Failed to execute kubectl apply", err, cfg)
		return nil
	}

	return nil
}

func setupKubeClient() (*kubeclient.Clientset, error) {
	kubeConfig, err := k8s.GetClientConfig(&k8s.ClientConfig{
		// We have seen quite a few clusters in the wild with invalid certificates.
		// Disable checking certificates as a result.
		Insecure: true,
	})
	if err != nil {
		return nil, fmt.Errorf("client config: %s", err)
	}
	return kubeclient.NewForConfig(kubeConfig)
}

func main() {
	raven.CapturePanicAndWait(mainImpl, nil)
}

func mainImpl() {
	logLevel := flag.String("log.level", "info", "verbosity of log output - one of 'debug', 'info' (default), 'warning', 'error', 'fatal'")

	agentPollURLTemplate := flag.String("agent.poll-url", defaultAgentPollURL, "URL to poll for the agent manifest")
	agentRecoveryWait := flag.Duration("agent.recovery-wait", defaultAgentRecoveryWait, "Duration to wait before recovering from a failed self update")
	wcToken := flag.String("wc.token", "", "Weave Cloud instance token")
	wcPollInterval := flag.Duration("wc.poll-interval", 1*time.Hour, "Polling interval to check WC manifests")
	wcPollURLTemplate := flag.String("wc.poll-url", defaultWCPollURL, "URL to poll for WC manifests")
	wcOrgLookupURLTemplate := flag.String("wc.org-lookup-url", weavecloud.DefaultWCOrgLookupURLTemplate, "URL to lookup org external ID by token")
	wcHostname := flag.String("wc.hostname", defaultWCHostname, "WC Hostname for WC agents and users API")

	eventsReportInterval := flag.Duration("events.report-interval", 3*time.Second, "Minimal time interval between two reports")

	featureInstall := flag.Bool("feature.install-agents", true, "Whether the agent should install anything in the cluster or not")
	featureEvents := flag.Bool("feature.kubernetes-events", false, "Whether the agent should forward kubernetes events to Weave Cloud or not")

	flag.Parse()

	if err := setLogLevel(*logLevel); err != nil {
		log.Fatal(err)
	}

	if *wcToken == "" {
		log.Fatal("missing Weave Cloud instance token, provide one with -wc.token")
	}

	cfg := &agentConfig{
		Token:                *wcToken,
		AgentRecoveryWait:    *agentRecoveryWait,
		KubectlClient:        kubectl.LocalClient{},
		WCHostname:           *wcHostname,
		AgentPollURLTemplate: *agentPollURLTemplate,
		WCPollURLTemplate:    *wcPollURLTemplate,
	}

	kubeClient, err := setupKubeClient()
	if err != nil {
		logError("kubernetes client", err, cfg)
		os.Exit(1)
	}
	cfg.KubeClient = kubeClient

	version, err := kubeClient.Discovery().ServerVersion()
	if err != nil {
		logError("get server version", err, cfg)
		os.Exit(1)
	}
	cfg.KubernetesVersion = version.GitVersion
	raven.SetTagsContext(map[string]string{
		"kubernetes": cfg.KubernetesVersion,
	})

	// Lookup instance ID
	wcOrgLookupURL, err := text.ResolveString(*wcOrgLookupURLTemplate, cfg)
	if err != nil {
		log.Fatal("invalid URL template:", err)
	}
	instanceID, _, err := weavecloud.LookupInstanceByToken(wcOrgLookupURL, *wcToken)
	if err != nil {
		logError("lookup instance by token", err, &agentConfig{})
	} else {
		cfg.InstanceID = instanceID
		raven.SetTagsContext(map[string]string{
			"instance": cfg.InstanceID,
		})
	}

	// Migrate kube system and reuse any existing flux config
	existingFluxCfg := migrateKubeSystem(cfg.KubectlClient)
	if existingFluxCfg != nil {
		log.Infof("Using existing flux config: %+v", existingFluxCfg)
	}
	cfg.FluxConfig = existingFluxCfg

	var g run.Group

	cancel := make(chan interface{})
	// Poll for new manifests every wcPollInterval.
	if *featureInstall {

		g.Add(
			func() error {
				for {

					err := updateAgents(cfg, cancel)
					if err != nil {
						return err
					}

					select {
					case <-time.After(*wcPollInterval):
						continue
					case <-cancel:
						return nil
					}
				}
			},
			func(err error) {
				close(cancel)
			},
		)
	}

	// Close gracefully on SIGTERM
	{
		term := make(chan os.Signal)
		signal.Notify(term, os.Interrupt, syscall.SIGTERM)
		g.Add(
			func() error {
				<-term
				log.Info("received SIGTERM")
				return nil
			},
			func(err error) {
				close(cancel)
			},
		)
	}

	eventSource := k8s.NewEventSource(kubeClient, apiv1.NamespaceAll)

	// Capture Kubernetes events
	if *featureEvents {
		cancel := make(chan interface{})
		g.Add(
			func() error {
				eventSource.Start(cancel)
				return nil
			},
			func(err error) {
				close(cancel)
			},
		)
	}

	// Report Kubernetes events
	if *featureEvents {
		g.Add(
			func() error {
				for {
					select {
					case <-time.After(*eventsReportInterval):
						events := eventSource.GetNewEvents()
						for _, event := range events {
							log.WithFields(log.Fields{
								"name": event.InvolvedObject.Name,
								"kind": event.InvolvedObject.Kind,
							}).Debug(event.Message)
						}
					case <-cancel:
						return nil
					}
				}
			},
			func(err error) {
				close(cancel)
			},
		)
	}

	if err := g.Run(); err != nil {
		logError("Agent error", err, cfg)
		os.Exit(1)
	}
}
