package kubectl

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	podSuccess = "Succeeded"
	podFailure = "Failed"
)

type pod struct {
	Status status `json:"status"`
}

type status struct {
	Phase string `json:"phase"`
}

// Client implements a kubectl client to execute commands
type Client interface {
	Execute(args ...string) (string, error)
	ExecuteOutputMatrix(args ...string) (stdout, stderr, combined string, err error)
}

// Execute executes kubectl <args> and returns the combined stdout/err output.
func Execute(c Client, args ...string) (string, error) {
	return c.Execute(args...)
}

// ClusterInfo describes a Kubernetes cluster
type ClusterInfo struct {
	Name          string
	ServerAddress string
}

type kubeCtlVersionInfo struct {
	GitVersion string `json:"gitVersion"`
}

type kubeCtlVersionData struct {
	ClientVersion *kubeCtlVersionInfo `json:"clientVersion,omitempty"`
	ServerVersion *kubeCtlVersionInfo `json:"serverVersion,omitempty"`
}

// GetVersionInfo returns the version metadata from kubectl
// May return a value for the kubectl client version, despite also returning an error
func GetVersionInfo(c Client) (string, string, error) {
	// Capture stdout only (to ignore server reachability errors)
	stdout, stderr, _, err := c.ExecuteOutputMatrix("version", "--output=json")
	var versionData kubeCtlVersionData
	parseErr := json.Unmarshal([]byte(stdout), &versionData)
	// If the server is unreachable, we might have an error but parsable output
	if parseErr != nil {
		if err != nil {
			if stderr == "" {
				return "", "", err
			}
			return "", "", fmt.Errorf("kubectl error (%v): %s", err, stderr)
		}
		return "", "", fmt.Errorf("error parsing kubectl output: %v", parseErr)
	}
	var clientVersion, serverVersion string
	var outErr error
	if versionData.ClientVersion != nil {
		clientVersion = versionData.ClientVersion.GitVersion
	}
	if versionData.ServerVersion == nil {
		outErr = errors.New(stderr)
	} else {
		serverVersion = versionData.ServerVersion.GitVersion
	}
	return clientVersion, serverVersion, outErr
}

// GetClusterInfo gets the current Kubernetes cluster information
func GetClusterInfo(c Client) (ClusterInfo, error) {
	currentContext, err := Execute(c, "config", "current-context")
	if err != nil {
		return ClusterInfo{}, err
	}

	name, err := Execute(c, "config", "view",
		fmt.Sprintf("-o=jsonpath='{.contexts[?(@.name == \"%s\")].context.cluster}'", currentContext),
	)
	if err != nil {
		return ClusterInfo{}, err
	}

	serverAddress, err := Execute(c,
		"config",
		"view",
		fmt.Sprintf("-o=jsonpath='{.clusters[?(@.name == \"%s\")].cluster.server}'", name),
	)
	if err != nil {
		return ClusterInfo{}, err
	}

	return ClusterInfo{
		Name:          name,
		ServerAddress: serverAddress,
	}, nil
}

// Apply applies via kubectl
func Apply(c Client, f string) error {
	_, err := Execute(c, "apply", "-f", f)
	return err
}

// ResourceExists return true if the resource exists
func ResourceExists(c Client, resourceType, namespace, resourceName string) (bool, error) {
	_, err := Execute(c, "get", resourceType, resourceName, fmt.Sprintf("--namespace=%s", namespace))
	if err != nil {
		// k8s 1.4 answers with "Error from server: secrets "weave-cloud" not found"
		// More recent versions with "Error from server (NotFound): secrets "weave-cloud" not found
		errorText := err.Error()
		if strings.Contains(errorText, "NotFound") ||
			strings.Contains(errorText, "not found") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// DeleteResource deletes a resource
func DeleteResource(c Client, resourceType, namespace, resourceName string) error {
	_, err := Execute(c, "delete", resourceType, resourceName, fmt.Sprintf("--namespace=%s", namespace))
	return err
}

func isPodReady(c Client, podName, ns string) error {
	// Timeout is set for 1 minute, as Kubernetes requires some time to create a pod.
	timeout := time.After(1 * time.Minute)
	tick := time.Tick(1 * time.Second)

	for {
		select {
		case <-timeout:
			return errors.New("Timed out during DNS check.")
		case <-tick:
			ok, err := checkPod(c, podName, ns)
			if err != nil {
				return err
			} else if ok {
				return nil
			}
		}
	}
}

func checkPod(c Client, podName, ns string) (bool, error) {
	// Retrieve current pod data.
	podJSON, err := Execute(c, "get", "pod", podName, "-ojson", "-n", ns)
	if err != nil {
		return false, err
	}
	p := pod{}
	err = json.Unmarshal([]byte(podJSON), &p)
	if err != nil {
		return false, err
	}

	if p.Status.Phase != podSuccess && p.Status.Phase != podFailure {
		return false, nil
	}

	return true, nil
}

//TestDNS creates a pod where a nslookup is called on a provided domain. It returns true only if the pod was successful.
func TestDNS(c Client, domain string) (bool, error) {
	podName := "launcher-pre-flight"
	ns := "weave"

	// Create weave namespace, as this happens before any resources are created.
	_, err := CreateNamespace(c, ns)
	if err != nil {
		return false, err
	}

	// Create pod to perform nslookup on a passed domain to check DNS is working.
	_, err = Execute(c, "run", "-n", "weave", "--image", "busybox", "--command", podName, "nslookup --timeout=10", domain, "--restart=Never", "--pod-running-timeout=10s")
	if err != nil {
		return false, err
	}

	// Initially fetch the pod, which was created above.
	podJSON, err := Execute(c, "get", "pod", podName, "-ojson", "-n", ns)
	if err != nil {
		return false, err
	}
	p := pod{}
	err = json.Unmarshal([]byte(podJSON), &p)
	if err != nil {
		return false, err
	}

	if p.Status.Phase != podSuccess && p.Status.Phase != podFailure {
		// If the state has not been reached yet, we enter a retry phase.
		// In isPodReady function we retry to get pod status phase for a minute and then timeout.
		err := isPodReady(c, podName, ns)
		// Either an error occurred or timeout was reached.
		if err != nil {
			// Attempt to cleanup pod.
			_ = DeleteResource(c, "pod", ns, podName)
			return false, fmt.Errorf("DNS check failed. %v", err)
		}
	}

	// Get fresh pod data.
	podJSON, err = Execute(c, "get", "pod", podName, "-ojson", "-n", ns)
	if err != nil {
		return false, err
	}
	err = json.Unmarshal([]byte(podJSON), &p)
	if err != nil {
		return false, err
	}

	// If the final status of the pod was failed, we should return an error as DNS is not working.
	if p.Status.Phase == podFailure {
		err = DeleteResource(c, "pod", ns, podName)
		if err != nil {
			return false, err
		}
		return false, fmt.Errorf("DNS check failed. The DNS in the Kuberentes cluster is not working correctly.")
	}

	// This should not happen but still lets error out in case it does.
	if p.Status.Phase != podSuccess {
		return false, fmt.Errorf("DNS check failed.")
	}

	// Cleanup the pod.
	err = DeleteResource(c, "pod", ns, podName)
	if err != nil {
		// We should still return that DNS works, there was only a problem with deleting the resource.
		return true, err
	}

	// We are certain that pod is up and running so we return DNS okay.
	return true, nil
}

// CreateNamespace creates a new namespace and returns whether it was created or not
func CreateNamespace(c Client, namespace string) (bool, error) {
	_, err := Execute(c, "create", "namespace", namespace)
	if err != nil {
		if strings.Contains(err.Error(), "AlreadyExists") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CreateClusterRoleBinding creates a new cluster role binding
func CreateClusterRoleBinding(c Client, name, role, user string) error {
	_, err := Execute(
		c,
		"create",
		"clusterrolebinding",
		name,
		"--clusterrole",
		role,
		"--user",
		user,
	)
	if err != nil && !strings.Contains(err.Error(), "AlreadyExists") {
		return err
	}
	return nil
}

// CreateSecretFromLiteral creates a new secret with a single (key,value) pair.
func CreateSecretFromLiteral(c Client, namespace, name, key, value string, override bool) (bool, error) {
	secretExists, err := ResourceExists(c, "secret", namespace, name)
	if err != nil {
		return false, err
	}

	if secretExists {
		if !override {
			return false, nil
		}
		err := DeleteResource(c, "secret", namespace, name)
		if err != nil {
			return false, err
		}
	}

	// Create the weave namespace and the weave-cloud secret
	_, err = CreateNamespace(c, namespace)
	if err != nil {
		return false, err
	}

	// Create the secret
	_, err = Execute(c,
		fmt.Sprintf("--namespace=%s", namespace),
		"create",
		"secret",
		"generic",
		name,
		fmt.Sprintf("--from-literal=%s=%s", key, value),
	)
	if err != nil {
		return false, err
	}

	return true, nil
}

type secretManifest struct {
	Data map[string]string
}

// GetSecretValue returns the value of a secret
func GetSecretValue(c Client, namespace, name, key string) (string, error) {
	output, err := Execute(c, "get", "secret", name, fmt.Sprintf("--namespace=%s", namespace), "--output=json")
	if err != nil {
		return "", err
	}
	var secretDefn secretManifest
	err = json.Unmarshal([]byte(output), &secretDefn)
	if err != nil {
		return "", err
	}
	encoded, ok := secretDefn.Data[key]
	if !ok {
		return "", fmt.Errorf("Secret missing key %q", key)
	}
	valueBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(valueBytes), nil
}
