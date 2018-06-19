#!/bin/bash -e

tests_root=$(dirname "$0")/..

. ${tests_root}/common.sh

echo "• Install agents in kube-system"
k8s_kube_system_yaml=${tests_root}/k8s/k8s-kube-system.yaml
templatinator "config.sh" $k8s_kube_system_yaml
kubectl apply -f $k8s_kube_system_yaml -n kube-system

echo "• Set WEAVE_CLOUD_TOKEN if it is not already set"
[ -z "$WEAVE_CLOUD_TOKEN" ] && WEAVE_CLOUD_TOKEN="abcd1234"

echo "• Start launcher/service on minikube"
service_yaml=${tests_root}/k8s/service.yaml
templatinator "config.sh" $service_yaml
kubectl apply -f $service_yaml

wait_for_service

echo "• Install Weave Cloud on the minikube cluster"
curl -Ls $(minikube service service --url) | sh -s -- --token=${WEAVE_CLOUD_TOKEN} --assume-yes

wait_for_wc_agents

