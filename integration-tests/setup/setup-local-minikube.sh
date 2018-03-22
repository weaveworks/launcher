#!/bin/bash -e

root=$(dirname "$0")/..
IMAGE_TAG=$($root/../docker/image-tag)
GIT_HASH=$(git rev-parse HEAD)

###
# This is better done once instead of waiting for a VM to boot everytime.
# echo "• Start minikube with launcher-tests profile"
# minikube profile launcher-tests
# minikube start

eval $(minikube docker-env)

###
echo "• Building service image on minikube"
(cd $root/.. && rm -f ./build/.service.done && make service)

###
echo "• Building agent image on minikube"
(cd $root/.. && rm -f ./build/.agent.done && make agent)

###
echo "• Building nginx image serving bootstrap"
dockerfile=$root/../build/Dockerfile.nginx-bootstrap
cp $root/docker/Dockerfile.nginx-bootstrap ${dockerfile}
docker build -t quay.io/weaveworks/launcher-nginx-bootstrap:${IMAGE_TAG} --build-arg version=${GIT_HASH} -f ${dockerfile} $root/../build/

###
echo "• Starting nginx image serving bootstrap"
bootstrap_yaml=$root/k8s/nginx-bootstrap.yaml
echo $($root/config.sh) | go run $root/../cmd/templatinator/templatinator.go $bootstrap_yaml.in > $bootstrap_yaml
kubectl apply -f $bootstrap_yaml

###
echo "• Waiting for nginx-bootstrap service to be available"
until curl -Ls $(minikube service nginx-bootstrap --url) >/dev/null 2>/dev/null; do echo -n .; sleep 1; done
echo
