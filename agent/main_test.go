package main

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetMinorMajorVersion(t *testing.T) {
	cases := []struct {
		majorVersion string
		minorVersion string
		gitVersion   string
		version      string
		err          error
	}{
		{"1", "", "v1.9.0", "v1.9", nil},
		{"1", "9", "v1.9.0", "v1.9", nil},
		{"", "", "v", "", errors.New("kubernetes version not formatted correctly")},
	}

	for _, c := range cases {
		v, err := getMajorMinorVersion(c.majorVersion, c.minorVersion, c.gitVersion)
		if err != nil {

			if c.err != nil {
				if e, a := c.err, err; !reflect.DeepEqual(e, a) {
					t.Errorf("Unexpected error; expected %v, got %v", e, a)
					return
				}
				return
			}
			t.Error(err)
			return
		}
		if v != c.version {
			t.Errorf("Version was wrong; expected: %s got %s", c.version, v)
		}
	}
}

func TestAgentManifestURL(t *testing.T) {
	cfg := &agentConfig{
		AgentPollURLTemplate: "https://get.weave.works/k8s/agent.yaml",
		CRIEndpoint:          "/foo/bar",
	}

	manifestURL := agentManifestURL(cfg)
	v := url.Values{
		"cri-endpoint": []string{"/foo/bar"},
	}
	assert.Contains(t, manifestURL, v.Encode())
}

func TestAgentFluxURL(t *testing.T) {
	// Not an exhaustive test; just representative
	gitPath := []string{"config/helloworld", "config/hej-world"}

	argsStr := fmt.Sprintf(`--git-path=%s --memcached-service= --git-ci-skip=false --git-timeout=40s`, strings.Join(gitPath, ","))

	fluxCfg, err := ParseFluxArgs(argsStr)
	assert.NoError(t, err)

	cfg := &agentConfig{
		AgentPollURLTemplate: defaultWCPollURL,
		FluxConfig:           fluxCfg,
	}

	manifestURL := agentManifestURL(cfg)

	v := url.Values{
		"git-path":          gitPath,
		"memcached-service": []string{""},
		"git-ci-skip":       []string{"false"},
		"git-timeout":       []string{"40s"},
	}

	assert.Contains(t, manifestURL, v.Encode())
}

func TestParseFluxArgs(t *testing.T) {
	// nothing
	argString := ""
	fluxCfg, err := ParseFluxArgs(argString)
	assert.Equal(t, nil, err)
	assert.Equal(t, "", fluxCfg.AsQueryParams())

	// Test handling boolean flags w/out `=true|false`
	argString = "--git-ci-skip"
	fluxCfg, err = ParseFluxArgs(argString)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, fluxCfg.GitCISkip)

	// Test handling boolean flags w `=true|false`
	argString = "--git-ci-skip=true"
	fluxCfg, err = ParseFluxArgs(argString)
	assert.Equal(t, nil, err)
	assert.Equal(t, true, fluxCfg.GitCISkip)

	argString = "--git-ci-skip=false"
	fluxCfg, err = ParseFluxArgs(argString)
	assert.Equal(t, nil, err)
	assert.Equal(t, false, fluxCfg.GitCISkip)

	// Test we only serialize props that we provided
	argString = "--git-label=foo --git-path=derp"
	fluxCfg, err = ParseFluxArgs(argString)
	assert.Equal(t, nil, err)
	assert.Equal(t, "foo", fluxCfg.GitLabel)
	assert.Equal(t, "git-label=foo&git-path=derp", fluxCfg.AsQueryParams())

	// string[]
	argString = "--git-path=zing --git-path=derp"
	fluxCfg, err = ParseFluxArgs(argString)
	assert.Equal(t, nil, err)
	assert.Equal(t, "git-path=zing&git-path=derp", fluxCfg.AsQueryParams())

	// unknown
	argString = "--token=derp"
	fluxCfg, err = ParseFluxArgs(argString)
	assert.Equal(t, nil, err)
	assert.Equal(t, "", fluxCfg.AsQueryParams())

	// some unknown
	argString = "--git-path=zing --token=derp"
	fluxCfg, err = ParseFluxArgs(argString)
	assert.Equal(t, nil, err)
	assert.Equal(t, "git-path=zing", fluxCfg.AsQueryParams())

	// Preserves empty values
	argString = "--memcached-service="
	fluxCfg, err = ParseFluxArgs(argString)
	assert.Equal(t, nil, err)
	assert.Equal(t, "memcached-service=", fluxCfg.AsQueryParams())
}
