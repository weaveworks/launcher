#!/bin/bash

set -e

if [ ! "$(command -v golangci-lint)" ]
then
    curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b "$GOPATH"/bin v1.31.0
fi


golangci-lint run --tests --disable-all --deadline=600s \
    --enable=misspell \
    --enable=vet \
    --enable=ineffassign \
    --enable=gofmt \
    --enable=gocyclo \
    --enable=golint \
    ./...
