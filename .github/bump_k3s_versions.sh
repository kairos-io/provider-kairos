#!/bin/bash

# Compares two semantic versions. True if the first is lower or equal to the second.
# https://stackoverflow.com/a/4024263
verlte() {
    [  "$1" = "$(echo -e "$1\n$2" | sort -V | head -n1)" ]
}

# https://www.shellcheck.net/wiki/SC2207
mapfile -t versionsArray < <(curl -s https://update.k3s.io/v1-release/channels | jq -rc '[ .data[] | select(.type == "channel") | select(.name | test("testing") | not) | .latest ] | unique | .[]')

# This gives us the latest stable version minor number
latest_version=$(curl -s https://update.k3s.io/v1-release/channels | jq -rc '.data[] | select(.id == "latest") | .latest' | cut -d. -f2)
# Supported versions are the latest 3 ones:
# "The Kubernetes project maintains release branches for the most recent three minor releases (1.27, 1.26, 1.25)"
# from https://kubernetes.io/releases/
# So we calculate that based on the $(latest release - 2) to get the supported upstream versions
supported_version=$((latest_version-2))
echo "Supported minimum version: v1.$supported_version.0"

# Filter only versions above v1.$supported_version.0 (https://stackoverflow.com/a/40375567)
for index in "${!versionsArray[@]}" ; do
    (verlte "${versionsArray[$index]}" v1.$supported_version.0) && unset -v 'versionsArray[$index]'
done
versions="${versionsArray[*]}"
echo "Found supported versions: $versions"

amd64_flavor=("opensuse-leap" "opensuse-tumbleweed" "alpine-ubuntu" "alpine-opensuse-leap" "ubuntu" "ubuntu-20-lts" "ubuntu-22-lts" "fedora" "debian")
arm64_flavor=("opensuse-leap-arm-rpi" "opensuse-tumbleweed-arm-rpi" "alpine-arm-rpi")
arm64_models=("rpi64")
releases="[]"
releases_arm="[]"

for row in $versions; do
  for flavor in "${amd64_flavor[@]}"; do
    echo "Adding version $row for flavor $flavor on amd64"
    releases=$(echo "$releases" | jq ". += [{ \"flavor\": \"$flavor\", \"k3s_version\": \"$row\" }]" )
  done
  for flavor in "${arm64_flavor[@]}"; do
    for model in "${arm64_models[@]}"; do
      echo "Adding version $row for flavor $flavor and model $model on arm64"
      releases_arm=$(echo "$releases_arm" | jq ". += [{ \"flavor\": \"$flavor\", \"model\": \"$model\", \"k3s_version\": \"$row\" }]" )
    done
  done
done

echo "$releases_arm" | jq  > releases-arm.json
echo "$releases" | jq > releases.json
