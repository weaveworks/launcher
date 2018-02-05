package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInstallHandler(t *testing.T) {
	installScript := "#!/bin/sh\necho \"Hello world!\""
	handlers := &Handlers{
		bootstrapVersion:  "aaa000",
		installScriptData: []byte(installScript),
	}

	server := httptest.NewServer(http.HandlerFunc(handlers.install))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200 status code, got: %d", resp.StatusCode)
	}

	// Check install script data in body
	actual, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if installScript != string(actual) {
		t.Errorf("Expected body '%s', got: '%s'", installScript, actual)
	}

	contentDisposition := resp.Header.Get("Content-Disposition")
	if contentDisposition != "attachment; filename=\"install-weave-cloud.sh\"" {
		t.Errorf("Expected Content-Disposition: attachment, got: '%s'", contentDisposition)
	}
}

func TestBootstrapHandler(t *testing.T) {
	handlers := &Handlers{
		bootstrapVersion:  "aaa000",
		installScriptData: []byte{},
	}
	bootstrapHandler := http.HandlerFunc(handlers.bootstrap)

	testCases := []struct {
		queryString        string
		expectedStatusCode int
		expectedLocation   string
	}{
		{"", 400, ""},
		{"?dist=darwin", 301, "https://weaveworks-launcher.s3.amazonaws.com/bootstrap/aaa000/bootstrap_darwin_amd64"},
		{"?dist=linux", 301, "https://weaveworks-launcher.s3.amazonaws.com/bootstrap/aaa000/bootstrap_linux_amd64"},
		{"?dist=other", 400, ""},
	}

	for _, tc := range testCases {
		// Record request made with queryString
		req, err := http.NewRequest("GET", tc.queryString, nil)
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		bootstrapHandler.ServeHTTP(rr, req)

		// Check status code
		if rr.Code != tc.expectedStatusCode {
			t.Errorf("Expected %d status code, got: %d", tc.expectedStatusCode, rr.Code)
		}

		// Check redirect location
		if tc.expectedLocation != "" {
			location, err := rr.Result().Location()
			if err != nil {
				t.Fatal(err)
			}

			if location.String() != tc.expectedLocation {
				t.Errorf("Expected location '%s', got: '%s'", tc.expectedLocation, location)
			}
		}
	}
}

func TestAgentYAMLHandler(t *testing.T) {
	agentManifest := "---\napiVersion: extensions/v1beta1"
	handlers := &Handlers{
		bootstrapVersion:  "aaa000",
		agentManifestData: []byte(agentManifest),
	}

	server := httptest.NewServer(http.HandlerFunc(handlers.agentYAML))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200 status code, got: %d", resp.StatusCode)
	}

	// Check install script data in body
	actual, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if agentManifest != string(actual) {
		t.Errorf("Expected body '%s', got: '%s'", agentManifest, actual)
	}

	contentDisposition := resp.Header.Get("Content-Disposition")
	if contentDisposition != "attachment" {
		t.Errorf("Expected Content-Disposition: attachment, got: '%s'", contentDisposition)
	}
}

func TestLoadData(t *testing.T) {
	ctx := &templateData{
		Scheme:   "https",
		Hostname: "hostname.test",
	}
	// install.sh
	installScriptData, err := loadData("./static/install.sh", ctx)
	if err != nil {
		t.Fatal(err)
	}
	installScript := string(installScriptData)

	if !strings.Contains(installScript, "https://hostname.test/bootstrap?dist=$dist") {
		t.Errorf("Expected 'https://hostname.test/bootstrap?dist=$dist' in install.sh")
	}
	if !strings.Contains(installScript, "--hostname=hostname.test") {
		t.Errorf("Expected '--hostname=hostname.test' in install.sh")
	}

	// agent.yaml
	agentYAMLData, err := loadData("./static/agent.yaml.in", ctx)
	if err != nil {
		t.Fatal(err)
	}
	agentYAML := string(agentYAMLData)

	if !strings.Contains(agentYAML, "-agent.poll-url=https://hostname.test/k8s/agent.yaml") {
		t.Errorf("Expected '-agent.poll-url=https://hostname.test/k8s/agent.yaml' in agent.yaml")
	}
}
