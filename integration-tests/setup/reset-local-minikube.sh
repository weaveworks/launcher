#!/bin/bash

# Cleans up a minikube cluster that has been populated by setup-local-minikube.sh

echo "• Removing legacy kube-system namespace agents"
kubectl delete --namespace=kube-system deployments,pods,services,daemonsets,serviceaccounts,configmaps,secrets \
    --selector='app in (weave-flux, weave-cortex, weave-scope)' 2> /dev/null
kubectl delete --namespace=kube-system deployments,pods,services,daemonsets,serviceaccounts,configmaps,secrets \
    --selector='name in (weave-flux, weave-cortex, weave-scope)' 2> /dev/null
kubectl delete --namespace=kube-system secret flux-git-deploy 2> /dev/null

echo "• Removing weave namespace"
kubectl delete ns weave 2> /dev/null

echo "• Removing cluster-wide role objects"
kubectl delete clusterrole,clusterrolebinding -l 'name in (weave-agent, weave-flux, weave-cortex, weave-scope)' 2> /dev/null

echo "• Removing service deployment"
kubectl delete deployment service 2> /dev/null
kubectl delete svc service 2> /dev/null

echo "• Removing bootstrap mock S3 server"
kubectl delete deployment nginx-bootstrap 2> /dev/null
kubectl delete svc nginx-bootstrap 2> /dev/null

echo "• Wait for terminating pods"
JSONPATH='{range .items[*]}{@.metadata.name}{end}'
while [ $(kubectl get pods -l name=service -o jsonpath="$JSONPATH" 2>&1 | wc -c | tr -d '[:space:]') != 0 ]; do echo -n .; sleep 1; done
while [ $(kubectl get pods -n weave -o jsonpath="$JSONPATH" 2>&1 | wc -c | tr -d '[:space:]') != 0 ]; do echo -n .; sleep 1; done

exit 0
