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
	"github.com/weaveworks/launcher/text"
)

const s3Bucket = "https://weaveworks-launcher.s3.amazonaws.com"

func main() {
	var (
		bootstrapVersion = flag.String("bootstrap-version", "", "Bootstrap version used for S3 binaries (short commit hash)")
		hostname         = flag.String("hostname", "get.weave.works", "Hostname for external launcher service")
		serverCfg        = server.Config{
			MetricsNamespace:        "launcher",
			RegisterInstrumentation: true,
		}
	)
	serverCfg.RegisterFlags(flag.CommandLine)
	flag.Parse()

	if *bootstrapVersion == "" {
		log.Fatal("a bootstrap version is required")
	}

	// Load install.sh and agent.yaml into memory
	installScriptData, err := loadInstallScript(*hostname)
	if err != nil {
		log.Fatal("error reading static/install.sh file:", err)
	}
	agentYAMLData, err := ioutil.ReadFile("./static/agent.yaml")
	if err != nil {
		log.Fatal("error reading static/agent.yaml file:", err)
	}

	handlers := &Handlers{
		bootstrapVersion:  *bootstrapVersion,
		installScriptData: installScriptData,
		agentYAMLData:     agentYAMLData,
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

func loadInstallScript(hostname string) ([]byte, error) {
	tmplData, err := ioutil.ReadFile("./static/install.sh")
	if err != nil {
		return []byte{}, err
	}
	data, err := text.ResolveString(string(tmplData), struct{ Hostname string }{hostname})
	if err != nil {
		return []byte{}, err
	}
	return []byte(data), nil
}

// Handlers contains the configuration for serving launcher related binaries
type Handlers struct {
	bootstrapVersion  string
	installScriptData []byte
	agentYAMLData     []byte
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

	http.Redirect(w, r, fmt.Sprintf("%s/bootstrap/%s/%s", s3Bucket, h.bootstrapVersion, filename), 301)
}

func (h *Handlers) agentYAML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", "attachment")
	http.ServeContent(w, r, "agent.yaml", time.Time{}, bytes.NewReader(h.agentYAMLData))
}
