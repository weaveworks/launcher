package main

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	flags "github.com/jessevdk/go-flags"
	"github.com/weaveworks/launcher/pkg/kubectl"
)

// FluxConfig stores existing flux arguments which will be used when updating WC agents
type FluxConfig struct {
	// git parameters
	GitLabel *string `long:"git-label"`
	GitURL   *string `long:"git-url"`
	// This arg is multi-valued, or can be passed as comma-separated
	// values. This accounts for either form.
	GitPath         []string       `long:"git-path"`
	GitBranch       *string        `long:"git-branch"`
	GitTimeout      *time.Duration `long:"git-timeout"`
	GitPollInterval *time.Duration `long:"git-poll-interval"`
	GitSetAuthor    *bool          `long:"git-set-author"`
	GitCISkip       *bool          `long:"git-ci-skip"`

	// sync behaviour
	GarbageCollection *bool `long:"sync-garbage-collection"`

	// For specifying ECR region from outside AWS (fluxd detects it when inside AWS)
	RegistryECRRegion []string `long:"registry-ecr-region"`
	// For requiring a particular registry to be accessible, else crash
	RegistryRequire []string `long:"registry-require"`
	// Can be used to switch image registry scanning off for some or
	// all images (with glob patterns)
	RegistryExcludeImage []string `long:"registry-exclude-image"`

	// This is now hard-wired to empty in launch-generator, to tell flux
	// _not_ to use service discovery. But: maybe someone needs to use
	// service discovery.
	MemcachedService *string `long:"memcached-service"`

	// Just in case we more explicitly support restricting Weave
	// Cloud, or just Flux to particular namespaces
	AllowNamespace []string `long:"k8s-allow-namepace"`
}

// AsQueryParams returns the configuration as a fragment of query
// string, so it can be interpolated into a text template.
func (c *FluxConfig) AsQueryParams() string {
	// Nothing clever here.
	vals := url.Values{}

	// String-valued arguments
	for arg, val := range map[string]*string{
		"git-label":         c.GitLabel,
		"git-url":           c.GitURL,
		"git-branch":        c.GitBranch,
		"memcached-service": c.MemcachedService,
	} {
		if val != nil {
			vals.Add(arg, *val)
		}
	}

	// []string-valued arguments
	for arg, slice := range map[string][]string{
		"git-path":               c.GitPath,
		"registry-ecr-region":    c.RegistryECRRegion,
		"registry-require":       c.RegistryRequire,
		"registry-exclude-image": c.RegistryExcludeImage,
		"k8s-allow-namespace":    c.AllowNamespace,
	} {
		for _, val := range slice {
			vals.Add(arg, val)
		}
	}

	// duration-valued arguments
	for arg, dur := range map[string]*time.Duration{
		"git-timeout":       c.GitTimeout,
		"git-poll-interval": c.GitPollInterval,
	} {
		if dur != nil {
			vals.Add(arg, dur.String())
		}
	}

	for arg, flag := range map[string]*bool{
		"sync-garbage-collection": c.GarbageCollection,
		"git-set-author":          c.GitSetAuthor,
		"git-ci-skip":             c.GitCISkip,
	} {
		if flag != nil {
			vals.Add(arg, strconv.FormatBool(*flag))
		}
	}

	return vals.Encode()
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

	return cfg, nil
}
