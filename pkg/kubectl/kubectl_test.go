package kubectl

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestClient struct {
	responses map[string]string
}

func NewTestClient() *TestClient {
	tc := &TestClient{}
	tc.responses = map[string]string{}
	return tc
}

func (t *TestClient) Execute(args ...string) (string, error) {
	cmd := strings.Join(args, " ")
	response, ok := t.responses[cmd]
	if ok {
		return response, nil
	}
	return "", fmt.Errorf("Missing response for %q", cmd)
}

func (t *TestClient) ExecuteOutputMatrix(args ...string) (stdout, stderr, combined string, err error) {
	stdout, err = Execute(t, args...)
	return stdout, "", stdout, err
}

func TestGetSecretValue(t *testing.T) {
	tc := NewTestClient()

	json := `{"data":{"token": "c2VjcmV0IQ=="}}`
	tc.responses["get secret weave-cloud --namespace=weave --output=json"] = json
	res, err := GetSecretValue(tc, "weave", "weave-cloud", "token")
	assert.Equal(t, res, "secret!")
	assert.NoError(t, err)
}
