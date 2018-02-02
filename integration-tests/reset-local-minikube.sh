#!/bin/bash

# Cleans up a minikube cluster that has been populated by setup-local-minikube.sh

echo "• Removing weave namespace"
kubectl delete ns weave 2> /dev/null

echo "• Removing service deployment"
kubectl delete deployment service 2> /dev/null
kubectl delete svc service 2> /dev/null

echo "• Removing bootstrap mock S3 server"
kubectl delete deployment nginx-bootstrap 2> /dev/null
kubectl delete svc nginx-bootstrap 2> /dev/null
