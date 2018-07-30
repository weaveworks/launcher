package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"syscall"

	"github.com/blang/semver"
	raven "github.com/getsentry/raven-go"

	"github.com/jessevdk/go-flags"
	"github.com/weaveworks/launcher/pkg/gcloud"
	"github.com/weaveworks/launcher/pkg/kubectl"
	"github.com/weaveworks/launcher/pkg/sentry"
	"github.com/weaveworks/launcher/pkg/text"
	"github.com/weaveworks/launcher/pkg/weavecloud"
)

const (
	agentK8sURLTemplate = "{{.Scheme}}://{{.LauncherHostname}}/k8s/agent.yaml" +
		"{{if .CRIEndpoint}}" +
		"&cri-endpoint={{.CRIEndpoint}}" +
		"{{end}}"
)

type options struct {
	AssumeYes        bool   `short:"y" long:"assume-yes" description:"Install without user confirmation"`
	Scheme           string `long:"scheme" description:"Weave Cloud scheme" default:"https"`
	LauncherHostname string `long:"wc.launcher" description:"Weave Cloud launcher hostname" default:"get.weave.works"`
	WCHostname       string `long:"wc.hostname" description:"Weave Cloud hostname" default:"cloud.weave.works"`
	Token            string `long:"token" description:"Weave Cloud token" required:"true"`
	GKE              bool   `long:"gke" description:"Create clusterrolebinding for GKE instances"`
	ReportErrors     bool   `long:"report-errors" description:"Should install errors be reported to sentry"`
	SkipChecks       bool   `long:"skip-checks" description:"Skip pre-flight checks"`
	CRIEndpoint      string `long:"cri-endpoint" description:"Set custom CRI container runtime endpoint. e.g.: 'unix:///var/run/crio/crio.sock'"`
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
		exitWithCapture(opts, "%s\n", err)
	}
	raven.SetTagsContext(map[string]string{
		"weave_cloud_scheme":   opts.Scheme,
		"weave_cloud_launcher": opts.LauncherHostname,
		"weave_cloud_hostname": opts.WCHostname,
	})

	// Due to some users Kubernetes clusters having invalid, e.g. self-signed,
	// certificates, we default to skipping the certificate validation.
	otherArgs = append(otherArgs, "--insecure-skip-tls-verify")
	kubectlClient := kubectl.LocalClient{
		GlobalArgs: otherArgs,
	}

	if !kubectlClient.IsPresent() {
		exitWithCapture(opts, "Could not find kubectl in PATH, please install it: https://kubernetes.io/docs/tasks/tools/install-kubectl/\n")
	}

	agentK8sURL, err := text.ResolveString(agentK8sURLTemplate, opts)
	if err != nil {
		log.Fatal("invalid URL template:", err)
	}
	wcOrgLookupURL, err := text.ResolveString(weavecloud.DefaultWCOrgLookupURLTemplate, opts)
	if err != nil {
		log.Fatal("invalid URL template:", err)
	}

	// Restore stdin, making fd 0 point at the terminal
	if err := syscall.Dup2(1, 0); err != nil {
		exitWithCapture(opts, "Could not restore stdin\n", err)
	}

	fmt.Println("Preparing for Weave Cloud setup")

	// Capture the kubernetes version info to help debug issues
	checkK8sVersion(kubectlClient, opts) // NB exits on error

	InstanceID, InstanceName, err := weavecloud.LookupInstanceByToken(wcOrgLookupURL, opts.Token)
	if err != nil {
		exitWithCapture(opts, "Error looking up Weave Cloud instance: %s\n", err)
	}
	raven.SetTagsContext(map[string]string{"instance": InstanceID})
	fmt.Printf("Connecting cluster to %q (id: %s) on Weave Cloud\n", InstanceName, InstanceID)

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

			errText := err.Error()
			if strings.Contains(errText, "Forbidden") || strings.Contains(errText, "forbidden") {
				exitWithCapture(opts, "Could not create clusterrolebinding. GKE role \"Kubernetes Engine Admin\" (containers.admin) required to create resources.\n%s\n", err)
			}
		}
	}

	if !opts.SkipChecks {
		// Perform a check to make sure DNS is working correctly.
		fmt.Println("Performing a check of the Kubernetes installation setup.")
		ok, err := kubectl.TestDNS(kubectlClient, "cloud.weave.works")
		if err != nil {
			exitWithCapture(opts, "There was an error while performing a DNS check: %s. Please check that your cluster can download images and run pods.", err)
		}

		// We exit if the DNS pods are not up and running, as the installer needs to be
		// able to connect to the server to correctly setup the needed resources.
		if !ok {
			exitWithCapture(opts, "DNS is not working in this Kubernetes cluster. We require correct DNS setup to continue.")
		}
	}

	secretCreated, err := kubectl.CreateSecretFromLiteral(kubectlClient, "weave", "weave-cloud", "token", opts.Token, opts.AssumeYes)
	if err != nil {
		exitWithCapture(opts, "There was an error creating the secret: %s\n", err)
	}
	if !secretCreated {
		currentToken, err := kubectl.GetSecretValue(kubectlClient, "weave", "weave-cloud", "token")
		if err != nil {
			exitWithCapture(opts, "There was an error checking the current secret: %s\n", err)
		}
		if currentToken != opts.Token {
			currentInstanceID, currentInstanceName, errCurrent := weavecloud.LookupInstanceByToken(wcOrgLookupURL, currentToken)
			msg := "This cluster is currently connected to "
			if errCurrent == nil {
				msg += fmt.Sprintf("%q (id: %s) on Weave Cloud", currentInstanceName, currentInstanceID)
			} else {
				msg += "a different Weave Cloud instance."
			}
			confirmed, err := askForConfirmation(fmt.Sprintf(
				"\n%s\nWould you like to continue and connect this cluster to %q (id: %s) instead?", msg, InstanceName, InstanceID))
			if err != nil {
				exitWithCapture(opts, "Could not ask for confirmation: %s\n", err)
			} else if !confirmed {
				exitWithCapture(opts, "Installation cancelled")
			}
			_, err = kubectl.CreateSecretFromLiteral(kubectlClient, "weave", "weave-cloud", "token", opts.Token, true)
			if err != nil {
				exitWithCapture(opts, "There was an error creating the secret: %s\n", err)
			}
		}
	}

	// Apply the agent
	err = kubectl.Apply(kubectlClient, agentK8sURL)
	if err != nil {
		captureAndSend(opts, 1, "There was an error applying the agent: %s\n", err)

		// We've failed to apply the agent. kubectl apply isn't an atomic operation
		// can leave some objects behind when encountering an error. Clean things up.
		fmt.Println("Rolling back cluster changes")
		kubectl.Execute(kubectlClient, "delete", "--ignore-not-found=true", "-f", agentK8sURL)
		// Exit with a specific error which will be checked against in install script,
		// as a way of deduplicating sending of these errors.
		os.Exit(111)
	}

	fmt.Println("Successfully installed.")
}

func captureAndSend(opts options, skipFrames uint, msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	fmt.Fprintf(os.Stderr, formatted)
	// Send errors to UI.
	if opts.Scheme != "" {
		sendError(formatted, opts)
	}
	if opts.ReportErrors {
		sentry.CaptureAndWait(skipFrames, formatted, nil)
	}
}

func exitWithCapture(opts options, msg string, args ...interface{}) {
	captureAndSend(opts, 2, msg, args...)
	// Exit with a specific error which will be checked against in install script,
	// as a way of deduplicating sending of these errors.
	os.Exit(111)
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

func checkK8sVersion(kubectlClient kubectl.Client, opts options) {
	fmt.Println("Checking kubectl & kubernetes versions")
	clientVersion, serverVersion, err := kubectl.GetVersionInfo(kubectlClient)
	if clientVersion != "" {
		raven.SetTagsContext(map[string]string{
			"kubectl_clientVersion_gitVersion": clientVersion,
		})
		if serverVersion == "" {
			exitWithCapture(opts, "Error checking your kubernetes server version: %v. Please check that you can connect to your cluster by running \"kubectl version\".\n", err)
		} else {
			raven.SetTagsContext(map[string]string{
				"kubectl_serverVersion_gitVersion": serverVersion,
			})
		}
	} else {
		exitWithCapture(opts, "Error checking kubernetes version info: %v. Please check your environment for problems by running \"kubectl version\".\n", err)
	}

	// Validate cluster version of at least 1.6.0
	if !supportedK8sVersion(serverVersion) {
		exitWithCapture(opts, "Kubernetes version %s not supported. We require Kubernetes cluster to be version 1.6.0 or newer.\n", serverVersion)
	}
}

func supportedK8sVersion(clusterVersion string) bool {
	versionStr := strings.TrimLeft(clusterVersion, "v")
	version, err := semver.Parse(versionStr)
	if err != nil {
		return false
	}

	// We only support clusters 1.6.0 and higher.
	if version.Major == 1 && version.Minor < 6 {
		return false
	}

	return true
}

type errorResponse struct {
	Type     string   `json:"type"`
	Messages messages `json:"messages"`
}

type messages struct {
	Browser browser `json:"browser"`
}

type browser struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// sendError sends the error msg to UI.
func sendError(errMsg string, opts options) {
	eventTypeFailed := "onboarding_failed"
	response := errorResponse{
		Type: eventTypeFailed,
		Messages: messages{
			Browser: browser{
				Type: eventTypeFailed,
				Text: errMsg,
			},
		},
	}

	jr, err := json.Marshal(response)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%s://%s/api/notification/external/events", opts.Scheme, opts.WCHostname)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jr))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", opts.Token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}
