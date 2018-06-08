package main

import (
	"errors"
	"reflect"
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
			t.Errorf("Version was wrongl expected: %s got %s", c.version, v)
		}
	}
}

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
