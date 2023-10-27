VERSION 0.6

# renovate: datasource=docker depName=golang
ARG GO_VERSION=1.20
ARG TARGETARCH
ARG CGO_ENABLED=0

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
