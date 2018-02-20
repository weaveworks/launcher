package main

import (
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/launcher/pkg/kubectl"
)

func migrateKubeSystem(kubectlClient kubectl.Client) *FluxConfig {
	// Save and return any existing flux config in kube-system
	fluxCfg, err := getFluxConfig(kubectlClient, "kube-system")
	if err != nil {
		log.Info("Failed to get existing flux config")
	}

	// Users that have installed Weave Cloud manifests before January the 11th
	// 2018, will have the agents installed in the kube-system namespace. If that's
	// the case let's handle the migration to the weave namespace:
	// 1. Delete our objects in the kube-system namespace
	// 2. Let launcher-agent install the new ones in the weave namespace
	log.Info("Checking for any previous installation...")
	deleted := deleteKubeSystemObjects(kubectlClient)
	if deleted {
		log.Info("Removed old agents from the kube-system namespace. You will have to reconfigure Deploy.")
	}

	return fluxCfg
}

// Delete old Weave Cloud objects and return if we have indeed deleted anything.
func deleteKubeSystemObjects(kubectlClient kubectl.Client) bool {
	deleted := false

	out, _ := kubectlClient.Execute("delete", "--namespace=kube-system",
		"deployments,pods,services,daemonsets,serviceaccounts,configmaps,secrets",
		"--selector=app in (weave-flux, weave-cortex, weave-scope)")
	// Used with a selector, kubectl 1.7.5 returns a 0 exit code with the message
	// "No resources found" when there's no matching resources.
	deleted = deleted || !strings.Contains(out, "No resources found")

	out, _ = kubectlClient.Execute("delete", "--namespace=kube-system",
		"deployments,pods,services,daemonsets,serviceaccounts,configmaps,secrets",
		"--selector=name in (weave-flux, weave-cortex, weave-scope)")
	deleted = deleted || !strings.Contains(out, "No resources found")

	_, err := kubectlClient.Execute("--namespace=kube-system", "delete", "secret", "flux-git-deploy")
	deleted = deleted || (err == nil)

	return deleted
}
