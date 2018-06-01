package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	raven "github.com/getsentry/raven-go"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/yaml"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

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
	defaultCloudwatchURL = "https://{{.WCHostname}}/k8s/{{.KubernetesMinorMajorVersion}}/cloudwatch.yaml?" +
		"aws-region={{.Region}}&aws-secret={{.SecretName}}&aws-resources=rds"
)

type agentConfig struct {
	KubernetesVersion      string
	KubernetesMajorVersion string
	KubernetesMinorVersion string
	WCHostname             string
	Token                  string
	InstanceID             string
	AgentPollURLTemplate   string
	AgentRecoveryWait      time.Duration
	ReportErrors           bool
	WCPollURLTemplate      string
	KubeClient             *kubeclient.Clientset
	KubectlClient          kubectl.Client
	FluxConfig             *FluxConfig

	CMInformer     cache.SharedIndexInformer
	SecretInformer cache.SharedIndexInformer
}

type cloudwatch struct {
	Region     string
	SecretName string
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
	if cfg.ReportErrors {
		sentry.CaptureAndWait(1, formatted, nil)
	}
}

func updateAgents(cfg *agentConfig, cancel <-chan interface{}) {
	// Self-update
	agentPollURL, err := text.ResolveString(cfg.AgentPollURLTemplate, cfg)
	if err != nil {
		log.Fatal("invalid URL template: ", err)
	}
	log.Info("Updating self from ", agentPollURL)

	initialRevision, err := k8s.GetLatestDeploymentReplicaSetRevision(cfg.KubeClient, "weave", "weave-agent")
	if err != nil {
		logError("Failed to fetch latest deployment replicateset revision", err, cfg)
		return
	}
	log.Info("Revision before self-update: ", initialRevision)
	err = kubectl.Apply(cfg.KubectlClient, agentPollURL)
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
		_, err := cfg.KubectlClient.Execute("rollout", "undo", "--namespace=weave", "deployment/weave-agent")
		if err != nil {
			logError("Failed rolling back agent. Will continue to check for updates.", err, cfg)
			return
		}

		// Return so we continue updating the agent until success
		logError("The new agent was rolled back.", errors.New("Rollback success"), cfg)
		return
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
		log.Fatal("invalid URL template: ", err)
	}
	log.Info("Updating WC from ", wcPollURL)
	err = kubectl.Apply(cfg.KubectlClient, wcPollURL)
	if err != nil {
		logError("Failed to execute kubectl apply", err, cfg)
		return
	}
}

func setupKubeClient() (*kubeclient.Clientset, error) {
	kubeConfig, err := k8s.NewClientConfig(&k8s.ClientConfig{
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
	reportErrors := flag.Bool("agent.report-errors", false, "Should the agent report errors to sentry")
	address := flag.String("agent.address", ":8080", "agent HTTP address")

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

	if *featureInstall && *wcToken == "" {
		log.Fatal("missing Weave Cloud instance token, provide one with -wc.token")
	}

	cfg := &agentConfig{
		Token:                *wcToken,
		AgentRecoveryWait:    *agentRecoveryWait,
		ReportErrors:         *reportErrors,
		KubectlClient:        kubectl.LocalClient{},
		WCHostname:           *wcHostname,
		AgentPollURLTemplate: *agentPollURLTemplate,
		WCPollURLTemplate:    *wcPollURLTemplate,
	}
	raven.SetTagsContext(map[string]string{
		"weave_cloud_hostname": *wcHostname,
	})

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
	cfg.KubernetesMajorVersion = version.Major
	cfg.KubernetesMinorVersion = version.Minor
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

	// HTTP server for prometheus metrics
	ln, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatal("HTTP listen: ", err)
		os.Exit(1)
	}

	http.Handle("/metrics", promhttp.Handler())

	g.Add(func() error {
		return http.Serve(ln, nil)
	}, func(error) {
		ln.Close()
	})

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

	{
		stopCh := make(chan struct{})
		g.Add(
			func() error {
				// Watch for ConfigMap and Secret creation/update/deletion
				// so we can deploy Cloudwatch resources.
				watchConfigMaps(cfg)
				watchSecrets(cfg)
				go cfg.CMInformer.Run(stopCh)
				go cfg.SecretInformer.Run(stopCh)

				<-stopCh
				return nil
			},
			func(err error) {
				close(stopCh)
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

// Watch for CM creation.
func watchConfigMaps(cfg *agentConfig) {
	source := cache.NewListWatchFromClient(
		cfg.KubeClient.Core().RESTClient(),
		"configmaps",
		"weave",
		fields.SelectorFromSet(fields.Set{"metadata.name": "cloudwatch"}))

	cfg.CMInformer = cache.NewSharedIndexInformer(
		source,
		&apiv1.ConfigMap{},
		0,
		cache.Indexers{},
	)

	cfg.CMInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    cfg.handleCMAdd,
		UpdateFunc: cfg.handleCMUpdate,
		DeleteFunc: cfg.handleCMDelete,
	})
}

// Triggered on all ConfigMap creation with name cloudwatch in weave ns
func (cfg *agentConfig) handleCMAdd(obj interface{}) {
	cm, ok := obj.(*apiv1.ConfigMap)
	if !ok {
		log.Error("Failed to type assert ConfigMap")
		return
	}

	cfg.checkOrInstallCloudWatch(cm)
}

// Triggered on all ConfigMap update with name cloudwatch in weave ns
func (cfg *agentConfig) handleCMUpdate(old, cur interface{}) {
	log.Info("ConfigMap was updated")
	cm, ok := cur.(*apiv1.ConfigMap)
	if !ok {
		log.Error("Failed to type assert ConfigMap")
		return
	}

	cfg.checkOrInstallCloudWatch(cm)
}

// Triggered on only ConfigMap deletion with name cloudwatch in weave ns
func (cfg *agentConfig) handleCMDelete(obj interface{}) {
	log.Info("ConfigMap was deleted")
	// TODO: Handle ConfigMap deletion. Delete individual objects that were created as at this point.
}

// Watch for Secret creation/update/deletion.
func watchSecrets(cfg *agentConfig) {
	source := cache.NewListWatchFromClient(
		cfg.KubeClient.Core().RESTClient(),
		"secrets",
		"weave",
		fields.Everything())

	cfg.SecretInformer = cache.NewSharedIndexInformer(
		source,
		&apiv1.Secret{},
		0,
		cache.Indexers{},
	)

	cfg.SecretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    cfg.handleSecretAdd,
		UpdateFunc: cfg.handleSecretUpdate,
		DeleteFunc: cfg.handleSecretDelete,
	})
}

// Triggered on all Secret created in the weave ns
func (cfg *agentConfig) handleSecretAdd(obj interface{}) {
	_, ok := obj.(*apiv1.Secret)
	if !ok {
		// If the object is not a secret we should ignore.
		log.Error("Failed to type assert Secret")
		return
	}

	if err := cfg.conformSecret(); err != nil {
		log.Error(err)
		return
	}
}

// Triggered on all Secret updates in the weave ns
func (cfg *agentConfig) handleSecretUpdate(old, cur interface{}) {
	_, ok := cur.(*apiv1.Secret)
	if !ok {
		// If the object is not a secret there was an error.
		log.Error("Failed to type assert Secret")
		return
	}

	if err := cfg.conformSecret(); err != nil {
		log.Error(err)
		return
	}
}

// Triggered on all Secret deletions in the weave ns
func (cfg *agentConfig) handleSecretDelete(obj interface{}) {
	// TODO: Handle Secret deletion. Delete individual objects that were created.
}

func (cfg *agentConfig) conformSecret() error {
	// Get ConfigMap
	cm, err := cfg.getConfigMap("cloudwatch")
	if err != nil {
		// If we don't have the cloudwatch CM this is the wrong secret.
		return err
	}

	cfg.checkOrInstallCloudWatch(cm)
	return nil
}

// checkOrInstallCloudWatch by applying the manifest file.
func (cfg *agentConfig) checkOrInstallCloudWatch(cm *apiv1.ConfigMap) {
	for name, content := range cm.Data {
		if name != "cloudwatch.yaml" {
			return
		}

		cw, err := parseCloudwatchYaml(content)
		if err != nil {
			log.Error(err)
			return
		}

		// Make sure secret exists before applying cloudwatch manifest file
		if !cfg.secretExists(cw.SecretName) {
			log.Errorf("Secret %s specified in the cloudwatch ConfigMap does not exist", cw.SecretName)
			return
		}

		err = deployCloudwatch(cfg, cw)
		if err != nil {
			log.Error(err)
			return
		}
	}
}

func (cfg *agentConfig) secretExists(secretName string) bool {
	_, err := cfg.KubeClient.CoreV1().Secrets("weave").Get(secretName, metav1.GetOptions{})
	if err != nil {
		return false
	}

	return true
}

func (cfg *agentConfig) getConfigMap(name string) (*apiv1.ConfigMap, error) {
	cm, err := cfg.KubeClient.CoreV1().ConfigMaps("weave").Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return cm, nil
}

func parseCloudwatchYaml(cm string) (*cloudwatch, error) {
	cw := cloudwatch{}
	if err := yaml.NewYAMLOrJSONDecoder(bytes.NewBufferString(cm), 1000).Decode(&cw); err != nil {
		return nil, err
	}
	return &cw, nil
}

func deployCloudwatch(cfg *agentConfig, cw *cloudwatch) error {
	// Get manifest file to apply
	cwPollURL, err := text.ResolveString(defaultCloudwatchURL, map[string]string{
		"WCHostname":                  cfg.WCHostname,
		"KubernetesMinorMajorVersion": fmt.Sprintf("v%s.%s", cfg.KubernetesMajorVersion, cfg.KubernetesMinorVersion),
		"Region":                      cw.Region,
		"SecretName":                  cw.SecretName,
	})
	log.Info(cwPollURL)
	if err != nil {
		log.Fatal("invalid URL template: ", err)
	}

	err = kubectl.Apply(cfg.KubectlClient, cwPollURL)
	if err != nil {
		return err
	}

	return nil
}
