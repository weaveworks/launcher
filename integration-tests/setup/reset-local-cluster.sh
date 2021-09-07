#!/bin/bash

# Deletes a local cluster that has been populated by setup-local-cluster.sh
kind delete cluster --name launcher-tests

exit 0
