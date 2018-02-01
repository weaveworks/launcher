#!/bin/sh -e

root=$(dirname "$0")

###
echo "• Start launcher/service on minikube"
service_yaml=$root/k8s/service.yaml
echo $($root/config.sh) | go run $root/../cmd/templatinator/templatinator.go $service_yaml.in > $service_yaml
kubectl apply -f $service_yaml

###
echo -n "• Wait for launcher/service pod to become ready"
JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'
until kubectl get pods -l name=service -o jsonpath="$JSONPATH" 2>&1 | grep -q "Ready=True"; do echo -n .; sleep 1; done
echo

###
echo -n "• Wait for launcher/service to be fully reachable"
until curl -Ls $(minikube service service --url) > /dev/null 2>/dev/null ; do echo -n .; sleep 1; done
echo

###
echo "• Install Weave Cloud on the minikube cluster"
curl -Ls $(minikube service service --url) | sh -s -- --token=${WEAVE_CLOUD_TOKEN} --assume-yes
