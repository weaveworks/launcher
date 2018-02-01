#!/bin/bash -e

root=$(dirname "$0")
DEFAULT_SERVICE_IMAGE="quay.io/weaveworks/launcher-service:$($root/../docker/image-tag)"

cat <<EOF
{
  "Service": {
    "Scheme": "http",
    "Hostname": "$(minikube ip):30080",
    "Image": "${SERVICE_IMAGE-$DEFAULT_SERVICE_IMAGE}"
  }
}
EOF
