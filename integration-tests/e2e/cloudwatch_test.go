package e2e

import (
	"testing"
	"time"
)

const (
	ns                       = "weave"
	timeout                  = 5 * time.Minute
	cloudwatchDeploymentName = "cloudwatch-exporter"
	cloudwatchCMName         = "weave-cloudwatch-exporter-config"
)

// TestClouwatchResourceCreation tests that by creating a user cloudwatch
// ConfigMap and Secret cloudwatch specific resources are deployed by the agent.
func TestClouwatchResourceCreating(t *testing.T) {
	test := kube.NewTest(t).Setup()
	defer test.Close()

	// Create Secret
	secret := test.CreateSecretFromFile(ns, "cloudwatch-secret.yaml")

	// Create ConfigMap
	cm := test.CreateConfigMapFromFile(ns, "cloudwatch-cm.yaml")

	// Wait for the above resources to be created.
	test.WaitForSecretReady(secret, timeout)
	test.WaitForConfigMapReady(cm, timeout)

	// Make sure that the Cloudwatch deployment with name 'cloudwatch-exporter' in 'weave' ns exists.
	_, err := test.GetDeployment(ns, cloudwatchDeploymentName)
	if err != nil {
		t.Fatal(err)
	}
	// Make sure that the cloudwatch cm with name 'weave-cloudwatch-exporter-config' in 'weave' ns exists
	_, err = test.GetConfigMap(ns, cloudwatchCMName)
	if err != nil {
		t.Fatal(err)
	}
}
