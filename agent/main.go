package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/getsentry/raven-go"
	"github.com/oklog/run"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/launcher/pkg/kubectl"
	"github.com/weaveworks/launcher/pkg/text"
)

const (
	defaultAgentPollURL = "https://get.weave.works/k8s/agent.yaml"
	defaultWCPollURL    = "https://cloud.weave.works/k8s.yaml?k8s-version={{.KubernetesVersion}}&t={{.Token}}"
)

type agentContext struct {
	KubernetesVersion string
	Token             string
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

func logError(msg string, err error, ctx agentContext) {
	log.Errorf("%s: %s", msg, err)
	ravenTags := map[string]string{
		// TODO: #25 - include instance identifier (token not suitable due to security concerns)
		"kubernetes": ctx.KubernetesVersion,
	}
	raven.CaptureErrorAndWait(err, ravenTags)
}

func updateAgents(agentPollURL, wcPollURL string, agentCtx agentContext, cancel <-chan interface{}) {
	// Self-update
	log.Info("Updating self from ", agentPollURL)
	output, err := kubectl.ExecuteCommand([]string{"apply", "-f", agentPollURL})
	if err != nil {
		logError("Failed to execute kubectl apply", err, agentCtx)
		return
	}

	// If the agent is updating, we will be killed via SIGTERM.
	// Because of the rollingUpdate strategy, we are only killed when the new
	// agent is ready.
	// If we are not killed after 5 minutes, assume the new agent did not become
	// ready so recover by rolling back.
	if strings.Contains(output, "configured") {
		select {
		case <-time.After(5 * time.Minute):
		case <-cancel:
			return
		}

		logError("Deployment of the new agent failed. Rolling back...", errors.New("Deployment failed"), agentCtx)
		_, err := kubectl.ExecuteCommand([]string{
			"rollout",
			"undo",
			"--namespace=weave",
			"deployment/weave-agent",
		})
		if err != nil {
			logError("Failed rolling back agent. Will continue to check for updates.", err, agentCtx)
			return
		}

		// Return so we continue updating the agent until success
		logError("The new agent was rolled back.", errors.New("Rollback success"), agentCtx)
		return
	}

	// Update Weave Cloud agents
	log.Info("Updating WC from ", wcPollURL)
	_, err = kubectl.ExecuteCommand([]string{"apply", "-f", wcPollURL})
	if err != nil {
		logError("Failed to execute kubectl apply", err, agentCtx)
		return
	}
}

func main() {
	logLevel := flag.String("log.level", "info", "verbosity of log output - one of 'debug', 'info' (default), 'warning', 'error', 'fatal'")

	agentPollURL := flag.String("agent.poll-url", defaultAgentPollURL, "URL to poll for the agent manifest")
	wcToken := flag.String("wc.token", "", "Weave Cloud instance token")
	wcPollInterval := flag.Duration("wc.poll-interval", 1*time.Hour, "Polling interval to check WC manifests")
	wcPollURLTemplate := flag.String("wc.poll-url", defaultWCPollURL, "URL to poll for WC manifests")

	flag.Parse()

	if err := setLogLevel(*logLevel); err != nil {
		log.Fatal(err)
	}

	if *wcToken == "" {
		log.Fatal("missing Weave Cloud instance token, provide one with -wc.token")
	}

	agentCtx := agentContext{
		KubernetesVersion: "1.8", // TODO: ask the API server
		Token:             *wcToken,
	}

	wcPollURL, err := text.ResolveString(*wcPollURLTemplate, agentCtx)
	if err != nil {
		log.Fatal("invalid URL template:", err)
	}

	var g run.Group

	// Poll for new manifests every wcPollInterval.
	{
		cancel := make(chan interface{})
		g.Add(
			func() error {
				for {

					updateAgents(*agentPollURL, wcPollURL, agentCtx, cancel)

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

	if err := g.Run(); err != nil {
		logError("Agent error", err, agentCtx)
		os.Exit(1)
	}
}
