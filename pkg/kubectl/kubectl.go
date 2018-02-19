package kubectl

import (
	"encoding/json"
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

// IsPresent returns true if there's a kubectl command in the PATH.
func IsPresent() bool {
	_, err := exec.LookPath("kubectl")
	return err == nil
}

// GetVersionInfo returns the version metadata from kubectl
func GetVersionInfo() (map[string]string, error) {
	// Capture stdout only (to ignore server reachability errors)
	output, err := exec.Command("kubectl", "version", "-ojson").Output()
	versionData := map[string]interface{}{}
	parseErr := json.Unmarshal(output, &versionData)
	// If the server is unreachable, we might have an error but parsable output
	if parseErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, parseErr
	}
	out := make(map[string]string)
	for key, maybeValuesMap := range versionData {
		valueMap, ok := maybeValuesMap.(map[string]interface{})
		if ok {
			for subKey, value := range valueMap {
				if str, ok := value.(string); ok {
					out[fmt.Sprintf("kubectl_%s_%s", key, subKey)] = str
				}
			}
		}
	}
	return out, nil
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

// CreateSecretFromLiteral creates a new secret with a single (key,value) pair.
func CreateSecretFromLiteral(namespace, secretName, key, value string, otherArgs []string) (string, error) {
	return ExecuteWithGlobalArgs(otherArgs,
		fmt.Sprintf("--namespace=%s", namespace),
		"create",
		"secret",
		"generic",
		secretName,
		fmt.Sprintf("--from-literal=%s=%s", key, value),
	)
}

// ResourceExists return true if the resource exists
func ResourceExists(resourceType, resourceName, namespace string, otherArgs []string) (bool, error) {
	_, err := ExecuteWithGlobalArgs(otherArgs, "get", resourceType, resourceName, fmt.Sprintf("--namespace=%s", namespace))
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
