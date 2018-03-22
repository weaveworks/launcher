package kubectl

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// Client implements a kubectl client to execute commands
type Client interface {
	Execute(args ...string) (string, error)
	ExecuteStdout(args ...string) (string, error)
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
func GetVersionInfo(c Client) (string, string, error) {
	// Capture stdout only (to ignore server reachability errors)
	output, err := c.ExecuteStdout("version", "--output=json")
	var versionData kubeCtlVersionData
	parseErr := json.Unmarshal([]byte(output), &versionData)
	// If the server is unreachable, we might have an error but parsable output
	if parseErr != nil {
		if err != nil {
			return "", "", err
		}
		return "", "", parseErr
	}
	var clientVersion, serverVersion string
	if versionData.ClientVersion != nil {
		clientVersion = versionData.ClientVersion.GitVersion
	}
	if versionData.ServerVersion != nil {
		serverVersion = versionData.ServerVersion.GitVersion
	}
	return clientVersion, serverVersion, nil
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
