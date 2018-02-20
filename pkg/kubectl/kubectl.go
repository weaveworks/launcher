package kubectl

import (
	"fmt"
	"strings"
)

// Client implements a kubectl client to execute commands
type Client interface {
	Execute(args ...string) (string, error)
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
	return err
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
