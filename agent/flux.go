package main

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/weaveworks/launcher/pkg/kubectl"
)

// FluxConfig stores existing flux arguments which will be used when updating WC agents
type FluxConfig struct {
	// git parameters
	GitLabel *string
	GitURL   *string
	// This arg is multi-valued, or can be passed as comma-separated
	// values. This accounts for either form.
	GitPath         []string
	GitBranch       *string
	GitTimeout      *time.Duration
	GitPollInterval *time.Duration
	GitSetAuthor    *bool
	GitCISkip       *bool

	// sync behaviour
	GarbageCollection *bool

	// For specifying ECR region from outside AWS (fluxd detects it when inside AWS)
	RegistryECRRegion []string
	// For requiring a particular registry to be accessible, else crash
	RegistryRequire []string
	// Can be used to switch image registry scanning off for some or
	// all images (with glob patterns)
	RegistryExcludeImage []string

	// This is now hard-wired to empty in launch-generator, to tell flux
	// _not_ to use service discovery. But: maybe someone needs to use
	// service discovery.
	MemcachedService *string

	// Just in case we more explicitly support restricting Weave
	// Cloud, or just Flux to particular namespaces
	AllowNamespace []string
}

// AsQueryParams returns the configuration as a fragment of query
// string, so it can be interpolated into a text template.
func (c *FluxConfig) AsQueryParams() string {
	// Nothing clever here.
	vals := url.Values{}

	if c == nil {
		return ""
	}

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

func isFlagPassed(fs *pflag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *pflag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

// ParseFluxArgs parses a string of args into a nice FluxConfig
func ParseFluxArgs(argString string) (*FluxConfig, error) {
	cfg := &FluxConfig{}

	fs := pflag.NewFlagSet("default", pflag.ContinueOnError)
	fs.ParseErrorsWhitelist.UnknownFlags = true

	// strings
	cfg.GitLabel = fs.String("git-label", "", "")
	cfg.GitURL = fs.String("git-url", "", "")
	cfg.GitBranch = fs.String("git-branch", "", "")
	cfg.MemcachedService = fs.String("memcached-service", "", "")

	// durations
	cfg.GitTimeout = fs.Duration("git-timeout", time.Second, "")
	cfg.GitPollInterval = fs.Duration("git-poll-interval", time.Minute, "")

	// bools
	cfg.GitSetAuthor = fs.Bool("git-set-author", false, "")
	cfg.GitCISkip = fs.Bool("git-ci-skip", false, "")
	cfg.GarbageCollection = fs.Bool("sync-garbage-collection", false, "")

	// string slices
	fs.StringSliceVar(&cfg.GitPath, "git-path", nil, "")
	fs.StringSliceVar(&cfg.RegistryECRRegion, "registry-ecr-region", nil, "")
	fs.StringSliceVar(&cfg.RegistryRequire, "registry-require", nil, "")
	fs.StringSliceVar(&cfg.RegistryExcludeImage, "registry-exclude-image", nil, "")
	fs.StringSliceVar(&cfg.AllowNamespace, "k8s-allow-namespace", nil, "")

	// Parse it all
	fs.Parse(strings.Split(argString, " "))

	// Cleanup anything that wasn't explicitly set
	for arg, val := range map[string]**string{
		"git-label":         &cfg.GitLabel,
		"git-url":           &cfg.GitURL,
		"git-branch":        &cfg.GitBranch,
		"memcached-service": &cfg.MemcachedService,
	} {
		if !isFlagPassed(fs, arg) {
			*val = nil
		}
	}
	for arg, val := range map[string]**time.Duration{
		"git-timeout":       &cfg.GitTimeout,
		"git-poll-interval": &cfg.GitPollInterval,
	} {
		if !isFlagPassed(fs, arg) {
			*val = nil
		}
	}
	for arg, val := range map[string]**bool{
		"sync-garbage-collection": &cfg.GarbageCollection,
		"git-set-author":          &cfg.GitSetAuthor,
		"git-ci-skip":             &cfg.GitCISkip,
	} {
		if !isFlagPassed(fs, arg) {
			*val = nil
		}
	}

	// Did we find anything?
	if cfg.AsQueryParams() == "" {
		return nil, nil
	}

	return cfg, nil
}

func getFluxConfig(k kubectl.Client, namespace string) (*FluxConfig, error) {
	out, err := k.Execute("get", "pod", "-n", namespace, "-l", "name=weave-flux-agent", "-o", "jsonpath='{.items[?(@.metadata.labels.name==\"weave-flux-agent\")].spec.containers[0].args[*]}'")
	if err != nil {
		return nil, err
	}

	return ParseFluxArgs(out)
}
