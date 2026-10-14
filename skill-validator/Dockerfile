# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27

# Build every supported platform on the build host (static, no cgo).
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build
RUN apk add --no-cache make
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN make build-all VERSION=${VERSION}

FROM ghcr.io/netcracker/qubership-core-base:alpine-latest
ARG TARGETARCH
# Binaries for all platforms, to be copied out, e.g.: docker cp <container>:/opt/skillcheck/dist .
COPY --from=build /src/dist/ /opt/skillcheck/dist/
# Binary for the image platform, used by "docker run".
# COPY instead of RUN cp: the base image runs as non-root, COPY still writes as root.
COPY --from=build --chmod=755 /src/dist/skillcheck-linux-${TARGETARCH} /usr/local/bin/skillcheck
WORKDIR /work
ENTRYPOINT ["skillcheck"]
