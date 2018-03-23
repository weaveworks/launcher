package kubectl

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
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
	cmd := exec.Command("kubectl", append(k.GlobalArgs, args...)...)
	_, stderr, combined, err := outputMatrix(cmd)
	if err != nil {
		// Kubectl error messages output to stdOut
		return "", fmt.Errorf("%s\nFull output:\n%s", trimOutput(stderr), trimOutput(combined))
	}
	return trimOutput(combined), nil
}

// ExecuteOutputMatrix executes kubectl <args> and returns stdout, stderr, and the combined interleaved output.
func (k LocalClient) ExecuteOutputMatrix(args ...string) (string, string, string, error) {
	cmd := exec.Command("kubectl", append(k.GlobalArgs, args...)...)
	return outputMatrix(cmd)
}

func outputMatrix(cmd *exec.Cmd) (string, string, string, error) {
	var stdoutBuf, stderrBuf, combinedBuf bytes.Buffer
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	stdoutWriter := io.MultiWriter(&combinedBuf, &stdoutBuf)
	stderrWriter := io.MultiWriter(&combinedBuf, &stderrBuf)

	var wg sync.WaitGroup
	copy := func(dst io.Writer, src io.Reader) {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
	}

	err := cmd.Start()
	if err == nil {
		wg.Add(2)
		go copy(stdoutWriter, stdout)
		go copy(stderrWriter, stderr)
		// we need to wait for all reads to finish before calling cmd.Wait
		wg.Wait()
		err = cmd.Wait()
	}
	return string(stdoutBuf.Bytes()), string(stderrBuf.Bytes()), string(combinedBuf.Bytes()), err
}

func trimOutput(output string) string {
	return strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(output), "'"), "'")
}
