package kubectl

import (
	"fmt"
	"os/exec"
	"strings"
)

// ExecuteCommand executes kubectl <args> and returns the formatted output or error
func ExecuteCommand(args []string) (string, error) {
	cmdOut, err := exec.Command("kubectl", args...).CombinedOutput()
	if err != nil {
		// Kubectl error messages output to stdOut
		return "", fmt.Errorf(formatCmdOutput(cmdOut))
	}
	return formatCmdOutput(cmdOut), nil
}

func formatCmdOutput(output []byte) string {
	return strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(string(output)), "'"), "'")
}

// ClusterInfo describes a Kubernetes cluster
type ClusterInfo struct {
	Name          string
	ServerAddress string
}

// GetClusterInfo gets the current Kubernetes cluster information
func GetClusterInfo(otherArgs []string) (ClusterInfo, error) {
	currentContext, err := ExecuteCommand(
		append([]string{"config", "current-context"}, otherArgs...),
	)

	if err != nil {
		return ClusterInfo{}, err
	}

	name, err := ExecuteCommand(
		append([]string{
			"config",
			"view",
			fmt.Sprintf("-o=jsonpath='{.contexts[?(@.name == \"%s\")].context.cluster}'", currentContext),
		}, otherArgs...),
	)
	if err != nil {
		return ClusterInfo{}, err
	}

	serverAddress, err := ExecuteCommand(
		append([]string{
			"config",
			"view",
			fmt.Sprintf("-o=jsonpath='{.clusters[?(@.name == \"%s\")].cluster.server}'", name),
		}, otherArgs...),
	)
	if err != nil {
		return ClusterInfo{}, err
	}

	return ClusterInfo{
		Name:          name,
		ServerAddress: serverAddress,
	}, nil
}

// CreateNamespace creates a new namespace and returns whether it was created or not
func CreateNamespace(namespace string, otherArgs []string) (bool, error) {
	_, err := ExecuteCommand(
		append([]string{
			"create",
			"namespace",
			namespace,
		}, otherArgs...),
	)
	if err != nil {
		if strings.Contains(err.Error(), "AlreadyExists") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ResourceExists return true if the resource exists
func ResourceExists(resourceType, resourceName, namespace string, otherArgs []string) (bool, error) {
	_, err := ExecuteCommand(
		append([]string{
			"get",
			resourceType,
			resourceName,
			fmt.Sprintf("--namespace=%s", namespace),
		}, otherArgs...),
	)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
