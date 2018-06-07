package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testValidateResources(t *testing.T) {
	tests := []struct {
		resources []string
		valid     bool
	}{
		{[]string{}, false},
		{[]string{"rds"}, true},
		{[]string{"rds", "classic-elb"}, true},
		{[]string{"foo", "classic-elb"}, false},
		{[]string{"foo"}, false},
	}

	for _, test := range tests {
		assert.Equal(t, test.valid, validateResources(test.resources))
	}
}

var testCloudwatchConfig = `
region: us-east-1
secretName: cloudwatch
resources:
  - rds
  - classic-elb
`

var testCloudwatchConfigNoResources = `
region: us-east-1
secretName: cloudwatch
`

var testCloudwatchConfigUnknownResource = `
region: us-east-1
secretName: cloudwatch
resources:
  - rds
  - foo
`

func TestParseCloudwatchYaml(t *testing.T) {
	tests := []struct {
		input    string
		valid    bool
		expected cloudwatch
	}{
		{
			testCloudwatchConfig,
			true,
			cloudwatch{
				Region:     "us-east-1",
				SecretName: "cloudwatch",
				Resources:  []string{"rds", "classic-elb"},
			},
		},
		{
			testCloudwatchConfigNoResources,
			false,
			cloudwatch{},
		},
		{
			testCloudwatchConfigUnknownResource,
			false,
			cloudwatch{},
		},
	}

	for _, test := range tests {
		got, err := parseCloudwatchYaml(test.input)

		if !test.valid {
			assert.NotNil(t, err)
			continue
		}

		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, test.expected, *got)
	}
}
