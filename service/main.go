package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/weaveworks/common/server"
	"github.com/weaveworks/launcher/pkg/text"
)

const (
	defaultAgentManifest = "./static/agent.yaml"
	installScriptFile    = "./static/install.sh"
	s3Bucket             = "https://weaveworks-launcher.s3.amazonaws.com"
)

type templateData struct {
	Scheme           string
	LauncherHostname string
	WCHostname       string
	CRIEndpoint      string
	ReadOnly         bool
}

var (
	bootstrapVersion = flag.String("bootstrap-version", "", "Bootstrap version used for S3 binaries (commit hash)")
	launcherHostname = flag.String("hostname", "get.weave.works", "Hostname for external launcher service")
	wcHostname       = flag.String("wcHostname", "cloud.weave.works", "Hostname for WC agents and users API")
	scheme           = flag.String("scheme", "https", "URL scheme for external launcher service")
	bootstrapBaseURL = flag.String("bootstrap.base-url", s3Bucket, "Base URL the bootstrap binary should be fetched from")
	agentManifest    = flag.String("agent-manifest", defaultAgentManifest, "File used to load agent k8s")
	serverCfg        = server.Config{
		MetricsNamespace:        "service",
		RegisterInstrumentation: true,
	}
)

func main() {
	serverCfg.RegisterFlags(flag.CommandLine)
	flag.Parse()

	if *bootstrapVersion == "" {
		log.Fatal("a bootstrap version is required")
	}

	// Load install.sh and agent.yaml into memory
	data := &templateData{
		Scheme:           *scheme,
		LauncherHostname: *launcherHostname,
		WCHostname:       *wcHostname,
		CRIEndpoint:      "",
	}
	installScriptData, err := loadData(installScriptFile, data)
	if err != nil {
		log.Fatal("error reading installScriptFile:", err)
	}

	handlers := &Handlers{
		bootstrapVersion:  *bootstrapVersion,
		installScriptData: installScriptData,

		templateData: data,
	}

	server, err := server.New(serverCfg)
	if err != nil {
		log.Fatal("error initialising server:", err)
	}
	defer server.Shutdown()

	server.HTTP.HandleFunc("/", handlers.install).Methods("GET").Name("install")
	server.HTTP.HandleFunc("/bootstrap", handlers.bootstrap).Methods("GET").Name("bootstrap")
	server.HTTP.HandleFunc("/k8s/agent.yaml", handlers.agentYAML).Methods("GET").Name("agentYAML")
	server.Run()
}

func loadData(filename string, ctx *templateData) ([]byte, error) {
	tmplData, err := ioutil.ReadFile(filename)
	if err != nil {
		return []byte{}, err
	}
	data, err := text.ResolveString(string(tmplData), ctx)
	if err != nil {
		return []byte{}, err
	}
	return []byte(data), nil
}

// Handlers contains the configuration for serving launcher related binaries
type Handlers struct {
	bootstrapVersion  string
	installScriptData []byte

	templateData *templateData
}

func (h *Handlers) install(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", "attachment; filename=\"install-weave-cloud.sh\"")
	http.ServeContent(w, r, "install.sh", time.Time{}, bytes.NewReader(h.installScriptData))
}

func (h *Handlers) bootstrap(w http.ResponseWriter, r *http.Request) {
	dist := r.URL.Query().Get("dist")

	var filename string

	switch dist {
	case "darwin":
		filename = "bootstrap_darwin_amd64"
	case "linux":
		filename = "bootstrap_linux_amd64"
	default:
		http.Error(w, "Invalid dist query parameter", http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("%s/bootstrap/%s/%s", *bootstrapBaseURL, h.bootstrapVersion, filename), 301)
}

func boolFromParam(r *http.Request, param string) bool {
	v := r.URL.Query().Get(param)
	if v == "true" {
		return true
	}
	return false
}

func (h *Handlers) agentYAML(w http.ResponseWriter, r *http.Request) {
	h.templateData.CRIEndpoint = r.URL.Query().Get("cri-endpoint")
	h.templateData.ReadOnly = boolFromParam(r, "read-only")
	agentManifestData, err := loadData(*agentManifest, h.templateData)
	if err != nil {
		log.Fatal("error reading agentYAMLFile:", err)
	}

	w.Header().Set("Content-Disposition", "attachment")
	http.ServeContent(w, r, "agent.yaml", time.Time{}, bytes.NewReader(agentManifestData))
}
