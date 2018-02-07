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
	"github.com/weaveworks/launcher/pkg/text"
	"github.com/weaveworks/launcher/pkg/weavecloud"
)

const (
	defaultAgentPollURL      = "https://get.weave.works/k8s/agent.yaml"
	defaultAgentRecoveryWait = 5 * time.Minute
	defaultWCPollURL         = "https://cloud.weave.works/k8s.yaml" +
		"?k8s-version={{.KubernetesVersion}}&t={{.Token}}&&omit-support-info=true" +
		"{{if .FluxConfig}}" +
		"&git-label={{.FluxConfig.GitLabel}}&git-url={{.FluxConfig.GitURL}}" +
		"&git-path={{.FluxConfig.GitPath}}&git-branch={{.FluxConfig.GitBranch}}" +
		"{{end}}"
	defaultWCOrgLookupURL = "https://cloud.weave.works/api/users/org/lookup"
)

type agentConfig struct {
	KubernetesVersion string
	Token             string
	InstanceID        string
	AgentPollURL      string
	AgentRecoveryWait time.Duration
	WCPollURL         string
	KubeClient        *kubeclient.Clientset
	FluxConfig        *FluxConfig
}

// FluxConfig stores existing flux arguments which will be used when updating WC agents
type FluxConfig struct {
	GitLabel  string `long:"git-label"`
	GitURL    string `long:"git-url"`
	GitPath   string `long:"git-path"`
	GitBranch string `long:"git-branch"`
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

func logError(msg string, err error, cfg agentConfig) {
	log.Errorf("%s: %s", msg, err)
	ravenTags := map[string]string{
		"kubernetes": cfg.KubernetesVersion,
		"instance":   cfg.InstanceID,
	}
	raven.CaptureErrorAndWait(err, ravenTags)
}

func updateAgents(cfg agentConfig, cancel <-chan interface{}) {
	// Self-update
	log.Info("Updating self from ", cfg.AgentPollURL)

	initialRevision, err := k8s.GetLatestDeploymentReplicaSetRevision(cfg.KubeClient, "weave", "weave-agent")
	if err != nil {
		logError("Failed to fetch latest deployment replicateset revision", err, cfg)
		return
	}
	log.Info("Revision before self-update: ", initialRevision)
	_, err = kubectl.Execute("apply", "-f", cfg.AgentPollURL)
	if err != nil {
		logError("Failed to execute kubectl apply", err, cfg)
		return
	}
	updatedRevision, err := k8s.GetLatestDeploymentReplicaSetRevision(cfg.KubeClient, "weave", "weave-agent")
	if err != nil {
		logError("Failed to fetch latest deployment replicateset revision", err, cfg)
		return
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
			return
		}

		logError("Deployment of the new agent failed. Rolling back.", errors.New("Deployment failed"), cfg)
		_, err := kubectl.Execute("rollout", "undo", "--namespace=weave", "deployment/weave-agent")
		if err != nil {
			logError("Failed rolling back agent. Will continue to check for updates.", err, cfg)
			return
		}

		// Return so we continue updating the agent until success
		logError("The new agent was rolled back.", errors.New("Rollback success"), cfg)
		return
	}

	// Get existing flux config
	fluxCfg, err := getFluxConfig("weave")
	if err != nil {
		logError("Failed getting existing flux config", err, cfg)
	}
	if fluxCfg != nil {
		cfg.FluxConfig = fluxCfg
	}

	// Update Weave Cloud agents
	log.Info("Updating WC from ", cfg.WCPollURL)
	_, err = kubectl.Execute("apply", "-f", cfg.WCPollURL)
	if err != nil {
		logError("Failed to execute kubectl apply", err, cfg)
		return
	}
}

func setupKubeClient() (*kubeclient.Clientset, error) {
	kubeConfig, err := k8s.GetClientConfig(&k8s.ClientConfig{})
	if err != nil {
		return nil, fmt.Errorf("client config: %s", err)
	}
	return kubeclient.NewForConfig(kubeConfig)
}

func handleAnyPanic() {
	if e := recover(); e != nil {
		raven.CaptureErrorAndWait(fmt.Errorf("%s", e), nil)
		panic(e)
	}
}

func main() {
	defer handleAnyPanic()

	logLevel := flag.String("log.level", "info", "verbosity of log output - one of 'debug', 'info' (default), 'warning', 'error', 'fatal'")

	agentPollURL := flag.String("agent.poll-url", defaultAgentPollURL, "URL to poll for the agent manifest")
	agentRecoveryWait := flag.Duration("agent.recovery-wait", defaultAgentRecoveryWait, "Duration to wait before recovering from a failed self update")
	wcToken := flag.String("wc.token", "", "Weave Cloud instance token")
	wcPollInterval := flag.Duration("wc.poll-interval", 1*time.Hour, "Polling interval to check WC manifests")
	wcPollURLTemplate := flag.String("wc.poll-url", defaultWCPollURL, "URL to poll for WC manifests")
	wcOrgLookupURL := flag.String("wc.org-lookup-url", defaultWCOrgLookupURL, "URL to lookup org external ID by token")

	eventsReportInterval := flag.Duration("events.report-interval", 3*time.Second, "Minimal time interval between two reports")

	featureInstall := flag.Bool("feature.install-agents", true, "Whether the agent should install anything in the cluster or not")

	flag.Parse()

	if err := setLogLevel(*logLevel); err != nil {
		log.Fatal(err)
	}

	if *wcToken == "" {
		log.Fatal("missing Weave Cloud instance token, provide one with -wc.token")
	}

	kubeClient, err := setupKubeClient()
	if err != nil {
		log.Fatal("kubernetes client:", err)
	}

	version, err := kubeClient.Discovery().ServerVersion()
	if err != nil {
		log.Fatal("get server version:", err)
	}

	instanceID, err := weavecloud.LookupInstanceByToken(*wcOrgLookupURL, *wcToken)
	if err != nil {
		logError("lookup instance by token", err, agentConfig{})
	}

	// Migrate old versions and use any existing flux config
	existingFluxCfg := migrate()
	if existingFluxCfg != nil {
		log.Infof("Using existing flux config: %+v", existingFluxCfg)
	}

	cfg := agentConfig{
		KubernetesVersion: version.GitVersion,
		Token:             *wcToken,
		InstanceID:        instanceID,
		AgentPollURL:      *agentPollURL,
		AgentRecoveryWait: *agentRecoveryWait,
		KubeClient:        kubeClient,
		FluxConfig:        existingFluxCfg,
	}

	wcPollURL, err := text.ResolveString(*wcPollURLTemplate, cfg)
	if err != nil {
		log.Fatal("invalid URL template:", err)
	}
	cfg.WCPollURL = wcPollURL

	var g run.Group

	// Poll for new manifests every wcPollInterval.
	if *featureInstall {
		cancel := make(chan interface{})

		g.Add(
			func() error {
				for {

					updateAgents(cfg, cancel)

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
		cancel := make(chan interface{})
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
	{
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
	{
		cancel := make(chan interface{})
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
