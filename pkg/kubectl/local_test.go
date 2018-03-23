package kubectl

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatCmdOutput(t *testing.T) {
	tests := []struct {
		input    []byte
		expected string
	}{
		{[]byte("foo"), "foo"},
		{[]byte("'foo'"), "foo"},
		{[]byte("  'foo'  "), "foo"},
	}

	for _, tc := range tests {
		if output := formatCmdOutput(tc.input); output != tc.expected {
			t.Errorf("Expected %s, got: %s", tc.expected, output)
		}
	}
}

func ExampleLocalClient() {
	local := LocalClient{}
	local.Execute("apply", "-f", "service.yaml")
}

func TestOutputMatrix(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "echo stdout; sleep 0.001; echo stderr >&2; sleep 0.001; echo stdout")

	stdout, stderr, combined, err := outputMatrix(cmd)
	assert.Equal(t, "stdout\nstdout\n", stdout)
	assert.Equal(t, "stderr\n", stderr)
	assert.Equal(t, "stdout\nstderr\nstdout\n", combined)
	assert.NoError(t, err)
}
