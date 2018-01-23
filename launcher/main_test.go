package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	Valid   = true
	Invalid = false
)

func TestResolveURL(t *testing.T) {
	tests := []struct {
		tmpl     string
		input    urlContext
		valid    bool
		expected string
	}{
		{"http://localhost/?k8s={{.KubernetesVersion}}", urlContext{KubernetesVersion: "1.8"}, Valid, "http://localhost/?k8s=1.8"},
		{"http://localhost/?t={{.Token}}", urlContext{Token: "Foo"}, Valid, "http://localhost/?t=Foo"},
		{"http://localhost/?t={{.Token}", urlContext{Token: "Foo"}, Invalid, "http://localhost/?t=Foo"},
	}

	for _, test := range tests {
		got, err := resolveURL(test.tmpl, test.input)
		if !test.valid {
			assert.NotNil(t, err)
			continue
		}
		assert.Nil(t, err)
		assert.Equal(t, test.expected, got)
	}
}
