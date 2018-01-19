#!/bin/sh

# Arrange for the bootstrap binary to be deleted when the script terminates
trap 'rm -f "/tmp/weave_bootstrap.$$"' 0
trap 'exit $?' 1 2 3 15

# Download the bootstrap binary
curl -L https://get.weave.works/bootstrap?dist=`uname` >/tmp/weave_bootstrap.$$

# Make the bootstrap binary executable
chmod +x /tmp/weave_bootstrap.$$

# Execute the boostrap binary
/tmp/weave_bootstrap.$$ $1
