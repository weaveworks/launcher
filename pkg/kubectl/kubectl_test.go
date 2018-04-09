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

const (
	outputVersion = `Client Version: version.Info{Major:"1", Minor:"6", GitVersion:"v1.6.13", GitCommit:"14ea65f53cdae4a5657cf38cfc8d7349b75b5512", GitTreeState:"clean", BuildDate:"2017-11-22T20:29:21Z", GoVersion:"go1.7.6", Compiler:"gc", Platform:"linux/amd64"}
Server Version: version.Info{Major:"1", Minor:"9", GitVersion:"v1.9.3", GitCommit:"d2835416544f298c919e2ead3be3d0864b52323b", GitTreeState:"clean", BuildDate:"2018-02-07T11:55:20Z", GoVersion:"go1.9.2", Compiler:"gc", Platform:"linux/amd64"}
`
	outputClientOnly = `Client Version: version.Info{Major:"1", Minor:"9", GitVersion:"v1.10.0", GitCommit:"d2835416544f298c919e2ead3be3d0864b52323b", GitTreeState:"clean", BuildDate:"2018-02-07T12:22:21Z", GoVersion:"go1.9.2", Compiler:"gc", Platform:"linux/amd64"}
`
	outputTruncated = `Client Version: version.Info{Major:"1", Minor:"9", GitVersion:"v`
)

func TestParseVersionOutput(t *testing.T) {
	tests := []struct {
		stdout                       string
		valid                        bool
		clientVersion, serverVersion string
	}{
		{outputVersion, true, "v1.6.13", "v1.9.3"},
		{outputClientOnly, true, "v1.10.0", ""},
		// Badly formatted things, will trigger a panic and exercise the defer/recover path.
		{outputTruncated, false, "", ""},
	}

	for _, test := range tests {
		clientVersion, serverVersion, err := parseVersionOutput(test.stdout)
		if !test.valid {
			assert.NotNil(t, err)
			continue
		}

		assert.NoError(t, err)
		assert.Equal(t, test.clientVersion, clientVersion)
		assert.Equal(t, test.serverVersion, serverVersion)
	}
}
