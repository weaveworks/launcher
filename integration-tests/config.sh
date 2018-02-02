#!/bin/bash -e

root=$(dirname "$0")
IMAGE_TAG=$($root/../docker/image-tag)
DEFAULT_SERVICE_IMAGE="quay.io/weaveworks/launcher-service:${IMAGE_TAG}"
DEFAULT_NGINX_BOOTSTRAP_IMAGE="quay.io/weaveworks/launcher-nginx-bootstrap:${IMAGE_TAG}"

cat <<EOF
{
  "Service": {
    "Scheme": "http",
    "Hostname": "$(minikube ip):30080",
    "Image": "${SERVICE_IMAGE-$DEFAULT_SERVICE_IMAGE}"
  },
  "Bootstrap" : {
    "Image": "${DEFAULT_NGINX_BOOTSTRAP_IMAGE}",
    "BaseURL": "http://$(minikube ip):30081"
  }
}
EOF
