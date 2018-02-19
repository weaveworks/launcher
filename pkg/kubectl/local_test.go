package kubectl

import (
	"testing"
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
