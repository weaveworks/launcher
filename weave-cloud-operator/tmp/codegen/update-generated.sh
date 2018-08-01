#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

vendor/k8s.io/code-generator/generate-groups.sh \
deepcopy \
github.com/weaveworks/launcher/weave-cloud-operator/pkg/generated \
github.com/weaveworks/launcher/weave-cloud-operator/pkg/apis \
agent:v1beta1 \
--go-header-file "./tmp/codegen/boilerplate.go.txt"
