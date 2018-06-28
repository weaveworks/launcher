package e2e

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/dlespiau/kube-harness"
	"github.com/dlespiau/kube-harness/logger"
)

var kube *harness.Harness

func manifestDirectory() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "resources")
}

func TestMain(m *testing.M) {
	kubeconfig := flag.String("kubeconfig", "", "kube config path, e.g. $HOME/.kube/config")
	verbose := flag.Bool("log.verbose", false, "turn on more verbose logging")
	//tag := flag.String("image-tag", "", "tag of docker images to test")
	flag.Parse()

	options := harness.Options{
		Kubeconfig:        *kubeconfig,
		ManifestDirectory: manifestDirectory(),
	}
	if *verbose {
		options.LogLevel = logger.Debug
	}
	kube = harness.New(options)

	if err := kube.Setup(); err != nil {
		log.Printf("failed to initialize test harness: %v\n", err)
	}

	code := kube.Run(m)

	if err := kube.Close(); err != nil {
		log.Printf("failed to teardown test harness: %v\n", err)
		code = 1
	}

	os.Exit(code)
}
