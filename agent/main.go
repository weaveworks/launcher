package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oklog/run"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/launcher/pkg/text"
)

const (
	defaultPollURL = "https://cloud.weave.works/k8s.yaml?k8s-version={{.KubernetesVersion}}&t={{.Token}}"
)

type urlContext struct {
	KubernetesVersion string
	Token             string
}

func applyManifest(kubectl interface{}, URL string) {
	log.Info("polling ", URL)

	// TODO: actually apply something!
}

func setLogLevel(logLevel string) error {
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("error parsing log level: %v", err)
	}
	log.SetLevel(level)
	return nil
}

func main() {
	logLevel := flag.String("log.level", "info", "verbosity of log output - one of 'debug', 'info' (default), 'warning', 'error', 'fatal'")

	wcToken := flag.String("wc.token", "", "Weave Cloud instance token")
	wcPollInterval := flag.Duration("wc.poll-interval", 1*time.Hour, "Polling interval to check for new manifests")
	wcPollURLTemplate := flag.String("wc.poll-url", defaultPollURL, "URL to poll for new manifests")

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

					applyManifest(nil, wcPollURL)

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
