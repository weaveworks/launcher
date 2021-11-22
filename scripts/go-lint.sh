#!/bin/bash

set -e

if [ ! $(command -v golangci-lint) ]
then
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.42.1
fi


golangci-lint run --tests --disable-all --deadline=600s \
    --enable=misspell \
    --enable=vet \
    --enable=ineffassign \
    --enable=gofmt \
    --enable=gocyclo \
    --enable=revive \
    ./...
