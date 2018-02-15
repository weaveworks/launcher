package gcloud

import (
	"os/exec"
	"strings"
)

// IsPresent returns true if there's a kubectl command in the PATH.
func IsPresent() bool {
	_, err := exec.LookPath("gcloud")
	return err == nil
}

func executeCommand(args ...string) (string, error) {
	cmdOut, err := exec.Command("gcloud", args...).CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(cmdOut), nil
}

// GetConfigValue returns the value of
func GetConfigValue(name string) (string, error) {
	value, err := executeCommand("config", "get-value", name, "--quiet", "--verbosity=none")
	if err != nil {
		return "", err
	}

	// gcloud sometimes outputs warnings, even with --verbosity=none
	lines := strings.Split(value, "\n")
	for _, line := range lines {
		if strings.Contains(line, "(unset)") {
			return "", nil
		}
		if strings.Contains(line, "WARNING") {
			continue
		}
		return line, nil
	}
	return "", nil
}
