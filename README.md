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
make
make integration-tests WEAVE_CLOUD_TOKEN=<YOUR_TEST_INSTANCE_ON_FRONTEND.DEV.WEAVE.WORKS>
```

This script will first ensure the dependencies are built and then run:
- `reset-local-minikube.sh`
- `setup-local-minikube.sh`
- `run.sh`

One can also use the local launcher service to provision a cluster:
```
curl -Ls $(minikube service service --url) | sh -s -- --token=${WEAVE_CLOUD_TOKEN}
```

## <a name="help"></a>Getting Help

If you have any questions about, feedback for or problems with `launcher`:

- Invite yourself to the <a href="https://slack.weave.works/" target="_blank">Weave Users Slack</a>.
- Ask a question on the [#general](https://weave-community.slack.com/messages/general/) slack channel.
- [File an issue](https://github.com/weaveworks/launcher/issues/new).

Weaveworks follows the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md). Instances of abusive, harassing, or otherwise unacceptable behavior may be reported by contacting a Weaveworks project maintainer, or Alexis Richardson (alexis@weave.works).

Your feedback is always welcome!
