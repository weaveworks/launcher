package k8s

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	deploymentutil "k8s.io/kubernetes/pkg/controller/deployment/util"
)

// GetDeploymentReplicaSetRevision gets the revision of the latest replica set of a deployment
func GetDeploymentReplicaSetRevision(kubeClient *kubeclient.Clientset, namespace, name string) (int64, error) {
	versionedClient := kubeClient.ExtensionsV1beta1()

	deployment, err := versionedClient.Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve deployment: %s", err)
	}

	_, oldRs, newRs, err := deploymentutil.GetAllReplicaSets(deployment, versionedClient)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve deployment replicasets: %s", err)
	}

	rs := deploymentutil.FindActiveOrLatest(newRs, oldRs)
	revision, err := deploymentutil.Revision(rs)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve deployment replicaset revision: %s", err)
	}

	return revision, nil
}
