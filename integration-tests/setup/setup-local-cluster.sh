#!/bin/bash -e

root=$(dirname "$0")/..
IMAGE_TAG=$($root/../docker/image-tag)
GIT_HASH=$(git rev-parse HEAD)

source $root/common.sh
###
# This is better done once instead of waiting for a VM to boot everytime.
echo "• Start kind with launcher-tests profile"

TMPFILE=$(mktemp)

cat <<EOM>${TMPFILE}
# cluster-config.yml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30091
    hostPort: 30091
    protocol: TCP
  - containerPort: 30080
    hostPort: 30080
    protocol: TCP
EOM

kind create cluster --name launcher-tests --config="${TMPFILE}"
rm "${TMPFILE}"

###
echo "• Building service image on test cluster"
(cd $root/.. && rm -f ./build/.service.Dockerfile.done && make service)
kind load --name launcher-tests docker-image weaveworks/launcher-service:${IMAGE_TAG}

###
echo "• Building agent image on test cluster"
(cd $root/.. && rm -f ./build/.agent.Dockerfile.done && make agent)
kind load --name launcher-tests docker-image weaveworks/launcher-agent:${IMAGE_TAG}

###
echo "• Building nginx image serving bootstrap"
(cd $root/.. && rm -f ./build/.bootstrap.Dockerfile.done && make bootstrap)
dockerfile=$root/../build/Dockerfile.nginx-bootstrap
cp $root/docker/Dockerfile.nginx-bootstrap ${dockerfile}
docker build -t weaveworks/launcher-nginx-bootstrap:${IMAGE_TAG} \
             --build-arg version=${GIT_HASH} \
             --build-arg base_tag=${IMAGE_TAG} \
             -f ${dockerfile} $root/..
kind load --name launcher-tests docker-image weaveworks/launcher-nginx-bootstrap:${IMAGE_TAG}

###
echo "• Starting nginx image serving bootstrap"
bootstrap_yaml=$root/k8s/nginx-bootstrap.yaml
echo $($root/config.sh) | go run $root/../cmd/templatinator/templatinator.go $bootstrap_yaml.in > $bootstrap_yaml

kubectl apply -f $bootstrap_yaml

###
echo -n "• Waiting for nginx-bootstrap service to be available at $(get_ip):30091"
until curl -Ls -m1 $(get_ip):30091 ; do sleep 1; done
