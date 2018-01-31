package kubectl

import (
	"fmt"
	"os/exec"
	"strings"
)

func executeCommand(args []string) (string, error) {
	cmdOut, err := exec.Command("kubectl", args...).CombinedOutput()
	if err != nil {
		// Kubectl error messages output to stdOut
		return "", fmt.Errorf(formatCmdOutput(cmdOut))
	}
	return formatCmdOutput(cmdOut), nil
}

// Execute executes kubectl <args> and returns the combined stdout/err output.
func Execute(args ...string) (string, error) {
	return executeCommand(args)
}

// ExecuteWithGlobalArgs is a convenience version of Execute that lets the user
// specify global arguments as an array. Global arguments are arguments that are
// not specific to a kubectl sub-command, eg. --kubeconfig. The list of global
// options can be retrieved with kubectl options.
func ExecuteWithGlobalArgs(globalArgs []string, args ...string) (string, error) {
	return executeCommand(append(globalArgs, args...))
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
	currentContext, err := ExecuteWithGlobalArgs(otherArgs, "config", "current-context")
	if err != nil {
		return ClusterInfo{}, err
	}

	name, err := ExecuteWithGlobalArgs(otherArgs, "config", "view",
		fmt.Sprintf("-o=jsonpath='{.contexts[?(@.name == \"%s\")].context.cluster}'", currentContext),
	)
	if err != nil {
		return ClusterInfo{}, err
	}

	serverAddress, err := ExecuteWithGlobalArgs(otherArgs,
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

// CreateNamespace creates a new namespace and returns whether it was created or not
func CreateNamespace(namespace string, otherArgs []string) (bool, error) {
	_, err := ExecuteWithGlobalArgs(otherArgs, "create", "namespace", namespace)
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
	_, err := ExecuteWithGlobalArgs(otherArgs, "get", resourceType, resourceName, fmt.Sprintf("--namespace=%s", namespace))
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
