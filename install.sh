#!/bin/sh
set -e

# Create a temporary file for the bootstrap binary
if [ TMPFILE="$(mktemp -qt weave_bootstrap)" -ne 0 ]; then
    echo "$0: Can't create temp file, exiting..."
    exit 1
fi

# Arrange for the bootstrap binary to be deleted when the script terminates
trap 'rm -f "$TMPFILE"' 0
trap 'exit $?' 1 2 3 15

# Download the bootstrap binary
curl -L "https://get.weave.works/bootstrap?dist=$(uname)" >> "$TMPFILE"

# Make the bootstrap binary executable
chmod +x "$TMPFILE"

# Execute the boostrap binary
"$TMPFILE" "$1"
