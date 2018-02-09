#!/bin/bash -e

root=$(dirname "$0")
IMAGE_TAG=$($root/../docker/image-tag)
DEFAULT_SERVICE_IMAGE="quay.io/weaveworks/launcher-service:${IMAGE_TAG}"
DEFAULT_NGINX_BOOTSTRAP_IMAGE="quay.io/weaveworks/launcher-nginx-bootstrap:${IMAGE_TAG}"
DEFAULT_BOOTSTRAP_BASE_URL="https://weaveworks-launcher.s3.amazonaws.com"

# When run locally, we source bootstrap from a local nginx service
[ -z "$CI" ] && BOOTSTRAP_BASE_URL="http://$(minikube ip):30081"

cat <<EOF
{
  "Agent": {
    "Token": "abc123"
  },
  "Service": {
    "Scheme": "http",
    "Hostname": "$(minikube ip):30080",
    "Image": "${SERVICE_IMAGE-$DEFAULT_SERVICE_IMAGE}"
  },
  "Bootstrap" : {
    "Image": "${DEFAULT_NGINX_BOOTSTRAP_IMAGE}",
    "BaseURL": "${BOOTSTRAP_BASE_URL-$DEFAULT_BOOTSTRAP_BASE_URL}"
  }
}
EOF
