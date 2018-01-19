# Weave Cloud Launcher (WIP)

## WIP Notes

- User will be asked to run: `curl -L https://get.weave.works | sh -s TOKEN_HERE`

### install.sh

- Downloads the bootstrap binary from https://get.weave.works/bootstrap?dist=`uname` (`bootstrap.go`)
- Executes this binary, passing the token as an argument

### bootstrap.go

- Applies the latest launcher config to the cluster using kubectl on the host.
- Uses config from https://github.com/weaveworks/config


### launcher.go

- The launcher which manages the Weave Cloud agents, making sure they are running and configured correctly.
