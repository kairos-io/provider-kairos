#!/bin/bash

versions=$(curl https://update.k3s.io/v1-release/channels | jq -rc '.data[] | select(.type == "channel") | select(.name | test("testing") | not) | .latest')

amd64_flavor=("opensuse" "alpine" "ubuntu" "ubuntu-rolling" "fedora")
arm64_flavor=("opensuse-arm-rpi" "alpine-arm-rpi")
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