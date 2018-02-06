#!/bin/sh -e

root=$(dirname "$0")

echo "####################################"
echo "### Healthcheck of $1 install script"
echo "####################################"

echo "• Set WEAVE_CLOUD_TOKEN if it is not already set"
[ -z "$WEAVE_CLOUD_TOKEN" ] && WEAVE_CLOUD_TOKEN="abcd1234"

echo "• Install Weave Cloud on the minikube cluster"
curl -Ls $1 | sh -s -- --token=${WEAVE_CLOUD_TOKEN} --assume-yes

echo -n "• Wait for weave pods to become ready"
for name in weave-agent kube-state-metrics prom-node-exporter prometheus weave-flux-agent weave-flux-memcached weave-scope-agent
do
    echo -n "    • Wait for weave/$name"
    JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'
    until kubectl get pods -n weave -l name=$name -o jsonpath="$JSONPATH" 2>&1 | grep -q "Ready=True"; do echo -n .; sleep 1; done
    echo
done
