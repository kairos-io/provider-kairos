VERSION 0.6

IMPORT github.com/kairos-io/kairos

FROM alpine
ARG OS_ID=kairos
ARG VARIANT=standard
ARG FLAVOR=opensuse-leap

## Versioning
ARG K3S_VERSION
RUN apk add git
COPY . ./
RUN echo $(git describe --always --tags --dirty) > VERSION
ARG CORE_VERSION=$(cat CORE_VERSION || echo "latest")
ARG VERSION=$(cat VERSION)
RUN echo "version ${VERSION}"
ARG K3S_VERSION_TAG=$(echo $K3S_VERSION | sed s/+/-/)
ARG TAG=${VERSION}-k3s${K3S_VERSION_TAG}
ARG BASE_REPO=quay.io/kairos
ARG IMAGE=${BASE_REPO}/${OS_ID}-${FLAVOR}:$TAG
ARG BASE_IMAGE=quay.io/kairos/core-${FLAVOR}:${CORE_VERSION}
# renovate: datasource=docker depName=quay.io/kairos/osbuilder-tools versioning=semver-coerced
ARG OSBUILDER_VERSION=v0.8.2
ARG OSBUILDER_IMAGE=quay.io/kairos/osbuilder-tools:$OSBUILDER_VERSION

## External deps pinned versions
ARG LUET_VERSION=0.33.0
# renovate: datasource=docker depName=golang
ARG GO_VERSION=1.20

ARG MODEL=generic
ARG TARGETARCH
ARG CGO_ENABLED=0

RELEASEVERSION:
    COMMAND
    RUN echo "$IMAGE" > IMAGE
    RUN echo "$VERSION" > VERSION
    SAVE ARTIFACT VERSION AS LOCAL build/VERSION
    SAVE ARTIFACT IMAGE AS LOCAL build/IMAGE

all-arm-generic:
  BUILD --platform=linux/arm64 +docker
  BUILD --platform=linux/arm64 +image-sbom
  BUILD --platform=linux/arm64 +iso
  DO +RELEASEVERSION

all:
  ARG SECURITY_SCANS=true
  BUILD +docker
  IF [ "$SECURITY_SCANS" = "true" ]
      BUILD +image-sbom
  END
  BUILD +iso
  BUILD +netboot
  BUILD +ipxe-iso
  DO +RELEASEVERSION

ci:
  BUILD +docker
  BUILD +iso

all-arm:
  ARG SECURITY_SCANS=true
  BUILD --platform=linux/arm64 +docker
  IF [ "$SECURITY_SCANS" = "true" ]
      BUILD --platform=linux/arm64  +image-sbom --MODEL=rpi64
  END
  BUILD +arm-image --MODEL=rpi64
  DO +RELEASEVERSION

go-deps:
    ARG GO_VERSION
    FROM golang:$GO_VERSION
    WORKDIR /build
    COPY go.mod go.sum ./
    RUN go mod download
    SAVE ARTIFACT go.mod AS LOCAL go.mod
    SAVE ARTIFACT go.sum AS LOCAL go.sum

test:
    FROM +go-deps
    WORKDIR /build
    COPY (kairos+luet/luet) /usr/bin/luet
    COPY . .
    RUN go run github.com/onsi/ginkgo/v2/ginkgo --fail-fast --covermode=atomic --coverprofile=coverage.out -p -r ./internal
    SAVE ARTIFACT coverage.out AS LOCAL coverage.out

BUILD_GOLANG:
    COMMAND
    WORKDIR /build
    COPY . ./
    ARG CGO_ENABLED
    ARG VERSION
    ARG LDFLAGS="-s -w -X 'github.com/kairos-io/provider-kairos/v2/internal/cli.VERSION=$VERSION'"
    ARG BIN
    ARG SRC
    ENV CGO_ENABLED=${CGO_ENABLED}
    RUN echo $LDFLAGS
    RUN go build -ldflags "${LDFLAGS}" -o ${BIN} ${SRC}
    SAVE ARTIFACT ${BIN} ${BIN} AS LOCAL build/${BIN}

build-kairos-agent-provider:
    FROM +go-deps
    DO +BUILD_GOLANG --BIN=agent-provider-kairos --SRC=./ --CGO_ENABLED=$CGO_ENABLED

build-kairosctl:
    FROM +go-deps
    DO +BUILD_GOLANG --BIN=kairosctl --SRC=./cli/kairosctl --CGO_ENABLED=$CGO_ENABLED

build:
    BUILD +build-kairos-agent-provider
    BUILD +build-kairosctl

version:
    FROM alpine
    RUN apk add git

    COPY . ./

    RUN --no-cache echo $(git describe --always --tags --dirty) > VERSION

    ARG VERSION=$(cat VERSION)
    SAVE ARTIFACT VERSION VERSION

docker:
    ARG FLAVOR
    ARG VARIANT
    FROM $BASE_IMAGE

    DO +PROVIDER_INSTALL

    ARG KAIROS_VERSION
    IF [ "$KAIROS_VERSION" = "" ]
        ARG OS_VERSION=${VERSION}
    ELSE
        ARG OS_VERSION=${KAIROS_VERSION}
    END

    ARG OS_ID
    ARG OS_NAME=${OS_ID}-${FLAVOR}
    ARG OS_REPO=quay.io/kairos/${OS_ID}-${FLAVOR}
    ARG OS_LABEL=latest

    DO kairos+OSRELEASE --BUG_REPORT_URL="https://github.com/kairos-io/kairos/issues/new/choose" --HOME_URL="https://github.com/kairos-io/provider-kairos" --OS_ID=${OS_ID} --OS_LABEL=${OS_LABEL} --OS_NAME=${OS_NAME} --OS_REPO=${OS_REPO} --OS_VERSION=${OS_VERSION}-k3s${K3S_VERSION} --GITHUB_REPO="kairos-io/provider-kairos" --VARIANT=${VARIANT} --FLAVOR=${FLAVOR}

    SAVE IMAGE $IMAGE

# This install the requirements for the provider to be included.
# Made as a command so it can be reused from other targets without depending on this repo BASE_IMAGE
PROVIDER_INSTALL:
    COMMAND
    IF [ "$K3S_VERSION" = "latest" ]
    ELSE
        ENV INSTALL_K3S_VERSION=${K3S_VERSION}
    END

    IF [ "$FLAVOR" = "opensuse-leap" ] || [ "$FLAVOR" = "opensuse-leap-arm-rpi" ]
      RUN zypper ref && zypper in -y nohang
    ELSE IF [ "$FLAVOR" = "alpine-ubuntu" ] || [ "$FLAVOR" = "alpine-opensuse-leap" ] || [ "$FLAVOR" = "alpine-arm-rpi" ]
      RUN apk add grep
    ELSE IF [ "$FLAVOR" = "opensuse-tumbleweed" ] || [ "$FLAVOR" = "opensuse-tumbleweed-arm-rpi" ]
      RUN zypper ref && zypper in -y nohang
    ELSE IF [ "$FLAVOR" = "ubuntu" ] || [ "$FLAVOR" = "ubuntu-20-lts" ] || [ "$FLAVOR" = "ubuntu-22-lts" ] || [ "$FLAVOR" = "debian" ]
      RUN apt-get update && apt-get install -y nohang
    END

    ENV INSTALL_K3S_BIN_DIR="/usr/bin"
    RUN curl -sfL https://get.k3s.io > installer.sh \
        && INSTALL_K3S_SELINUX_WARN=true INSTALL_K3S_SKIP_START="true" INSTALL_K3S_SKIP_ENABLE="true" INSTALL_K3S_SKIP_SELINUX_RPM="true" bash installer.sh \
        && INSTALL_K3S_SELINUX_WARN=true INSTALL_K3S_SKIP_START="true" INSTALL_K3S_SKIP_ENABLE="true" INSTALL_K3S_SKIP_SELINUX_RPM="true" bash installer.sh agent \
        && rm -rf installer.sh

    # If base image does not bundle a luet config use one
    # TODO: Remove this, use luet config from base images so they are in sync
    IF [ ! -e "/etc/luet/luet.yaml" ]
        COPY repository.yaml /etc/luet/luet.yaml
    END

    RUN luet install -y utils/edgevpn utils/k9s utils/nerdctl container/kubectl utils/kube-vip && luet cleanup
    # Drop env files from k3s as we will generate them
    IF [ -e "/etc/rancher/k3s/k3s.env" ]
        RUN rm -rf /etc/rancher/k3s/k3s.env /etc/rancher/k3s/k3s-agent.env && touch /etc/rancher/k3s/.keep
    END

    COPY +build-kairos-agent-provider/agent-provider-kairos /system/providers/agent-provider-kairos
    RUN ln -s /system/providers/agent-provider-kairos /usr/bin/kairos


docker-rootfs:
    FROM +docker
    SAVE ARTIFACT /. rootfs

kairos:
   ARG KAIROS_VERSION=master
   FROM alpine
   RUN apk add git
   WORKDIR /kairos
   RUN git clone https://github.com/kairos-io/kairos /kairos && cd /kairos && git checkout "$KAIROS_VERSION"
   SAVE ARTIFACT /kairos/

iso:
    ARG OSBUILDER_IMAGE
    ARG IMG=docker:$IMAGE
    ARG overlay=overlay/files-iso
    IF [ "$TARGETARCH" = "arm64" ]
        ARG DISTRO=$(echo $FLAVOR | sed 's/-arm-.*//')
        ARG ISO_NAME=${OS_ID}-${VARIANT}-${DISTRO}-${TARGETARCH}-${MODEL}-${VERSION}
    ELSE
        ARG ISO_NAME=${OS_ID}-${VARIANT}-${FLAVOR}-${TARGETARCH}-${MODEL}-${VERSION}
    END
    FROM $OSBUILDER_IMAGE
    RUN zypper in -y jq docker
    WORKDIR /build
    COPY . ./
    RUN mkdir -p overlay/files-iso
    COPY +kairos/kairos/overlay/files-iso/ ./$overlay/
    COPY +docker-rootfs/rootfs /build/image
    RUN /entrypoint.sh --name $ISO_NAME --debug build-iso --date=false dir:/build/image --overlay-iso /build/${overlay} --output /build
    # See: https://github.com/rancher/elemental-cli/issues/228
    RUN sha256sum $ISO_NAME.iso > $ISO_NAME.iso.sha256
    SAVE ARTIFACT /build/$ISO_NAME.iso kairos.iso AS LOCAL build/$ISO_NAME.iso
    SAVE ARTIFACT /build/$ISO_NAME.iso.sha256 kairos.iso.sha256 AS LOCAL build/$ISO_NAME.iso.sha256

netboot:
   FROM opensuse/leap
   ARG VERSION
   IF [ "$TARGETARCH" = "arm64" ]
       ARG DISTRO=$(echo $FLAVOR | sed 's/-arm-.*//')
       ARG ISO_NAME=${OS_ID}-${VARIANT}-${DISTRO}-${TARGETARCH}-${MODEL}-${VERSION}
   ELSE
       ARG ISO_NAME=${OS_ID}-${VARIANT}-${FLAVOR}-${TARGETARCH}-${MODEL}-${VERSION}
   END
   WORKDIR /build
   COPY +iso/kairos.iso kairos.iso
   COPY . .
   RUN zypper in -y cdrtools

   COPY +kairos/kairos/scripts/netboot.sh ./
   RUN sh netboot.sh kairos.iso $ISO_NAME $VERSION

   SAVE ARTIFACT /build/$ISO_NAME.squashfs squashfs AS LOCAL build/$ISO_NAME.squashfs
   SAVE ARTIFACT /build/$ISO_NAME-kernel kernel AS LOCAL build/$ISO_NAME-kernel
   SAVE ARTIFACT /build/$ISO_NAME-initrd initrd AS LOCAL build/$ISO_NAME-initrd
   SAVE ARTIFACT /build/$ISO_NAME.ipxe ipxe AS LOCAL build/$ISO_NAME.ipxe

arm-image:
  ARG OSBUILDER_IMAGE
  ARG COMPRESS_IMG=true
  FROM $OSBUILDER_IMAGE
  ARG MODEL=rpi64
  ARG DISTRO=$(echo $FLAVOR | sed 's/-arm-.*//')
  # TARGETARCH is not used here because OSBUILDER_IMAGE is not available in arm64. When this changes, then the caller
  # of this target can simply pass the desired TARGETARCH.
  ARG IMAGE_NAME=${OS_ID}-${VARIANT}-${DISTRO}-arm64-${MODEL}-${VERSION}-k3s${K3S_VERSION}.img
  WORKDIR /build

  ENV SIZE="15200"

  IF [[ "$FLAVOR" = "ubuntu-20-lts-arm-nvidia-jetson-agx-orin" ]]
    ENV STATE_SIZE="14000"
    ENV RECOVERY_SIZE="10000"
    ENV DEFAULT_ACTIVE_SIZE="4500"
  ELSE IF [[ "$FLAVOR" =~ ^ubuntu* ]]
    ENV STATE_SIZE="6900"
    ENV RECOVERY_SIZE="4600"
    ENV DEFAULT_ACTIVE_SIZE="2700"
  ELSE
    ENV STATE_SIZE="6200"
    ENV RECOVERY_SIZE="4200"
    ENV DEFAULT_ACTIVE_SIZE="2000"
  END


  COPY --platform=linux/arm64 +docker-rootfs/rootfs /build/image
  # With docker is required for loop devices
  WITH DOCKER --allow-privileged
    RUN /build-arm-image.sh --use-lvm --model $MODEL --directory "/build/image" /build/$IMAGE_NAME
  END
  IF [ "$COMPRESS_IMG" = "true" ]
    RUN xz -v /build/$IMAGE_NAME
    SAVE ARTIFACT /build/$IMAGE_NAME.xz img AS LOCAL build/$IMAGE_NAME.xz
  ELSE
    SAVE ARTIFACT /build/$IMAGE_NAME img AS LOCAL build/$IMAGE_NAME
  END
  SAVE ARTIFACT /build/$IMAGE_NAME.sha256 img-sha256 AS LOCAL build/$IMAGE_NAME.sha256

syft:
    FROM anchore/syft:latest
    SAVE ARTIFACT /syft syft

image-sbom:
    FROM +docker
    WORKDIR /build
    ARG TAG
    ARG FLAVOR
    ARG VARIANT
    ARG MODEL
    IF [ "$TARGETARCH" = "arm64" ]
        ARG DISTRO=$(echo $FLAVOR | sed 's/-arm-.*//')
        ARG ISO_NAME=${OS_ID}-${VARIANT}-${DISTRO}-${TARGETARCH}-${MODEL}-${VERSION}
    ELSE
        ARG ISO_NAME=${OS_ID}-${VARIANT}-${FLAVOR}-${TARGETARCH}-${MODEL}-${VERSION}
    END
    COPY +syft/syft /usr/bin/syft
    RUN syft / -o json=sbom.syft.json -o spdx-json=sbom.spdx.json
    SAVE ARTIFACT /build/sbom.syft.json sbom.syft.json AS LOCAL build/${ISO_NAME}-sbom.syft.json
    SAVE ARTIFACT /build/sbom.spdx.json sbom.spdx.json AS LOCAL build/${ISO_NAME}-sbom.spdx.json

ipxe-iso:
    FROM ubuntu
    ARG ipxe_script
    RUN apt update
    RUN apt install -y -o Acquire::Retries=50 \
                           mtools syslinux isolinux gcc-arm-none-eabi git make gcc liblzma-dev mkisofs xorriso
                           # jq docker
    WORKDIR /build
    RUN git clone https://github.com/ipxe/ipxe
    IF [ "$ipxe_script" = "" ]
        COPY +netboot/ipxe /build/ipxe/script.ipxe
    ELSE
        COPY $ipxe_script /build/ipxe/script.ipxe
    END
    RUN cd ipxe/src && make EMBED=/build/ipxe/script.ipxe
    SAVE ARTIFACT /build/ipxe/src/bin/ipxe.iso iso AS LOCAL build/${ISO_NAME}-ipxe.iso
    SAVE ARTIFACT /build/ipxe/src/bin/ipxe.usb usb AS LOCAL build/${ISO_NAME}-ipxe-usb.img

## Security targets
trivy:
    FROM aquasec/trivy
    SAVE ARTIFACT /usr/local/bin/trivy /trivy

trivy-scan:
    ARG SEVERITY=CRITICAL
    FROM +docker
    COPY +trivy/trivy /trivy
    RUN /trivy filesystem --severity $SEVERITY --exit-code 1 --no-progress /
