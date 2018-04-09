package kubectl

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type deployment struct {
	Status status `json:"status"`
}

type status struct {
	UnavailableReplicas int `json:"unavailableReplicas"`
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

// Example stdout:
// Client Version: version.Info{Major:"1", Minor:"9", GitVersion:"v1.9.2", ..., Platform:"linux/amd64"}
// Server Version: version.Info{Major:"1", Minor:"9", GitVersion:"v1.9.3", ..., Platform:"linux/amd64"}

// We don't care about the exact reason why the parsing failed, we'll display
// more context in the error message anyway.
var errParsing = errors.New("parse error")

func parseVersionLine(line string) (string, error) {
	// Only interested in what's between '{', '}'
	idx := strings.Index(line, "{")
	list := line[idx+1 : len(line)-2]

	parts := strings.Split(list, ",")
	for _, part := range parts {
		// parts are of the form key:"value"
		part := strings.TrimSpace(part)
		colon := strings.Index(part, ":")
		if colon == -1 {
			return "", errParsing
		}
		key := part[0:colon]

		if key == "GitVersion" {
			value := part[colon+2 : len(part)-1]
			return value, nil
		}
	}

	return "", nil
}

func parseVersionOutput(stdout string) (clientVersion, serverVersion string, err error) {
	// Protect against invalid input triggering panics (eg. out of bounds).
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("error parsing: %s", stdout)
		}
	}()

	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")

	// Associate line prefixes to their respective version.
	a := []struct {
		prefix, version string
	}{
		{"Client Version:", ""},
		{"Server Version:", ""},
	}
	for _, line := range lines {
		for i := range a {
			if strings.HasPrefix(line, a[i].prefix) {
				version, err := parseVersionLine(line)
				if err != nil {
					return "", "", fmt.Errorf("error parsing: %s", line)
				}
				a[i].version = version
			}
		}
	}

	return a[0].version, a[1].version, nil
}

// GetVersionInfo returns the version metadata from kubectl
// May return a value for the kubectl client version, despite also returning an error
func GetVersionInfo(c Client) (string, string, error) {
	// Capture stdout only (to ignore server reachability errors)
	stdout, stderr, _, err := c.ExecuteOutputMatrix("version")
	clientVersion, serverVersion, parseErr := parseVersionOutput(stdout)
	// If the server is unreachable, we might have an error but parsable output
	if parseErr != nil {
		if err != nil {
			if stderr == "" {
				return "", "", err
			}
			return "", "", fmt.Errorf("kubectl error (%v): %s", err, stderr)
		}
		return "", "", fmt.Errorf("error parsing kubectl version output: %s", parseErr)
	}

	// kubectl couldn't contact the server.
	if serverVersion == "" {
		err = errors.New(stderr)
	}
	return clientVersion, serverVersion, err
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

// DeploymentAvailability gets the current deployment status and returs true if all the replicas are up and running.
func DeploymentAvailability(c Client, name, namespace string) (bool, error) {
	// Get the deployment information in json format, so it can be easily parsed.
	deploymentJson, err := Execute(c, "get", "deployment", name, "-ojson", "-n", namespace)
	if err != nil {
		return false, err
	}

	d := deployment{}
	err = json.Unmarshal([]byte(deploymentJson), &d)
	if err != nil {
		return false, err
	}

	if d.Status.UnavailableReplicas > 0 {
		return false, nil
	}

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
