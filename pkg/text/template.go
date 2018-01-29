package text

import (
	"bytes"
	"text/template"
)

// ResolveString resolves the templated string using the context
func ResolveString(s string, ctx interface{}) (string, error) {
	tmpl, err := template.New("").Parse(s)
	if err != nil {
		return "", err
	}

	var result bytes.Buffer
	if err := tmpl.Execute(&result, ctx); err != nil {
		return "", err
	}

	return result.String(), nil
}
