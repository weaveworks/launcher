#!/bin/sh -e

root=$(dirname "$0")

run () {
    run_install_test
    run_self_update_test
}

run_install_test () {
    echo "#############################################"
    echo "### Test installation and creation of WC pods"
    echo "#############################################"

    echo "• Set WEAVE_CLOUD_TOKEN if it is not already set"
    [ -z "$WEAVE_CLOUD_TOKEN" ] && WEAVE_CLOUD_TOKEN="abcd1234"

    echo "• Start launcher/service on minikube"
    service_yaml=$root/k8s/service.yaml
    templatinator "config.sh" $service_yaml
    kubectl apply -f $service_yaml

    wait_for_service

    echo "• Install Weave Cloud on the minikube cluster"
    curl -Ls $(minikube service service --url) | sh -s -- --token=${WEAVE_CLOUD_TOKEN} --assume-yes

    wait_for_wc_agents
}

run_self_update_test () {
    echo "###########################"
    echo "### Test agent self update"
    echo "###########################"

    service_pod=$(kubectl get pods -l name=service -o jsonpath='{range .items[*]}{@.metadata.name}')
    updated_service_yaml=$root/k8s/service.updated.yaml
    updated_agent_yaml=$root/k8s/agent.updated.yaml
    templatinator "config.sh" $updated_service_yaml

    echo "• Take the current service agent k8s and add a new label to it"
    kubectl cp default/${service_pod}:static/agent.yaml $updated_agent_yaml
    yq w -i $updated_agent_yaml items.4.spec.template.metadata.labels.newLabel foo

    echo "• Create a configmap for the updated yaml which will be mounted as a volume"
    if kubectl get configmap agent-k8s; then
        kubectl delete configmap agent-k8s
    fi
    kubectl create configmap agent-k8s --from-file=$updated_agent_yaml

    echo "• Apply the updated service which will use the configmap"
    kubectl apply -f $updated_service_yaml

    wait_for_service

    echo "• Restart the weave-agent pod to force an update"
    kubectl delete pod -n weave -l name=weave-agent

    echo "• Wait for 60 seconds to allow the agent to self update..."
    sleep 60

    wait_for_wc_agents

    echo "• Confirm the agent has self updated correctly"
    numAgents=$(kubectl get pods --no-headers -n weave -l name=weave-agent | wc -l | tr -d '[:space:]')
    newLabelValue=$(kubectl get pods -n weave -l name=weave-agent -o jsonpath="{.items[0].metadata.labels.newLabel}")

    if [ "$numAgents" != "1" ]; then
        echo "Failed to self update. More than 1 agent exists."
        exit 1
    fi

    if [ "$newLabelValue" != "foo" ]; then
        echo "Failed to self update. Label newLabel=foo does not exist."
        exit 1
    fi
}

wait_for_service () {
    echo -n "• Wait for launcher/service pod to become ready"
    JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'
    until kubectl get pods -l name=service -o jsonpath="$JSONPATH" 2>&1 | grep -q "Ready=True"; do echo -n .; sleep 1; done
    echo
    echo -n "• Wait for launcher/service to be fully reachable"
    until curl -Ls $(minikube service service --url) > /dev/null 2>/dev/null ; do echo -n .; sleep 1; done
    echo
}

wait_for_wc_agents () {
    echo -n "• Wait for weave pods to become ready"
    for name in weave-agent kube-state-metrics prom-node-exporter prometheus weave-flux-agent weave-flux-memcached weave-scope-agent
    do
        echo -n "    • Wait for weave/$name"
        JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'
        until kubectl get pods -n weave -l name=$name -o jsonpath="$JSONPATH" 2>&1 | grep -q "Ready=True"; do echo -n .; sleep 1; done
        echo
    done
}

templatinator () {
    config="$1"
    file="$2"
    echo $($root/$config) | go run $root/../cmd/templatinator/templatinator.go $file.in > $file
}

run
