# Weave Cloud Launcher (WIP)

## WIP Notes

- User will be asked to run: `curl -L https://get.weave.works | sh -s TOKEN_HERE`

### launcher-agent

- The launcher which manages the Weave Cloud agents, making sure they are running and configured correctly.

### launcher-bootstrap

- Applies the latest `launcher-agent` k8s config to the cluster using the host's kubectl.
- Fetches the config from https://github.com/weaveworks/config
- CircleCI uploads compiled binaries to S3

### launcher-service

- https://get.weave.works/
  - serves install.sh which downloads and executes the correct bootstrap binary
    for the host's distribution, passing the token as an argument.
- https://get.weave.works/bootstrap?dist=`uname`
  - serves the bootstrap binary for the provided distribution from S3
