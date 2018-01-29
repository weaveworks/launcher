package text

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	Valid   = true
	Invalid = false
)

type context struct {
	Foo string
	Bar string
}

func TestResolveString(t *testing.T) {
	tests := []struct {
		tmpl     string
		input    interface{}
		valid    bool
		expected string
	}{
		{"", context{}, Valid, ""},
		{"Hello world! {{.Foo}}.", context{Foo: "foo"}, Valid, "Hello world! foo."},
		{"Hello world! {{.Foo}} and {{.Bar}}.", context{Foo: "foo", Bar: "bar"}, Valid, "Hello world! foo and bar."},
		{"Hello world! {{.Foo}.", context{Foo: "foo"}, Invalid, "Hello world! foo."},
	}

	for _, test := range tests {
		got, err := ResolveString(test.tmpl, test.input)
		if !test.valid {
			assert.NotNil(t, err)
			continue
		}
		assert.Nil(t, err)
		assert.Equal(t, test.expected, got)
	}
}
