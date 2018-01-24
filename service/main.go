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
)

const s3Bucket = "https://weaveworks-launcher.s3.amazonaws.com"

func main() {
	var (
		bootstrapVersion = flag.String("bootstrap-version", "", "Bootstrap version used for S3 binaries (short commit hash)")
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

	// Load install.sh into memory
	installScriptData, err := ioutil.ReadFile("./install.sh")
	if err != nil {
		log.Fatal("error reading install.sh file:", err)
	}

	handlers := &Handlers{
		bootstrapVersion:  *bootstrapVersion,
		installScriptData: installScriptData,
	}

	server, err := server.New(serverCfg)
	if err != nil {
		log.Fatal("error initialising server:", err)
	}
	defer server.Shutdown()

	server.HTTP.HandleFunc("/", handlers.install).Methods("GET").Name("install")
	server.HTTP.HandleFunc("/bootstrap", handlers.bootstrap).Methods("GET").Name("bootstrap")
	server.Run()
}

// Handlers contains the configuration for serving launcher related binaries
type Handlers struct {
	bootstrapVersion  string
	installScriptData []byte
}

func (h *Handlers) install(w http.ResponseWriter, r *http.Request) {
	http.ServeContent(w, r, "install.sh", time.Time{}, bytes.NewReader(h.installScriptData))
}

func (h *Handlers) bootstrap(w http.ResponseWriter, r *http.Request) {
	dist := r.URL.Query().Get("dist")

	var filename string

	switch dist {
	case "darwin":
		filename = "bootstrap-darwin-amd64"
	case "linux":
		filename = "bootstrap-linux-amd64"
	default:
		http.Error(w, "Invalid dist query parameter", http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("%s/bootstrap/%s/%s", s3Bucket, h.bootstrapVersion, filename), 301)
}
