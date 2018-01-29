# Weave Cloud Launcher

[![Circle CI](https://circleci.com/gh/weaveworks/launcher/tree/master.svg?style=shield)](https://circleci.com/gh/weaveworks/launcher/tree/master)

<h3 align="center">
  <code>curl -L https://get.weave.works | sh -s -- --token=XXXXXX</code>
</h3>

## Overview

- `curl -L https://get.weave.works | sh -s -- --token=XXXXXX` (on the host)
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
