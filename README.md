# Weave Cloud Launcher

[![Circle CI](https://circleci.com/gh/weaveworks/launcher/tree/master.svg?style=shield)](https://circleci.com/gh/weaveworks/launcher/tree/master)

<h3 align="center">
  <code>curl -Ls https://get.weave.works | sh -s -- --token=XXXXXX</code>
</h3>

## Overview

- `curl -Ls https://get.weave.works | sh -s -- --token=XXXXXX` (on the host)
  - https://get.weave.works serves [install.sh](service/static/install.sh)
  - Downloads and executes the bootstrap binary
- [Bootstrap](bootstrap) binary (on the host)
  - Confirms the current k8s cluster with the user
  - Applies the Agent to the cluster via the host's `kubectl`
- [Agent](agent) (in the cluster)
  - Checks for updates once an hour
  - Self updates with the latest [agent.yaml](service/static/agent.yaml.in)
    - RollingUpdate with **auto recovery** if the new version fails
  - Creates/Updates Weave Cloud agents currently using the [Launch Generator](https://github.com/weaveworks/launch-generator/) (internal)
- [Service](service) (get.weave.works)
  - `/` - [install.sh](service/static/install.sh)
  - `/bootstrap?dist=...` - [bootstrap](bootstrap)
  - `/k8s/agent.yaml` - [agent.yaml.in](service/static/agent.yaml.in)

## Running the integration tests locally

Launcher has quite few components and we provide a way to test the full end to
end flow in a local minukube:

Start by setting up a minikube instance to run the tests on:

```
# minikube profile launcher-tests
# minikube start
```

Run the tests:
```
make integration-tests
```

This script will first ensure the dependencies are built and then run:
- `reset-local-minikube.sh`
- `setup-local-minikube.sh`
- `run.sh`

One can also use the local launcher service to provision a cluster:
```
curl -Ls $(minikube service service --url) | sh -s -- --token=${WEAVE_CLOUD_TOKEN}
```
