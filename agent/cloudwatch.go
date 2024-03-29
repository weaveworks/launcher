package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/cache"

	"github.com/weaveworks/launcher/pkg/kubectl"
	"github.com/weaveworks/launcher/pkg/text"
)

type cloudwatch struct {
	Region     string
	SecretName string
	Resources  []string
}

// Watch for CM creation.
func watchConfigMaps(cfg *agentConfig) {
	source := cache.NewListWatchFromClient(
		cfg.KubeClient.CoreV1().RESTClient(),
		"configmaps",
		"weave",
		fields.SelectorFromSet(fields.Set{"metadata.name": "cloudwatch"}))

	cfg.CMInformer = cache.NewSharedIndexInformer(
		source,
		&apiv1.ConfigMap{},
		0,
		cache.Indexers{},
	)

	cfg.CMInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    cfg.handleCMAdd,
		UpdateFunc: cfg.handleCMUpdate,
		DeleteFunc: cfg.handleCMDelete,
	})
}

// Triggered on all ConfigMap creation with name cloudwatch in weave ns
func (cfg *agentConfig) handleCMAdd(obj interface{}) {
	cm, ok := obj.(*apiv1.ConfigMap)
	if !ok {
		log.Error("Failed to type assert ConfigMap: ", obj)
		return
	}

	log.Debugf("ConfigMap %v/%v was created", cm.ObjectMeta.Name, cm.ObjectMeta.Name)

	cfg.checkOrInstallCloudWatch(cm)
}

// Triggered on all ConfigMap update with name cloudwatch in weave ns
func (cfg *agentConfig) handleCMUpdate(old, cur interface{}) {
	cm, ok := cur.(*apiv1.ConfigMap)
	if !ok {
		log.Error("Failed to type assert ConfigMap: ", cur)
		return
	}

	log.Debugf("ConfigMap %v/%v was updated", cm.ObjectMeta.Namespace, cm.ObjectMeta.Name)

	cfg.checkOrInstallCloudWatch(cm)
}

// Triggered on only ConfigMap deletion with name cloudwatch in weave ns
func (cfg *agentConfig) handleCMDelete(obj interface{}) {
	// TODO: Handle ConfigMap deletion. Delete individual objects that were created as at this point.
}

// Watch for Secret creation/update/deletion.
func watchSecrets(cfg *agentConfig) {
	source := cache.NewListWatchFromClient(
		cfg.KubeClient.CoreV1().RESTClient(),
		"secrets",
		"weave",
		fields.Everything())

	cfg.SecretInformer = cache.NewSharedIndexInformer(
		source,
		&apiv1.Secret{},
		0,
		cache.Indexers{},
	)

	cfg.SecretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    cfg.handleSecretAdd,
		UpdateFunc: cfg.handleSecretUpdate,
		DeleteFunc: cfg.handleSecretDelete,
	})
}

// Triggered on all Secret created in the weave ns
func (cfg *agentConfig) handleSecretAdd(obj interface{}) {
	secret, ok := obj.(*apiv1.Secret)
	if !ok {
		// If the object is not a secret we should ignore.
		log.Error("Failed to type assert Secret: ", obj)
		return
	}

	log.Debugf("Secret %v/%v was created", secret.ObjectMeta.Namespace, secret.ObjectMeta.Name)

	cfg.conformSecret()
}

// Triggered on all Secret updates in the weave ns
func (cfg *agentConfig) handleSecretUpdate(old, cur interface{}) {
	secret, ok := cur.(*apiv1.Secret)
	if !ok {
		// If the object is not a secret we should ignore.
		log.Error("Failed to type assert Secret: ", cur)
		return
	}

	log.Debugf("Secret %v/%v was updated", secret.ObjectMeta.Namespace, secret.ObjectMeta.Name)

	cfg.conformSecret()
}

// Triggered on all Secret deletions in the weave ns
func (cfg *agentConfig) handleSecretDelete(obj interface{}) {
	// TODO: Handle Secret deletion. Delete individual objects that were created.
}

func (cfg *agentConfig) conformSecret() {
	// Get ConfigMap
	cm, err := cfg.getConfigMap("cloudwatch")
	if err != nil {
		// If we don't have the cloudwatch CM this is the wrong secret.
		log.Debug(err)
		return
	}

	cfg.checkOrInstallCloudWatch(cm)
}

// checkOrInstallCloudWatch by applying the manifest file.
func (cfg *agentConfig) checkOrInstallCloudWatch(cm *apiv1.ConfigMap) {
	for name, content := range cm.Data {
		if name != "cloudwatch.yaml" {
			return
		}

		cw, err := parseCloudwatchYaml(content)
		if err != nil {
			log.Error(err)
			return
		}

		// Get the Secret UID and with that we make sure the Secret actually exists.
		secret, err := cfg.getSecret(cw.SecretName)
		if err != nil {
			log.Error(err)
			return
		}

		err = cfg.deployCloudwatch(cw, cm.Name, string(cm.UID), string(secret.UID))
		if err != nil {
			log.Error("Error while deploying cloudwatch manifest: ", err)
			return
		}
	}
}

func (cfg *agentConfig) getSecret(name string) (*apiv1.Secret, error) {
	s, err := cfg.KubeClient.CoreV1().Secrets("weave").Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (cfg *agentConfig) getConfigMap(name string) (*apiv1.ConfigMap, error) {
	cm, err := cfg.KubeClient.CoreV1().ConfigMaps("weave").Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return cm, nil
}

func isValidResource(name string) bool {
	for _, resource := range validResources {
		if name == resource {
			return true
		}
	}
	return false
}

func (cfg *agentConfig) deployCloudwatch(cw *cloudwatch, CMName, CMUID, secretUID string) error {
	k8sVersion, err := getMajorMinorVersion(cfg.KubernetesMajorVersion, cfg.KubernetesMinorVersion, cfg.KubernetesVersion)
	if err != nil {
		log.Fatal("invalid Kubernetes version: ", err)
	}

	cwPollURL, err := text.ResolveString(defaultCloudwatchURL, map[string]string{
		"WCHostname":                  cfg.WCHostname,
		"KubernetesMajorMinorVersion": k8sVersion,
		"Region":                      cw.Region,
		"SecretName":                  cw.SecretName,
		"Resources":                   strings.Join(cw.Resources, "%2C"),
		"ConfigName":                  CMName,
		"ConfigUID":                   CMUID,
		"SecretUID":                   secretUID,
	})
	if err != nil {
		log.Fatal("invalid URL template: ", err)
	}

	log.Info("Applying cloudwatch manifest from: ", cwPollURL)

	err = kubectl.Apply(cfg.KubectlClient, cwPollURL)
	if err != nil {
		return err
	}

	return nil
}

func validateResources(resources []string) error {
	if len(resources) == 0 {
		return errors.New("cloudwatch: at least one AWS resource must be specified")
	}
	for _, resource := range resources {
		if !isValidResource(resource) {
			return fmt.Errorf("cloudwatch: unknown resource '%s'", resource)
		}
	}
	return nil
}

func parseCloudwatchYaml(cm string) (*cloudwatch, error) {
	cw := cloudwatch{}
	if err := yaml.NewYAMLOrJSONDecoder(bytes.NewBufferString(cm), 1000).Decode(&cw); err != nil {
		return nil, err
	}
	if err := validateResources(cw.Resources); err != nil {
		return nil, err
	}
	return &cw, nil
}
