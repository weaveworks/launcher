package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"
)

const usage = `Usage: templatinator TEMPLATE < INPUT

templatinator reads a go template from the TEMPLATE file and use the values
stored in the JSON INPUT file to execute the template.

Example:
    $ cat test.template
    http://{{.Hostname}}:{{.Port}}{{.Path}}
    $ cat test-input.json
    {
      "Hostname": "localhost",
      "Port": 8080,
      "Path": "/index.html"
    }
    $ go run cmd/templatinator/templatinator.go test.template < test-input.json
    http://localhost:8080/index.html
`

var funcMap = template.FuncMap{
	"toUpper": strings.ToUpper,
	"toLower": strings.ToLower,
	"toTitle": strings.ToTitle,
	"join":    strings.Join,
}

func die(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

func main() {
	if len(os.Args) != 2 {
		die(usage)
	}

	template, err := template.ParseFiles(os.Args[1])
	if err != nil {
		die("could not parse template file: %v", err)
	}

	template.Funcs(funcMap)

	var input map[string]interface{}
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		die("could not decode input JSON: %v", err)
	}

	if err := template.Execute(os.Stdout, &input); err != nil {
		die("could not execute template: %v", err)
	}
}
