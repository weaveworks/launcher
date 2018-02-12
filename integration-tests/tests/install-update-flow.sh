#!/bin/sh -e

root=$(dirname "$0")/..

. $root/common.sh

run () {
    run_install_test
    run_self_update_test
    run_self_update_failure_test
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

    echo "• Take the current service agent k8s and add a new label to it and reduce the recovery wait to 60s"
    kubectl cp default/${service_pod}:static/agent.yaml $updated_agent_yaml
    yq w -i $updated_agent_yaml items.4.spec.template.metadata.labels.newLabel foo
    yq w -i $updated_agent_yaml items.4.spec.template.spec.containers.0.args.3 '"-agent.recovery-wait=60s"'

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

    echo "• Wait for the agent to start the self update"
    while [ $(kubectl get pods --no-headers -n weave -l name=weave-agent | wc -l | tr -d '[:space:]') = 1 ] ; do echo -n .; sleep 1; done

    echo "• Wait for the agent to finish the self update"
    until [ $(kubectl get pods --no-headers -n weave -l name=weave-agent | wc -l | tr -d '[:space:]') = 1 ] ; do echo -n .; sleep 1; done

    wait_for_wc_agents

    echo "• Check the agent updated correctly"
    if [ $(kubectl get pods -n weave -l name=weave-agent -o jsonpath="{.items[0].metadata.labels.newLabel}") != "foo" ]; then
        echo "Failed to self update. Label newLabel=foo does not exist."
        exit 1
    fi
}

run_self_update_failure_test () {
    echo "##################################"
    echo "### Test agent self update failure"
    echo "##################################"

    service_pod=$(kubectl get pods -l name=service -o jsonpath='{range .items[*]}{@.metadata.name}')
    updated_service_yaml=$root/k8s/service.updated.yaml
    updated_agent_yaml=$root/k8s/agent.updated.yaml
    templatinator "config.sh" $updated_service_yaml

    echo "• Take the current service agent k8s and set the image to one that does not exist"
    yq w -i $updated_agent_yaml items.4.spec.template.spec.containers.0.image example.org/does_not_exist

    echo "• Create a configmap for the updated yaml which will be mounted as a volume"
    if kubectl get configmap agent-k8s; then
        kubectl delete configmap agent-k8s
    fi
    kubectl create configmap agent-k8s --from-file=$updated_agent_yaml

    echo "• Restart the service"
    kubectl delete pod -l name=service
    wait_for_service

    echo "• Restart the weave-agent pod to force an update"
    kubectl delete pod -n weave -l name=weave-agent

    echo "• Wait for the new agent to fail to pull the image"
    JSONPATH='{range .items[*]}{@.metadata.name}:{@.status.containerStatuses[*].state.waiting.reason}{end}'
    until kubectl get pods -n weave -o jsonpath="$JSONPATH" 2>&1 | grep -q "ImagePullBackOff"; do echo -n .; sleep 1; done

    echo "• Wait for the agent to begin recovery"
    JSONPATH='{range .items[*]}{@.metadata.name}:{@.status.containerStatuses[*].state.waiting.reason}{end}'
    while kubectl get pods -n weave -o jsonpath="$JSONPATH" 2>&1 | grep -q "ImagePullBackOff"; do echo -n .; sleep 1; done

    echo "• Wait for the agent to finish recovery"
    until [ $(kubectl get pods --no-headers -n weave -l name=weave-agent | wc -l | tr -d '[:space:]') = 1 ] ; do echo -n .; sleep 1; done

    wait_for_wc_agents
}

run
