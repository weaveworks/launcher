package main

import (
	"errors"
	"reflect"
	"testing"
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
