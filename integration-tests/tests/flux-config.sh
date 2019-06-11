#!/bin/bash -e

tests_root=$(dirname "$0")/..

. ${tests_root}/common.sh

echo "####################"
echo "### Test flux config"
echo "####################"

echo "• Set WEAVE_CLOUD_TOKEN if it is not already set"
[ -z "$WEAVE_CLOUD_TOKEN" ] && WEAVE_CLOUD_TOKEN="abcd1234"

echo "• Start launcher/service on minikube"
service_yaml=${tests_root}/k8s/service.yaml
templatinator "config.sh" $service_yaml
kubectl apply -f $service_yaml

wait_for_service

service_pod=$(kubectl get pods -l name=service -o jsonpath='{range .items[*]}{@.metadata.name}')
updated_service_yaml=${tests_root}/k8s/service.updated.yaml
updated_agent_yaml=${tests_root}/k8s/agent.updated.yaml
templatinator "config.sh" $updated_service_yaml

echo "• Take the current service agent k8s and reduce the poll interval to 10 seconds"
kubectl cp default/${service_pod}:static/agent.yaml $updated_agent_yaml
yq w -i $updated_agent_yaml items.4.spec.template.spec.containers.0.args.3 '"-wc.poll-interval=10s"'

echo "• Create a configmap for the updated yaml which will be mounted as a volume"
if kubectl get configmap agent-k8s; then
    kubectl delete configmap agent-k8s
fi
kubectl create configmap agent-k8s --from-file=$updated_agent_yaml

echo "• Apply the updated service which will use the configmap"
kubectl apply -f $updated_service_yaml

wait_for_service

echo "• Install flux with some configuration"
kubectl apply -f "https://frontend.dev.weave.works/k8s/flux.yaml?t=${WEAVE_CLOUD_TOKEN}&k8s-version=$(kubectl version | base64 | tr -d '\n')&flux-version=%5E1&git-label=example&git-url=git%40github.com%3Aweaveworks%2Fexample&git-path=k8s%2Fexample&git-branch=example"

echo "• Wait for weave-flux-agent to become ready"
JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'
until kubectl get pods -n weave -l name=weave-flux-agent -o jsonpath="$JSONPATH" 2>&1 | grep -q "Ready=True"; do sleep 1; done

echo "• Install Weave Cloud"
curl -Ls $(minikube service service --url) | sh -s -- --token=${WEAVE_CLOUD_TOKEN} --assume-yes

wait_for_wc_agents

echo "• Wait a couple of update cycles"
sleep 40

echo "• Check flux configuration still exists"
args=$(kubectl get pod -n weave -l name=weave-flux-agent -o jsonpath='{.items[?(@.metadata.labels.name=="weave-flux-agent")].spec.containers[?(@.name=="flux-agent")].args[*]}')
expected="--git-url=git@github.com:weaveworks/example --git-path=k8s/example --git-branch=example --git-label=example"
if [[ $args != *"$expected"* ]]; then
    echo "Missing existing flux args: \"$expected\" not found in \"$args\""
    exit 1
fi
