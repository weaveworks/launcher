package kubectl

import (
	"fmt"
	"strings"
)

// Client implements a kubectl client to execute commands
type Client interface {
	Execute(args ...string) (string, error)
	ExecuteWithGlobalArgs(globalArgs []string, args ...string) (string, error)
	IsPresent() bool
}

// ClusterInfo describes a Kubernetes cluster
type ClusterInfo struct {
	Name          string
	ServerAddress string
}

// GetClusterInfo gets the current Kubernetes cluster information
func GetClusterInfo(c Client, otherArgs []string) (ClusterInfo, error) {
	currentContext, err := c.ExecuteWithGlobalArgs(otherArgs, "config", "current-context")
	if err != nil {
		return ClusterInfo{}, err
	}

	name, err := c.ExecuteWithGlobalArgs(otherArgs, "config", "view",
		fmt.Sprintf("-o=jsonpath='{.contexts[?(@.name == \"%s\")].context.cluster}'", currentContext),
	)
	if err != nil {
		return ClusterInfo{}, err
	}

	serverAddress, err := c.ExecuteWithGlobalArgs(otherArgs,
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
func Apply(c Client, f string, otherArgs []string) error {
	_, err := c.ExecuteWithGlobalArgs(otherArgs, "apply", "-f", f)
	return err
}

// ResourceExists return true if the resource exists
func ResourceExists(c Client, resourceType, namespace, resourceName string, otherArgs []string) (bool, error) {
	_, err := c.ExecuteWithGlobalArgs(otherArgs, "get", resourceType, resourceName, fmt.Sprintf("--namespace=%s", namespace))
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
func DeleteResource(c Client, resourceType, namespace, resourceName string, otherArgs []string) error {
	_, err := c.ExecuteWithGlobalArgs(otherArgs, "delete", resourceType, resourceName, fmt.Sprintf("--namespace=%s", namespace))
	return err
}

// CreateNamespace creates a new namespace and returns whether it was created or not
func CreateNamespace(c Client, namespace string, otherArgs []string) (bool, error) {
	_, err := c.ExecuteWithGlobalArgs(otherArgs, "create", "namespace", namespace)
	if err != nil {
		if strings.Contains(err.Error(), "AlreadyExists") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CreateClusterRoleBinding creates a new cluster role binding
func CreateClusterRoleBinding(c Client, name, role, user string, otherArgs []string) error {
	_, err := c.ExecuteWithGlobalArgs(
		otherArgs,
		"create",
		"clusterrolebinding",
		name,
		"--clusterrole",
		role,
		"--user",
		user,
	)
	return err
}

// CreateSecretFromLiteral creates a new secret with a single (key,value) pair.
func CreateSecretFromLiteral(c Client, namespace, name, key, value string, override bool, otherArgs []string) (bool, error) {
	secretExists, err := ResourceExists(c, "secret", namespace, name, otherArgs)
	if err != nil {
		return false, err
	}

	if secretExists {
		if !override {
			return false, nil
		}
		err := DeleteResource(c, "secret", namespace, name, otherArgs)
		if err != nil {
			return false, err
		}
	}

	// Create the weave namespace and the weave-cloud secret
	_, err = CreateNamespace(c, namespace, otherArgs)
	if err != nil {
		return false, err
	}

	// Create the secret
	_, err = c.ExecuteWithGlobalArgs(otherArgs,
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
