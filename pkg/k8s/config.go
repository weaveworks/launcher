// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8s

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// ClientConfig holds configuration options for API server clients.
type ClientConfig struct {
	CertificateAuthority string
	ClientCertificate    string
	ClientKey            string
	Cluster              string
	Context              string
	Insecure             bool
	Kubeconfig           string
	Password             string
	Server               string
	Token                string
	User                 string
	Username             string
}

func homeDirectory() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

// kubeconfigPath returns the default kubeconfig location.
func kubeconfigPath() string {
	home := homeDirectory()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}

// NewClientConfig returns a configuration object that can be used to configure a client in
// order to contact an API server with.
func NewClientConfig(config *ClientConfig) (*rest.Config, error) {
	var restConfig *rest.Config
	var err error

	if config.Server == "" && config.Kubeconfig == "" {
		// If no API server address or kubeconfig was provided, assume we are
		// running inside a pod and Try to connect to the API server through
		// its Service environment variables, using the default Service Account
		// Token.
		restConfig, err = rest.InClusterConfig()
	}

	if restConfig == nil {
		// We're not in a pod? try to use kubeconfig.

		// Try the default kubeconfig location if nothing else is provided.
		if config.Kubeconfig == "" {
			config.Kubeconfig = kubeconfigPath()
		}

		restConfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: config.Kubeconfig},
			&clientcmd.ConfigOverrides{
				AuthInfo: clientcmdapi.AuthInfo{
					ClientCertificate: config.ClientCertificate,
					ClientKey:         config.ClientKey,
					Token:             config.Token,
					Username:          config.Username,
					Password:          config.Password,
				},
				ClusterInfo: clientcmdapi.Cluster{
					Server:                config.Server,
					InsecureSkipTLSVerify: config.Insecure,
					CertificateAuthority:  config.CertificateAuthority,
				},
				Context: clientcmdapi.Context{
					Cluster:  config.Cluster,
					AuthInfo: config.User,
				},
				CurrentContext: config.Context,
			},
		).ClientConfig()
	}

	if err != nil {
		return nil, err
	}

	log.Infof("kubernetes: targeting api server %s", restConfig.Host)

	return restConfig, nil
}
