VERSION 0.6

IMPORT github.com/c3os-io/c3os

FROM alpine
ARG VARIANT=c3os # core, lite, framework
ARG FLAVOR=opensuse
ARG IMAGE=quay.io/c3os/${VARIANT}-${FLAVOR}:latest
ARG BASE_IMAGE=quay.io/c3os/core-${FLAVOR}:latest
ARG ISO_NAME=c3os-${VARIANT}-${FLAVOR}
ARG LUET_VERSION=0.32.4
ARG OS_ID=c3os

ARG CGO_ENABLED=0
ARG ELEMENTAL_IMAGE=quay.io/costoolkit/elemental-cli:v0.0.15-8a78e6b
ARG GOLINT_VERSION=1.47.3
ARG GO_VERSION=1.18

all:
  BUILD +docker
  BUILD +iso
  BUILD +netboot
  BUILD +ipxe-iso

all-arm:
  BUILD --platform=linux/arm64 +docker
  BUILD +arm-image

go-deps:
    ARG GO_VERSION
    FROM golang:$GO_VERSION
    WORKDIR /build
    COPY go.mod go.sum ./
    RUN go mod download
    RUN apt-get update && apt-get install -y upx
    SAVE ARTIFACT go.mod AS LOCAL go.mod
    SAVE ARTIFACT go.sum AS LOCAL go.sum

test:
    FROM +go-deps
    WORKDIR /build
    RUN go get github.com/onsi/gomega/...
    RUN go get github.com/onsi/ginkgo/v2/ginkgo/internal@v2.1.4
    RUN go get github.com/onsi/ginkgo/v2/ginkgo/generators@v2.1.4
    RUN go get github.com/onsi/ginkgo/v2/ginkgo/labels@v2.1.4
    RUN go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo
    RUN curl https://luet.io/install.sh | sh
    COPY . .
    RUN ginkgo run --fail-fast --slow-spec-threshold 30s --covermode=atomic --coverprofile=coverage.out -p -r ./internal
    SAVE ARTIFACT coverage.out AS LOCAL coverage.out

BUILD_GOLANG:
    COMMAND
    WORKDIR /build
    COPY . ./
    ARG CGO_ENABLED
    ARG BIN
    ARG SRC
    ENV CGO_ENABLED=${CGO_ENABLED}

    RUN go build -ldflags "-s -w" -o ${BIN} ${SRC} && upx ${BIN}
    SAVE ARTIFACT ${BIN} ${BIN} AS LOCAL build/${BIN}

version:
    FROM alpine
    RUN apk add git

    COPY . ./

    RUN echo $(git describe --exact-match --tags || echo "v0.0.0-$(git log --oneline -n 1 | cut -d" " -f1)") > VERSION

    SAVE ARTIFACT VERSION VERSION

build-c3os-agent-provider:
    FROM +go-deps
    DO +BUILD_GOLANG --BIN=agent-provider-c3os --SRC=./ --CGO_ENABLED=$CGO_ENABLED

build:
    BUILD +build-c3os-agent-provider

dist:
    ARG GO_VERSION
    FROM golang:$GO_VERSION
    RUN echo 'deb [trusted=yes] https://repo.goreleaser.com/apt/ /' | tee /etc/apt/sources.list.d/goreleaser.list
    RUN apt update
    RUN apt install -y goreleaser
    WORKDIR /build
    COPY . .
    RUN goreleaser build --rm-dist --skip-validate --snapshot
    SAVE ARTIFACT /build/dist/* AS LOCAL dist/

lint:
    ARG GO_VERSION
    FROM golang:$GO_VERSION
    ARG GOLINT_VERSION
    RUN wget -O- -nv https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v$GOLINT_VERSION
    WORKDIR /build
    COPY . .
    RUN golangci-lint run

docker:
    ARG FLAVOR
    ARG VARIANT
    ARG K3S_VERSION

    FROM $BASE_IMAGE

    IF [ "$K3S_VERSION" = "latest" ]
    ELSE
        ENV INSTALL_K3S_VERSION=${K3S_VERSION}
    END

    ENV INSTALL_K3S_BIN_DIR="/usr/bin"
    RUN curl -sfL https://get.k3s.io > installer.sh \
        && INSTALL_K3S_SKIP_START="true" INSTALL_K3S_SKIP_ENABLE="true" bash installer.sh \
        && INSTALL_K3S_SKIP_START="true" INSTALL_K3S_SKIP_ENABLE="true" bash installer.sh agent \
        && rm -rf installer.sh

    COPY +build-c3os-agent-provider/agent-provider-c3os /system/providers/agent-provider-c3os

    ARG C3OS_VERSION
    IF [ "$C3OS_VERSION" = "" ]
        COPY +version/VERSION ./
        ARG VERSION=$(cat VERSION)
        RUN echo "version ${VERSION}"
        IF [ "$VARIANT" = "" ]
            ARG OS_VERSION=c3OS-${VERSION}
        ELSE
            ARG OS_VERSION=c3OS-${VARIANT}-${VERSION}
        END
        
        RUN rm VERSION
    ELSE 
        ARG OS_VERSION=c3OS-${VARIANT}-${C3OS_VERSION}
    END
    
    ARG OS_ID
    ARG OS_NAME=${OS_ID}-${FLAVOR}
    ARG OS_REPO=quay.io/c3os/${VARIANT}-${FLAVOR}
    ARG OS_LABEL=${FLAVOR}-latest

    DO c3os+OSRELEASE --OS_ID=${OS_ID} --OS_LABEL=${OS_LABEL} --OS_NAME=${OS_NAME} --OS_REPO=${OS_REPO} --OS_VERSION=${OS_VERSION}

    SAVE IMAGE $IMAGE

docker-rootfs:
    FROM +docker
    SAVE ARTIFACT /. rootfs

elemental:
    ARG ELEMENTAL_IMAGE
    FROM ${ELEMENTAL_IMAGE}
    SAVE ARTIFACT /usr/bin/elemental elemental

c3os:
   ARG C3OS_VERSION=master
   FROM alpine
   RUN apk add git
   WORKDIR /c3os
   RUN git clone https://github.com/c3os-io/c3os /c3os && cd /c3os && git checkout "$C3OS_VERSION"
   SAVE ARTIFACT /c3os/

get-c3os-scripts:
    FROM alpine
    WORKDIR /build
    COPY +c3os/c3os/ ./
    SAVE ARTIFACT /build/scripts AS LOCAL scripts

iso:
    ARG ELEMENTAL_IMAGE
    ARG ISO_NAME=${OS_ID}
    ARG IMG=docker:$IMAGE
    ARG overlay=overlay/files-iso

    ARG TOOLKIT_REPOSITORY=quay.io/costoolkit/releases-teal
    FROM $ELEMENTAL_IMAGE
    RUN zypper in -y jq docker
    WORKDIR /build

    COPY . ./
    RUN mkdir -p overlay/files-iso
    COPY +c3os/c3os/overlay/files-iso/ ./overlay/files-iso/

    WITH DOCKER --allow-privileged --load $IMAGE=(+docker)
        RUN elemental --repo $TOOLKIT_REPOSITORY --name $ISO_NAME --debug build-iso --date=false --local --overlay-iso /build/${overlay} $IMAGE --output /build/
    END
    # See: https://github.com/rancher/elemental-cli/issues/228
    RUN sha256sum $ISO_NAME.iso > $ISO_NAME.iso.sha256
    SAVE ARTIFACT /build/$ISO_NAME.iso c3os.iso AS LOCAL build/$ISO_NAME.iso
    SAVE ARTIFACT /build/$ISO_NAME.iso.sha256 c3os.iso.sha256 AS LOCAL build/$ISO_NAME.iso.sha256

netboot:
   FROM opensuse/leap
   ARG VERSION
   ARG ISO_NAME=${OS_ID}
   WORKDIR /build
   COPY +iso/c3os.iso c3os.iso
   COPY . .
   RUN zypper in -y cdrtools

   COPY +c3os/c3os/scripts/netboot.sh ./
   RUN sh netboot.sh c3os.iso $ISO_NAME $VERSION

   SAVE ARTIFACT /build/$ISO_NAME.squashfs squashfs AS LOCAL build/$ISO_NAME.squashfs
   SAVE ARTIFACT /build/$ISO_NAME-kernel kernel AS LOCAL build/$ISO_NAME-kernel
   SAVE ARTIFACT /build/$ISO_NAME-initrd initrd AS LOCAL build/$ISO_NAME-initrd
   SAVE ARTIFACT /build/$ISO_NAME.ipxe ipxe AS LOCAL build/$ISO_NAME.ipxe

arm-image:
  ARG ELEMENTAL_IMAGE
  FROM $ELEMENTAL_IMAGE
  ARG MODEL=rpi64
  ARG IMAGE_NAME=${FLAVOR}.img
  RUN zypper in -y jq docker git curl gptfdisk kpartx sudo
  #COPY +luet/luet /usr/bin/luet
  WORKDIR /build
  RUN git clone https://github.com/rancher/elemental-toolkit && mkdir elemental-toolkit/build
  RUN curl https://luet.io/install.sh | sh
  ENV STATE_SIZE="6200"
  ENV RECOVERY_SIZE="4200"
  ENV SIZE="15200"
  ENV DEFAULT_ACTIVE_SIZE="2000"
  COPY --platform=linux/arm64 +docker-rootfs/rootfs /build/image
  # With docker is required for loop devices
  WITH DOCKER --allow-privileged
    RUN cd elemental-toolkit && \
          ./images/arm-img-builder.sh --model $MODEL --directory "/build/image" build/$IMAGE_NAME && mv build ../
  END
  RUN xz -v /build/build/$IMAGE_NAME
  SAVE ARTIFACT /build/build/$IMAGE_NAME.xz img AS LOCAL build/$IMAGE_NAME
  SAVE ARTIFACT /build/build/$IMAGE_NAME.sha256 img-sha256 AS LOCAL build/$IMAGE_NAME.sha256

ipxe-iso:
    FROM ubuntu
    ARG ipxe_script
    RUN apt update
    RUN apt install -y -o Acquire::Retries=50 \
                           mtools syslinux isolinux gcc-arm-none-eabi git make gcc liblzma-dev mkisofs xorriso
                           # jq docker
    WORKDIR /build
    ARG ISO_NAME=${OS_ID}
    RUN git clone https://github.com/ipxe/ipxe
    IF [ "$ipxe_script" = "" ]
        COPY +netboot/ipxe /build/ipxe/script.ipxe
    ELSE
        COPY $ipxe_script /build/ipxe/script.ipxe
    END
    RUN cd ipxe/src && make EMBED=/build/ipxe/script.ipxe
    SAVE ARTIFACT /build/ipxe/src/bin/ipxe.iso iso AS LOCAL build/${ISO_NAME}-ipxe.iso.ipxe
    SAVE ARTIFACT /build/ipxe/src/bin/ipxe.usb usb AS LOCAL build/${ISO_NAME}-ipxe-usb.img.ipxe


## Security targets
trivy:
    FROM aquasec/trivy
    SAVE ARTIFACT /usr/local/bin/trivy /trivy

trivy-scan:
    ARG SEVERITY=CRITICAL
    FROM +docker
    COPY +trivy/trivy /trivy
    RUN /trivy filesystem --severity $SEVERITY --exit-code 1 --no-progress /

linux-bench:
    ARG GO_VERSION
    FROM golang:$GO_VERSION
    GIT CLONE https://github.com/aquasecurity/linux-bench /linux-bench-src
    RUN cd /linux-bench-src && CGO_ENABLED=0 go build -o linux-bench . && mv linux-bench /
    SAVE ARTIFACT /linux-bench /linux-bench

# The target below should run on a live host instead. 
# However, some checks are relevant as well at container level.
# It is good enough for a quick assessment.
linux-bench-scan:
    FROM +docker
    GIT CLONE https://github.com/aquasecurity/linux-bench /build/linux-bench
    WORKDIR /build/linux-bench
    COPY +linux-bench/linux-bench /build/linux-bench/linux-bench
    RUN /build/linux-bench/linux-bench

# Generic targets
# usage e.g. ./earthly.sh +datasource-iso --CLOUD_CONFIG=tests/assets/qrcode.yaml
datasource-iso:
  ARG ELEMENTAL_IMAGE
  ARG CLOUD_CONFIG
  FROM $ELEMENTAL_IMAGE
  RUN zypper in -y mkisofs
  WORKDIR /build
  RUN touch meta-data
  COPY ./${CLOUD_CONFIG} user-data
  RUN cat user-data
  RUN mkisofs -output ci.iso -volid cidata -joliet -rock user-data meta-data
  SAVE ARTIFACT /build/ci.iso iso.iso AS LOCAL build/datasource.iso

# usage e.g. ./earthly.sh +run-qemu-tests --FLAVOR=alpine --FROM_ARTIFACTS=true
run-qemu-tests:
    FROM opensuse/leap
    WORKDIR /test
    RUN zypper in -y qemu-x86 qemu-arm qemu-tools go
    ARG FLAVOR
    ARG TEST_SUITE=autoinstall-test
    ARG FROM_ARTIFACTS
    ENV FLAVOR=$FLAVOR
    ENV SSH_PORT=60022
    ENV CREATE_VM=true
    ARG CLOUD_CONFIG="/tests/tests/assets/autoinstall.yaml"
    ENV USE_QEMU=true

    ENV GOPATH="/go"

    RUN go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo
    ENV CLOUD_CONFIG=$CLOUD_CONFIG

    IF [ "$FROM_ARTIFACTS" = "true" ]
        COPY . .
        ENV ISO=/test/build/c3os.iso
        ENV DATASOURCE=/test/build/datasource.iso
    ELSE
        COPY ./tests .
        COPY +iso/c3os.iso c3os.iso
        COPY ( +datasource-iso/iso.iso --CLOUD_CONFIG=$CLOUD_CONFIG) datasource.iso
        ENV ISO=/test/c3os.iso
        ENV DATASOURCE=/test/datasource.iso
    END

    ENV CLOUD_INIT=$CLOUD_CONFIG

    RUN PATH=$PATH:$GOPATH/bin ginkgo --label-filter "$TEST_SUITE" --fail-fast -r ./tests/

test-create-config:
    FROM alpine
    COPY +build-c3os-agent-provider/agent-provider-c3os agent-provider-c3os
    RUN ./agent-provider-c3os create-config > config.yaml
    SAVE ARTIFACT config.yaml AS LOCAL config.yaml
