package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/oklog/run"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/launcher/pkg/kubectl"
	"github.com/weaveworks/launcher/pkg/text"
)

const (
	defaultAgentPollURL = "https://get.weave.works/k8s/agent.yaml"
	defaultWCPollURL    = "https://cloud.weave.works/k8s.yaml?k8s-version={{.KubernetesVersion}}&t={{.Token}}"
)

type urlContext struct {
	KubernetesVersion string
	Token             string
}

func setLogLevel(logLevel string) error {
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("error parsing log level: %v", err)
	}
	log.SetLevel(level)
	return nil
}

func updateAgents(agentPollURL, wcPollURL string) {
	// Self-update
	log.Info("Updating self from ", agentPollURL)
	output, err := kubectl.ExecuteCommand([]string{"apply", "-f", agentPollURL})
	if err != nil {
		log.Errorf("Failed to execute kubectl apply: %s", err)
		return
	}

	// If the agent is updating, we will be killed via SIGTERM.
	// Because of the rollingUpdate strategy, we are only killed when the new
	// agent is ready.
	// If we are not killed after 5 minutes, assume the new agent did not become
	// ready so recover by rolling back.
	if strings.Contains(output, "configured") {
		time.Sleep(5 * time.Minute)

		log.Error("Deployment of the new agent failed. Rolling back...")
		_, err := kubectl.ExecuteCommand([]string{
			"rollout",
			"undo",
			"--namespace=weave",
			"deployment/weave-agent",
		})
		if err != nil {
			log.Errorf("Failed rolling back agent. Will continue to check for updates.")
			return
		}

		// Return so we continue updating the agent until success
		log.Info("The new agent was rolled back.")
		return
	}

	// Update Weave Cloud agents
	log.Info("Updating WC from ", wcPollURL)
	_, err = kubectl.ExecuteCommand([]string{"apply", "-f", wcPollURL})
	if err != nil {
		log.Errorf("Failed to execute kubectl apply: %s", err)
		return
	}
}

func main() {
	logLevel := flag.String("log.level", "info", "verbosity of log output - one of 'debug', 'info' (default), 'warning', 'error', 'fatal'")

	agentPollURL := flag.String("wc.poll-url", defaultAgentPollURL, "URL to poll for the agent manifest")
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

	wcPollURL, err := text.ResolveString(*wcPollURLTemplate, urlContext{
		KubernetesVersion: "1.8", // TODO: ask the API server
		Token:             *wcToken,
	})
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

					updateAgents(*agentPollURL, wcPollURL)

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
		log.Error(err)
		os.Exit(1)
	}
}
