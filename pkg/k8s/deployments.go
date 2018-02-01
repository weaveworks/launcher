package k8s

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	deploymentutil "k8s.io/kubernetes/pkg/controller/deployment/util"
)

// GetLatestDeploymentReplicaSetRevision gets the latest revision of replica sets of a deployment
func GetLatestDeploymentReplicaSetRevision(kubeClient *kubeclient.Clientset, namespace, name string) (int64, error) {
	// Based on https://github.com/kubernetes/kubernetes/blob/release-1.9/pkg/kubectl/history.go
	versionedClient := kubeClient.ExtensionsV1beta1()

	deployment, err := versionedClient.Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve deployment: %s", err)
	}

	_, allOldRSs, newRS, err := deploymentutil.GetAllReplicaSets(deployment, versionedClient)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve deployment replicasets: %s", err)
	}
	allRSs := allOldRSs
	if newRS != nil {
		allRSs = append(allRSs, newRS)
	}

	var maxRevision int64
	for _, rs := range allRSs {
		v, err := deploymentutil.Revision(rs)
		if err != nil {
			continue
		}
		if v > maxRevision {
			maxRevision = v
		}
	}

	return maxRevision, nil
}
