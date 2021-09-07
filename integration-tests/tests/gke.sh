#!/bin/bash -e

tests_root=$(dirname "$0")/..

. ${tests_root}/common.sh

echo "####################"
echo "### Test GKE option"
echo "####################"

echo "• Set WEAVE_CLOUD_TOKEN if it is not already set"
[ -z "$WEAVE_CLOUD_TOKEN" ] && WEAVE_CLOUD_TOKEN="abcd1234"

echo "• Start launcher/service on test cluster"
service_yaml=${tests_root}/k8s/service.yaml
templatinator "config.sh" $service_yaml
kubectl apply -f $service_yaml

wait_for_service

echo "• Install Weave Cloud on the test cluster with --gke"
curl -Ls $(get_ip):30080 | sh -s -- --token=${WEAVE_CLOUD_TOKEN} --gke --assume-yes --report-errors

wait_for_wc_agents

echo "• Check clusterrolebinding was created"
roleuser=$(kubectl get clusterrolebinding cluster-admin-$USER -o jsonpath="{.subjects[0].name}")
if [[ $roleuser != "circleci-integration-test-gke@launcher-integration-tests.iam.gserviceaccount.com" ]]; then
    echo "clusterrolebinding user is not as expected. Got: $roleuser"
    exit 1
fi
clusterrole=$(kubectl get clusterrolebinding cluster-admin-$USER -o jsonpath="{.roleRef.name}")
if [[ $clusterrole != "cluster-admin" ]]; then
    echo "clusterrolebinding role is not as expected. Got: $clusterrole"
    exit 1
fi
