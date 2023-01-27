#!/bin/bash

# Compares two semantic versions. True if the first is lower or equal to the second.
# https://stackoverflow.com/a/4024263
verlte() {
    [  "$1" = "$(echo -e "$1\n$2" | sort -V | head -n1)" ]
}

versions=($(curl https://update.k3s.io/v1-release/channels | jq -rc '[ .data[] | select(.type == "channel") | select(.name | test("testing") | not) | .latest ] | unique | .[]'))

# Filter only versions above v1.20.0 (https://stackoverflow.com/a/40375567)
for index in "${!versions[@]}" ; do
    (verlte ${versions[$index]} v1.20.0) && unset -v 'versions[$index]'
done
versions="${versions[@]}"

amd64_flavor=("opensuse-leap" "opensuse-tumbleweed" "alpine-ubuntu" "alpine-opensuse-leap" "ubuntu" "ubuntu-20-lts" "ubuntu-22-lts" "fedora" "debian")
arm64_flavor=("opensuse-leap-arm-rpi" "opensuse-tumbleweed-arm-rpi" "alpine-arm-rpi")
arm64_models=("rpi64")
releases="[]"
releases_arm="[]"

for row in $versions; do
    for flavor in "${amd64_flavor[@]}"; do
    	releases=$(echo $releases | jq ". += [{ \"flavor\": \"$flavor\", \"k3s_version\": \"$row\" }]" )
    done
    for flavor in "${arm64_flavor[@]}"; do
        for model in "${arm64_models[@]}"; do
    		releases_arm=$(echo $releases_arm | jq ". += [{ \"flavor\": \"$flavor\", \"model\": \"$model\", \"k3s_version\": \"$row\" }]" )
    	done
    done
done

echo $releases_arm | jq  > releases-arm.json
echo $releases | jq > releases.json
