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
	agentK8sURLTemplate = "{{.Scheme}}://{{.Hostname}}/k8s/agent.yaml"
)

type options struct {
	AssumeYes bool   `short:"y" long:"assume-yes" description:"Install without user confirmation"`
	Scheme    string `long:"scheme" description:"Weave Cloud scheme" default:"https"`
	Hostname  string `long:"hostname" description:"Weave Cloud hostname" default:"get.weave.works"`
	Token     string `long:"token" description:"Weave Cloud token" required:"true"`
}

// Delete old Weave Cloud objects and return if we have indeed deleted anything.
func deleteKubeSystemObjects(globalArgs []string) bool {
	deleted := false

	out, _ := kubectl.ExecuteWithGlobalArgs(globalArgs, "delete", "--namespace=kube-system",
		"deployments,pods,services,daemonsets,serviceaccounts,configmaps,secrets",
		"--selector=app in (weave-flux, weave-cortex, weave-scope)")
	// Used with a selector, kubectl 1.7.5 returns a 0 exit code with the message
	// "No resources found" when there's no matching resources.
	deleted = deleted || !strings.Contains(out, "No resources found")

	out, _ = kubectl.ExecuteWithGlobalArgs(globalArgs, "delete", "--namespace=kube-system",
		"deployments,pods,services,daemonsets,serviceaccounts,configmaps,secrets",
		"--selector=name in (weave-flux, weave-cortex, weave-scope)")
	deleted = deleted || !strings.Contains(out, "No resources found")

	_, err := kubectl.ExecuteWithGlobalArgs(globalArgs, "--namespace=kube-system", "delete", "secret", "flux-git-deploy")
	deleted = deleted || (err == nil)

	return deleted
}

func main() {
	opts := options{}
	// Parse arguments with go-flags so we can forward unknown arguments to kubectl
	parser := flags.NewParser(&opts, flags.IgnoreUnknown)
	otherArgs, err := parser.Parse()
	if err != nil {
		die("%s\n", err)
	}

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

	// Users that have installed Weave Cloud manifests before January the 11th
	// 2018, will have the agents installed in the kube-system namespace. If that's
	// the case let's handle the migration to the weave namespace:
	// 1. Delete our objects in the kube-system namespace
	// 2. Let launcher-agent install the new ones in the weave namespace
	fmt.Println("Checking for any previous installation...")
	deleted := deleteKubeSystemObjects(otherArgs)
	if deleted {
		fmt.Println("Removed old agents from the kube-system namespace. You will have to reconfigure Deploy.")
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

	fmt.Println("Successfully installed. Please check the status at https://cloud.weave.works.")
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
