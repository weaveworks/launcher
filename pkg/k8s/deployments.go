package k8s

import (
	"context"
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
)

// GetLatestDeploymentReplicaSetRevision gets the latest revision of replica sets of a deployment
func GetLatestDeploymentReplicaSetRevision(kubeClient *kubeclient.Clientset, namespace, name string) (int64, error) {
	// Based on https://github.com/kubernetes/kubernetes/blob/release-1.9/pkg/kubectl/history.go
	versionedClient := kubeClient.AppsV1()

	deployment, err := versionedClient.Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve deployment: %s", err)
	}

	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return 0, fmt.Errorf("failed to turn label selector into selector: %s", err)
	}
	allRSs, err := versionedClient.ReplicaSets(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve deployment replicasets: %s", err)
	}

	var maxRevision int64
	for _, rs := range allRSs.Items {
		rev := rs.ObjectMeta.Annotations["deployment.kubernetes.io/revision"]
		v, err := strconv.ParseInt(rev, 10, 8)
		if err != nil {
			continue
		}
		if v > maxRevision {
			maxRevision = v
		}
	}

	return maxRevision, nil
}
