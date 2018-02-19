package main

import (
	"strings"

	flags "github.com/jessevdk/go-flags"
	"github.com/weaveworks/launcher/pkg/kubectl"
)

// FluxConfig stores existing flux arguments which will be used when updating WC agents
type FluxConfig struct {
	GitLabel  string `long:"git-label"`
	GitURL    string `long:"git-url"`
	GitPath   string `long:"git-path"`
	GitBranch string `long:"git-branch"`
}

func getFluxConfig(k kubectl.Client, namespace string) (*FluxConfig, error) {
	out, err := k.Execute("get", "pod", "-n", namespace, "-l", "name=weave-flux-agent", "-o", "jsonpath='{.items[?(@.metadata.labels.name==\"weave-flux-agent\")].spec.containers[0].args[*]}'")
	if err != nil {
		return nil, err
	}

	cfg := &FluxConfig{}
	parser := flags.NewParser(cfg, flags.IgnoreUnknown)
	_, err = parser.ParseArgs(strings.Split(out, " "))
	if err != nil {
		return nil, err
	}

	if cfg.GitBranch == "" && cfg.GitLabel == "" && cfg.GitPath == "" && cfg.GitURL == "" {
		return nil, nil
	}

	return cfg, nil
}
