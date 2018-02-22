package kubectl

import (
	"fmt"
	"os/exec"
	"strings"
)

// LocalClient implements Kubectl
type LocalClient struct {
	GlobalArgs []string
}

// IsPresent returns true if there's a kubectl command in the PATH.
func (k LocalClient) IsPresent() bool {
	_, err := exec.LookPath("kubectl")
	return err == nil
}

// Execute executes kubectl <args> and returns the combined stdout/err output.
func (k LocalClient) Execute(args ...string) (string, error) {
	cmdOut, err := exec.Command("kubectl", append(k.GlobalArgs, args...)...).CombinedOutput()
	if err != nil {
		// Kubectl error messages output to stdOut
		return "", fmt.Errorf(formatCmdOutput(cmdOut))
	}
	return formatCmdOutput(cmdOut), nil
}

// ExecuteStdout executes kubectl <args> and returns the stdout output.
func (k LocalClient) ExecuteStdout(args ...string) (string, error) {
	cmdOut, err := exec.Command("kubectl", append(k.GlobalArgs, args...)...).Output()
	return string(cmdOut), err
}

func formatCmdOutput(output []byte) string {
	return strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(string(output)), "'"), "'")
}
