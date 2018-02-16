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
		die("%s\n", err)
	}
	raven.SetTagsContext(map[string]string{
		"weave_cloud_scheme":   opts.Scheme,
		"weave_cloud_hostname": opts.Hostname,
	})

	if !kubectl.IsPresent() {
		die("Could not find kubectl in PATH, please install it: https://kubernetes.io/docs/tasks/tools/install-kubectl/\n")
	}

	agentK8sURL, err := text.ResolveString(agentK8sURLTemplate, opts)
	if err != nil {
		log.Fatal("invalid URL template:", err)
	}

	// Restore stdin, making fd 0 point at the terminal
	if err := syscall.Dup2(1, 0); err != nil {
		die("Could not restore stdin\n", err)
	}

	// Ask the user to confirm the cluster
	cluster, err := kubectl.GetClusterInfo(otherArgs)
	if err != nil {
		die("There was an error fetching the current cluster info: %s\n", err)
	}

	fmt.Printf("Installing Weave Cloud agents on %s at %s\n", cluster.Name, cluster.ServerAddress)

	if opts.GKE {
		err := createGKEClusterRoleBinding(otherArgs)
		if err != nil {
			fmt.Println("WARNING: For GKE installations, a cluster-admin clusterrolebinding is required.")
			fmt.Printf("Could not create clusterrolebinding: %s", err)
		}
	}

	secretCreated, err := createWCSecret(opts, otherArgs)
	if err != nil {
		die("There was an error creating the secret: %s\n", err)
	}
	if !secretCreated {
		fmt.Println("Cancelled.")
		return
	}

	// Apply the agent
	_, err = kubectl.ExecuteWithGlobalArgs(otherArgs, "apply", "-f", agentK8sURL)
	if err != nil {
		die("There was an error applying the agent: %s\n", err)
	}

	fmt.Println("Successfully installed.")
}

func die(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	fmt.Fprintf(os.Stderr, formatted)
	raven.CaptureMessageAndWait(formatted, nil)
	os.Exit(1)
}

func createGKEClusterRoleBinding(otherArgs []string) error {
	if !gcloud.IsPresent() {
		return errors.New("Could not find gcloud in PATH, please install it: https://cloud.google.com/sdk/docs/")
	}

	account, err := gcloud.GetConfigValue("core/account")
	if err != nil || account == "" {
		return errors.New("Could not find gcloud account. Please run: gcloud auth login `ACCOUNT`")
	}
	hostUser := os.Getenv("USER")

	_, err = kubectl.ExecuteWithGlobalArgs(
		otherArgs,
		"create",
		"clusterrolebinding",
		fmt.Sprintf("cluster-admin-%s", hostUser),
		"--clusterrole=cluster-admin",
		"--user",
		account,
	)
	if err != nil {
		return err
	}
	return nil
}

func createWCSecret(opts options, otherArgs []string) (bool, error) {
	secretExists, err := kubectl.ResourceExists("secret", "weave-cloud", "weave", otherArgs)
	if err != nil {
		return false, err
	}

	if secretExists {
		confirmed, err := askForConfirmation("A weave-cloud secret already exists. Would you like to continue and replace the secret?", opts.AssumeYes)
		if err != nil {
			return false, err
		}
		if !confirmed {
			return false, nil
		}

		// Delete the secret
		_, err = kubectl.ExecuteWithGlobalArgs(otherArgs, "delete", "secret", "weave-cloud", "--namespace=weave")
		if err != nil {
			return false, err
		}
	}

	// Create the weave namespace and the weave-cloud secret
	_, err = kubectl.CreateNamespace("weave", otherArgs)
	if err != nil {
		return false, err
	}

	_, err = kubectl.CreateSecretFromLiteral("weave", "weave-cloud", "token", opts.Token, otherArgs)
	if err != nil {
		return false, err
	}
	return true, nil
}

func askForConfirmation(s string, assumeYes bool) (bool, error) {
	if assumeYes {
		return true, nil
	}

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
