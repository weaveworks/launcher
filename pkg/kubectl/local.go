package kubectl

import (
	"fmt"
	"os/exec"
	"strings"
)

// LocalClient implements Kubectl
type LocalClient struct{}

// IsPresent returns true if there's a kubectl command in the PATH.
func (k LocalClient) IsPresent() bool {
	_, err := exec.LookPath("kubectl")
	return err == nil
}

// Execute executes kubectl <args> and returns the combined stdout/err output.
func (k LocalClient) Execute(args ...string) (string, error) {
	return executeCommand(args)
}

// ExecuteWithGlobalArgs is a convenience version of Execute that lets the user
// specify global arguments as an array. Global arguments are arguments that are
// not specific to a kubectl sub-command, eg. --kubeconfig. The list of global
// options can be retrieved with kubectl options.
func (k LocalClient) ExecuteWithGlobalArgs(globalArgs []string, args ...string) (string, error) {
	return executeCommand(append(globalArgs, args...))
}

func executeCommand(args []string) (string, error) {
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
