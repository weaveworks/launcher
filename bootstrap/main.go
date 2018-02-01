package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/jessevdk/go-flags"
	"github.com/weaveworks/launcher/pkg/kubectl"
	"github.com/weaveworks/launcher/pkg/text"
)

const (
	agentK8sURLTemplate = "https://{{.Hostname}}/k8s/agent.yaml"
)

type options struct {
	AssumeYes bool   `short:"y" long:"assume-yes" description:"Install without user confirmation"`
	Hostname  string `long:"hostname" description:"Weave Cloud hostname" default:"get.weave.works"`
	Token     string `long:"token" description:"Weave Cloud token" required:"true"`
}

func main() {
	opts := options{}
	// Parse arguments with go-flags so we can forward unknown arguments to kubectl
	parser := flags.NewParser(&opts, flags.IgnoreUnknown)
	otherArgs, err := parser.Parse()
	if err != nil {
		die("%s\n", err)
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

	fmt.Printf("\nThis will install Weave Cloud on the following cluster:\n")
	fmt.Printf("    Name: %s\n    Server: %s\n\n", cluster.Name, cluster.ServerAddress)
	fmt.Printf("Please run 'kubectl config use-context' or pass '--kubeconfig' if you would like to change this.\n\n")

	confirmed, err := askForConfirmation("Would you like to continue?", opts.AssumeYes)
	if err != nil {
		die("There was an error: %s\n", err)
	}
	if !confirmed {
		fmt.Println("Cancelled.")
		return
	}

	fmt.Println("Storing the instance token in the weave-cloud secret...")
	secretCreated, err := createWCSecret(opts, otherArgs)
	if err != nil {
		die("There was an error creating the secret: %s\n", err)
	}
	if !secretCreated {
		fmt.Println("Cancelled.")
		return
	}

	fmt.Println("Applying the agent...")
	_, err = kubectl.ExecuteWithGlobalArgs(otherArgs, "apply", "-f", agentK8sURL)
	if err != nil {
		die("There was an error applying the agent: %s\n", err)
	}
}

func die(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg, args...)
	os.Exit(1)
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
