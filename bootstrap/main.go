package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	raven "github.com/getsentry/raven-go"

	"github.com/jessevdk/go-flags"
	"github.com/weaveworks/launcher/pkg/gcloud"
	"github.com/weaveworks/launcher/pkg/kubectl"
	"github.com/weaveworks/launcher/pkg/sentry"
	"github.com/weaveworks/launcher/pkg/text"
)

const (
	agentK8sURLTemplate = "{{.Scheme}}://{{.Hostname}}/k8s/agent.yaml"
)

type options struct {
	AssumeYes bool   `short:"y" long:"assume-yes" description:"Install without user confirmation"`
	Scheme    string `long:"scheme" description:"Weave Cloud scheme" default:"https"`
	Hostname  string `long:"hostname" description:"Weave Cloud hostname" default:"get.weave.works"`
	Token     string `long:"token" description:"Weave Cloud token" required:"true"`
	GKE       bool   `long:"gke" description:"Create clusterrolebinding for GKE instances"`
}

func init() {
	// https://sentry.io/weaveworks/launcher-bootstrap/
	raven.SetDSN("https://44cf71b08710447888c993011b1302fc:8f57948cabd34bbe854b196635bff59f@sentry.io/288665")
}

func main() {
	raven.CapturePanicAndWait(mainImpl, nil)
}

func mainImpl() {
	opts := options{}
	// Parse arguments with go-flags so we can forward unknown arguments to kubectl
	parser := flags.NewParser(&opts, flags.IgnoreUnknown)
	otherArgs, err := parser.Parse()
	if err != nil {
		exitNoCapture("%s\n", err)
	}
	raven.SetTagsContext(map[string]string{
		"weave_cloud_scheme":   opts.Scheme,
		"weave_cloud_hostname": opts.Hostname,
	})

	kubectlClient := kubectl.LocalClient{
		GlobalArgs: otherArgs,
	}

	if !kubectlClient.IsPresent() {
		exitNoCapture("Could not find kubectl in PATH, please install it: https://kubernetes.io/docs/tasks/tools/install-kubectl/\n")
	}

	agentK8sURL, err := text.ResolveString(agentK8sURLTemplate, opts)
	if err != nil {
		log.Fatal("invalid URL template:", err)
	}

	// Restore stdin, making fd 0 point at the terminal
	if err := syscall.Dup2(1, 0); err != nil {
		exitWithCapture("Could not restore stdin\n", err)
	}

	// Capture the kubernetes version info to help debug issues
	fmt.Println("Checking kubectl & kubernetes versions")
	versionMeta, err := kubectl.GetVersionInfo(kubectlClient)
	if err == nil {
		raven.SetTagsContext(versionMeta)
	} else {
		fmt.Fprintln(os.Stderr, "WARNING: Could not get kubernetes version info.")
	}

	// Display information on the cluster we're about to install the agent onto.
	//
	// This relies on having a current-context defined and is only to try to be
	// user friendly. So, in case of errors (eg. no current-context) we simply
	// assume kubectl can reach the API server eg. through a previously set up api
	// server proxy with kubectl proxy.
	cluster, err := kubectl.GetClusterInfo(kubectlClient)
	if err == nil {
		fmt.Printf("Installing Weave Cloud agents on %s at %s\n", cluster.Name, cluster.ServerAddress)
	}

	if opts.GKE {
		err := createGKEClusterRoleBinding(kubectlClient)
		if err != nil {
			raven.SetTagsContext(map[string]string{
				"gke_clusterrolebindingError": err.Error(),
			})
			fmt.Fprintln(os.Stderr, "WARNING: For GKE installations, a cluster-admin clusterrolebinding is required.")
			fmt.Fprintf(os.Stderr, "Could not create clusterrolebinding: %s\n", err)
		}
	}

	secretCreated, err := kubectl.CreateSecretFromLiteral(kubectlClient, "weave", "weave-cloud", "token", opts.Token, opts.AssumeYes)
	if err != nil {
		exitWithCapture("There was an error creating the secret: %s\n", err)
	}
	if !secretCreated {
		askForConfirmation("A weave-cloud secret already exists. Would you like to continue and replace the secret?")
		_, err := kubectl.CreateSecretFromLiteral(kubectlClient, "weave", "weave-cloud", "token", opts.Token, true)
		if err != nil {
			exitWithCapture("There was an error creating the secret: %s\n", err)
		}
	}

	// Apply the agent
	err = kubectl.Apply(kubectlClient, agentK8sURL)
	if err != nil {
		capture(1, "There was an error applying the agent: %s\n", err)

		// We've failed to apply the agent. kubectl apply isn't an atomic operation
		// can leave some objects behind when encountering an error. Clean things up.
		fmt.Println("Rolling back cluster changes")
		kubectl.Execute(kubectlClient, "delete", "--ignore-not-found=true", "-f", agentK8sURL)
		os.Exit(1)
	}

	fmt.Println("Successfully installed.")
}

func exitNoCapture(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg, args...)
	os.Exit(1)
}

func capture(skipFrames uint, msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	fmt.Fprintf(os.Stderr, formatted)
	sentry.CaptureAndWait(skipFrames, formatted, nil)
}

func exitWithCapture(msg string, args ...interface{}) {
	capture(2, msg, args...)
	os.Exit(1)
}

func createGKEClusterRoleBinding(kubectlClient kubectl.Client) error {
	if !gcloud.IsPresent() {
		return errors.New("Could not find gcloud in PATH, please install it: https://cloud.google.com/sdk/docs/")
	}

	account, err := gcloud.GetConfigValue("core/account")
	if err != nil || account == "" {
		return errors.New("Could not find gcloud account. Please run: gcloud auth login `ACCOUNT`")
	}
	hostUser := os.Getenv("USER")

	err = kubectl.CreateClusterRoleBinding(
		kubectlClient,
		fmt.Sprintf("cluster-admin-%s", hostUser),
		"cluster-admin",
		account,
	)
	if err != nil {
		return err
	}
	return nil
}

func askForConfirmation(s string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [y/n]: ", s)
		response, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}

		response = strings.ToLower(strings.TrimSpace(response))
		if response == "y" || response == "yes" {
			return true, nil
		} else if response == "n" || response == "no" {
			return false, nil
		}
	}
}
