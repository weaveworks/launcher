#!/bin/bash -e

tests_root=$(dirname "$0")
IMAGE_TAG=$(${tests_root}/../docker/image-tag)
DEFAULT_SERVICE_IMAGE="weaveworks/launcher-service:${IMAGE_TAG}"
DEFAULT_NGINX_BOOTSTRAP_IMAGE="weaveworks/launcher-nginx-bootstrap:${IMAGE_TAG}"
DEFAULT_BOOTSTRAP_BASE_URL="https://weaveworks-launcher.s3.amazonaws.com"

# When run locally, we source bootstrap from a local nginx service
[ -z "$CI" ] && BOOTSTRAP_BASE_URL="http://$(minikube ip):30091"

cat <<EOF
{
  "Service": {
    "Scheme": "http",
    "LauncherHostname": "$(minikube ip):30080",
    "WCHostname": "frontend.dev.weave.works",
    "Image": "${SERVICE_IMAGE-$DEFAULT_SERVICE_IMAGE}"
  },
  "Bootstrap" : {
    "Image": "${DEFAULT_NGINX_BOOTSTRAP_IMAGE}",
    "BaseURL": "${BOOTSTRAP_BASE_URL-$DEFAULT_BOOTSTRAP_BASE_URL}"
  },
  "K8sKubeSystem": {
    "Token": "abc123"
  }
}
EOF
