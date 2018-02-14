#!/bin/sh
set -e

# Create a temporary file for the bootstrap binary
TMPFILE="$(mktemp -qt weave_bootstrap.XXXXXXXXXX)" || exit 1

# Arrange for the bootstrap binary to be deleted when the script terminates
trap 'rm -f "$TMPFILE"' 0
trap 'exit $?' 1 2 3 15

# Get distribution
unamestr=$(uname)
if [ "$unamestr" = 'Darwin' ]; then
    dist='darwin'
elif [ "$unamestr" = 'Linux' ]; then
    dist='linux'
fi

# Download the bootstrap binary
echo "Downloading the Weave Cloud installer...  "
curl -Ls "{{.Scheme}}://{{.Hostname}}/bootstrap?dist=$dist" >> "$TMPFILE"

# Make the bootstrap binary executable
chmod +x "$TMPFILE"

# Execute the bootstrap binary
"$TMPFILE" "--scheme={{.Scheme}}" "--hostname={{.Hostname}}" "$@"
