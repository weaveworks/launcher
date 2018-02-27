#!/bin/sh
set -e

# Parse command-line arguments
for arg in "$@"; do
    case $arg in
        --token=*)
            token=$(echo $arg | cut -d '=' -f 2)
            ;;
    esac
done

if [ -z "$token" ]; then
    echo "error: please specify the instance token with --token=<TOKEN>"
    exit 1
fi

# Notify installation has started
curl -s >/dev/null 2>/dev/null -H "Accept: application/json" -H "Authorization: Bearer $token" -X POST -d \
    '{"type": "onboarding_started", "messages": {"browser": { "type": "onboarding_started", "text": "Installation of Weave Cloud agents has started"}}}' \
    {{.Scheme}}://{{.WCHostname}}/api/notification/external/events || true

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
"$TMPFILE" "--scheme={{.Scheme}}" "--hostname={{.Hostname}}" "--wc-hostname={{.WCHostname}}" "$@"
