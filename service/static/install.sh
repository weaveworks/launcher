#!/bin/bash
set -e

# Create a temporary file for the bootstrap binary
TMPFILE="$(mktemp -qt weave_bootstrap.XXXXXXXXXX)" || exit 1

finish(){
  # Send only when this script errors out
  # Filter out the bootstrap errors
  if [ $? -ne 111 ] && [ $? -ne 0 ]; then
    curl -s >/dev/null 2>/dev/null -H "Accept: application/json" -H "Authorization: Bearer $token" -X POST -d \
        '{"type": "onboarding_failed", "messages": {"browser": { "type": "onboarding_failed", "text": "Installation of Weave Cloud agents did not finish."}}}' \
        {{.Scheme}}://{{.WCHostname}}/api/notification/external/events || true
  fi
  # Arrange for the bootstrap binary to be deleted
  rm -f "$TMPFILE"
}

# Call finish function on exit
trap finish EXIT

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

# Get distribution
unamestr=$(uname)
if [ "$unamestr" = 'Darwin' ]; then
    dist='darwin'
elif [ "$unamestr" = 'Linux' ]; then
    dist='linux'
else
  echo "This distribution is not supported"
  exit 1
fi

# Download the bootstrap binary
echo "Downloading the Weave Cloud installer...  "
curl -Ls "{{.Scheme}}://{{.LauncherHostname}}/bootstrap?dist=$dist" >> "$TMPFILE"

# Make the bootstrap binary executable
chmod +x "$TMPFILE"

# Execute the bootstrap binary
"$TMPFILE" "--scheme={{.Scheme}}" "--wc.launcher={{.LauncherHostname}}" "--wc.hostname={{.WCHostname}}" "$@"
