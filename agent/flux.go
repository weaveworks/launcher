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
	GitLabel string
	GitURL   string
	// This arg is multi-valued, or can be passed as comma-separated
	// values. This accounts for either form.
	GitPath         []string
	GitBranch       string
	GitTimeout      time.Duration
	GitPollInterval time.Duration
	GitSetAuthor    bool
	GitCISkip       bool

	// sync behaviour
	GarbageCollection bool

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
	MemcachedService string

	// Just in case we more explicitly support restricting Weave
	// Cloud, or just Flux to particular namespaces
	AllowNamespace []string

	fs *pflag.FlagSet
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
	for arg, val := range map[string]string{
		"git-label":         c.GitLabel,
		"git-url":           c.GitURL,
		"git-branch":        c.GitBranch,
		"memcached-service": c.MemcachedService,
	} {
		if c.fs.Changed(arg) {
			vals.Add(arg, val)
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
		for _, val := range deduplicate(slice) {
			vals.Add(arg, val)
		}
	}

	// duration-valued arguments
	for arg, dur := range map[string]time.Duration{
		"git-timeout":       c.GitTimeout,
		"git-poll-interval": c.GitPollInterval,
	} {
		if c.fs.Changed(arg) {
			vals.Add(arg, dur.String())
		}
	}

	for arg, flag := range map[string]bool{
		"sync-garbage-collection": c.GarbageCollection,
		"git-set-author":          c.GitSetAuthor,
		"git-ci-skip":             c.GitCISkip,
	} {
		if c.fs.Changed(arg) {
			vals.Add(arg, strconv.FormatBool(flag))
		}
	}

	return vals.Encode()
}

// ParseFluxArgs parses a string of args into a nice FluxConfig
func ParseFluxArgs(argString string) (*FluxConfig, error) {
	fs := pflag.NewFlagSet("default", pflag.ContinueOnError)
	fs.ParseErrorsWhitelist.UnknownFlags = true
	cfg := &FluxConfig{fs: fs}

	// strings
	fs.StringVar(&cfg.GitLabel, "git-label", "", "")
	fs.StringVar(&cfg.GitURL, "git-url", "", "")
	fs.StringVar(&cfg.GitBranch, "git-branch", "", "")
	fs.StringVar(&cfg.MemcachedService, "memcached-service", "", "")

	// durations
	fs.DurationVar(&cfg.GitTimeout, "git-timeout", time.Second, "")
	fs.DurationVar(&cfg.GitPollInterval, "git-poll-interval", time.Minute, "")

	// bools
	fs.BoolVar(&cfg.GitSetAuthor, "git-set-author", false, "")
	fs.BoolVar(&cfg.GitCISkip, "git-ci-skip", false, "")
	fs.BoolVar(&cfg.GarbageCollection, "sync-garbage-collection", false, "")

	// string slices
	fs.StringSliceVar(&cfg.GitPath, "git-path", nil, "")
	fs.StringSliceVar(&cfg.RegistryECRRegion, "registry-ecr-region", nil, "")
	fs.StringSliceVar(&cfg.RegistryRequire, "registry-require", nil, "")
	fs.StringSliceVar(&cfg.RegistryExcludeImage, "registry-exclude-image", nil, "")
	fs.StringSliceVar(&cfg.AllowNamespace, "k8s-allow-namespace", nil, "")

	// Parse it all
	fs.Parse(strings.Split(argString, " "))

	if fs.NFlag() > 0 {
		return cfg, nil
	}
	return nil, nil
}

// MemcachedConfig stores existing memcached arguments which will be
// used when updating WC agents
type MemcachedConfig struct {
	// Memory holds the -m argument value,
	// MB memory max to use for object storage
	Memory string
	// ItemSizeLimits holds the -I argument value,
	// the default size of each slab page, default is 1m,
	// minimum is 1k, max is 128m.
	ItemSizeLimit string

	fs *pflag.FlagSet
}

// AsQueryParams returns the configuration as a fragment of query
// string, so it can be interpolated into a text template.
func (c *MemcachedConfig) AsQueryParams() string {
	// Nothing clever here.
	vals := url.Values{}

	if c == nil {
		return ""
	}

	// String-valued arguments
	for arg, val := range map[string]string{
		"memcached-memory":    c.Memory,
		"memcached-item-size": c.ItemSizeLimit,
	} {
		if c.fs.Changed(arg) {
			vals.Add(arg, val)
		}
	}

	return vals.Encode()
}

// ParseMemcachedArgs parses a string of args into a nice
// MemcachedConfig
func ParseMemcachedArgs(argString string) (*MemcachedConfig, error) {
	fs := pflag.NewFlagSet("default", pflag.ContinueOnError)
	fs.ParseErrorsWhitelist.UnknownFlags = true
	cmg := &MemcachedConfig{fs: fs}

	// strings
	fs.StringVarP(&cmg.Memory, "memcached-memory", "m", "", "")
	fs.StringVarP(&cmg.ItemSizeLimit, "memcached-item-size", "I", "", "")

	// Parse it all
	fs.Parse(strings.Split(argString, " "))

	if fs.NFlag() > 0 {
		return cmg, nil
	}
	return nil, nil
}

func getFluxConfig(k kubectl.Client, namespace string) (*FluxConfig, error) {
	out, err := k.Execute("get", "deploy", "-n", namespace, "-l", "name=weave-flux-agent", "-o", "jsonpath='{.items[?(@.metadata.labels.name==\"weave-flux-agent\")].spec.template.spec.containers[0].args[*]}'")
	if err != nil {
		return nil, err
	}

	return ParseFluxArgs(out)
}

func getMemcachedConfig(k kubectl.Client, namespace string) (*MemcachedConfig, error) {
	out, err := k.Execute("get", "deploy", "-n", namespace, "-l", "name=weave-flux-memcached", "-o", "jsonpath='{.items[?(@.metadata.labels.name==\"weave-flux-memcached\")].spec.template.spec.containers[0].args[*]}'")
	if err != nil {
		return nil, err
	}

	return ParseMemcachedArgs(out)
}

func deduplicate(s []string) []string {
	if len(s) <= 1 {
		return s
	}

	res := []string{}
	seen := make(map[string]bool)
	for _, val := range s {
		if _, ok := seen[val]; !ok {
			res = append(res, val)
			seen[val] = true
		}
	}
	return res
}
