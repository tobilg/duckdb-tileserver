ARG GOLANG_VERSION=1.24.6
ARG TARGETARCH=amd64
ARG VERSION=latest
ARG PLATFORM=amd64

FROM docker.io/library/golang:${GOLANG_VERSION}-bookworm AS builder
LABEL stage=tileserverbuilder

# Install build dependencies for CGO and C++ (glibc)
RUN apt-get update && apt-get install -y \
    build-essential \
    pkg-config \
  && rm -rf /var/lib/apt/lists/*

ARG TARGETARCH
ARG VERSION

WORKDIR /app
COPY . ./

# Native build on the target platform (buildx handles emulation)
RUN CGO_ENABLED=1 go build -v -ldflags "-s -w -X github.com/tobilg/duckdb-tileserver/internal/conf.setVersion=${VERSION}"

FROM --platform=${TARGETARCH} gcr.io/distroless/cc-debian12:nonroot AS multi-stage

WORKDIR /
COPY --from=builder /app/duckdb-tileserver /duckdb-tileserver
COPY --from=builder /app/assets /assets

VOLUME ["/config"]
VOLUME ["/assets"]

EXPOSE 9000

ENTRYPOINT ["/duckdb-tileserver"]
CMD []

# Local stage for running with a locally built binary
FROM --platform=${PLATFORM} gcr.io/distroless/cc-debian12:nonroot AS local

WORKDIR /
ADD ./duckdb-tileserver /duckdb-tileserver
ADD ./assets /assets

VOLUME ["/config"]
VOLUME ["/assets"]

EXPOSE 9000
EXPOSE 9001

ENTRYPOINT ["/duckdb-tileserver"]
CMD []

# To build
# make APPVERSION=1.1 clean build docker

# To build using binaries from golang docker image
# make APPVERSION=1.1 clean multi-stage-docker
