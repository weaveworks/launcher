package main

import (
	"errors"
	"net/url"
	"reflect"
	"testing"
	"time"

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
	memcachedService := ""
	gitCISkip := false
	gitTimeout := 40 * time.Second

	fluxCfg := &FluxConfig{
		GitPath:          gitPath,
		MemcachedService: &memcachedService,
		GitCISkip:        &gitCISkip,
		GitTimeout:       &gitTimeout,
	}

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
