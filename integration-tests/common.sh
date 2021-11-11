#!/bin/bash -e
get_ip () {
    if [[ $(uname) == "Darwin" ]]; then
        echo "localhost"
    else
        kubectl --context=kind-launcher-tests get nodes -o=jsonpath='{range .items[*]}{.status.addresses[?(@.type=="InternalIP")].address}{end}'
    fi
}

wait_for_service () {
    echo "• Wait for launcher/service pod to become ready"
    JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'
    until kubectl get pods -l name=service -o jsonpath="$JSONPATH" 2>&1 | grep -q "Ready=True"; do sleep 1; done

    echo "• Wait for launcher/service to be fully reachable"
    until curl -Ls -m1 $(get_ip):30080; do sleep 1; done
}

wait_for_wc_agents () {
    echo "• Wait for weave pods to become ready"
    for name in weave-agent kube-state-metrics prom-node-exporter prometheus weave-flux-agent weave-flux-memcached weave-scope-agent
    do
        echo "    • Wait for weave/$name"
        JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}'
        until kubectl get pods -n weave -l name=$name -o jsonpath="$JSONPATH" 2>&1 | grep -q "Ready=True"; do sleep 1; done
    done
}

templatinator () {
    config="$1"
    file="$2"
    echo $(${tests_root}/$config) | go run ${tests_root}/../cmd/templatinator/templatinator.go $file.in > $file
}
