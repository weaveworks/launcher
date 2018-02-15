#!/bin/bash -e

tests_root=$(dirname "$0")/..

. ${tests_root}/common.sh

echo "####################################"
echo "### Healthcheck of $1 install script"
echo "####################################"

echo "• Set WEAVE_CLOUD_TOKEN if it is not already set"
[ -z "$WEAVE_CLOUD_TOKEN" ] && WEAVE_CLOUD_TOKEN="abcd1234"

echo "• Install Weave Cloud on the minikube cluster"
curl -Ls $1 | sh -s -- --token=${WEAVE_CLOUD_TOKEN} --assume-yes

wait_for_wc_agents
