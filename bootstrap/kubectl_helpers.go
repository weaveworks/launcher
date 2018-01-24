package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func executeKubectlCommand(args []string) (string, error) {
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

type clusterInfo struct {
	Name          string
	ServerAddress string
}

func getClusterInfo(otherArgs []string) (clusterInfo, error) {
	currentContext, err := executeKubectlCommand(
		append([]string{"config", "current-context"}, otherArgs...),
	)

	if err != nil {
		return clusterInfo{}, err
	}

	name, err := executeKubectlCommand(
		append([]string{
			"config",
			"view",
			fmt.Sprintf("-o=jsonpath='{.contexts[?(@.name == \"%s\")].context.cluster}'", currentContext),
		}, otherArgs...),
	)
	if err != nil {
		return clusterInfo{}, err
	}

	serverAddress, err := executeKubectlCommand(
		append([]string{
			"config",
			"view",
			fmt.Sprintf("-o=jsonpath='{.clusters[?(@.name == \"%s\")].cluster.server}'", name),
		}, otherArgs...),
	)
	if err != nil {
		return clusterInfo{}, err
	}

	return clusterInfo{
		Name:          name,
		ServerAddress: serverAddress,
	}, nil
}
