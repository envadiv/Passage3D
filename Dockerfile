# syntax=docker/dockerfile:1
#
# Static (musl) passage binary with libwasmvm linked in.
#
#   docker build -t passage:local .                                    # runtime image
#   docker build --target export --output type=local,dest=build .     # just ./build/passage
#
# wasmvm MUST match go.mod (github.com/CosmWasm/wasmvm/v2 v2.2.9); Go is the release toolchain
# (scripts/build-security-release.sh). `passage start` refuses to run when the linked
# libwasmvm differs from the Go module.

ARG GO_VERSION=1.25.14
ARG ALPINE_VERSION=3.23

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder

ARG WASMVM_VERSION=v2.2.9
ARG WASMVM_SHA256_X86_64=56e7c590fe11a6a51381c80c2710f71af1244acfb8cd1d5839d638313b7bd401
ARG WASMVM_SHA256_AARCH64=4b632c22534d330b5111d7279faa7c96374e2097cbdff208cc5e040f1df41982
# Optional overrides for the version stamp (default: from git, see Makefile).
ARG VERSION
ARG COMMIT

RUN apk add --no-cache build-base git linux-headers bash make file

# libwasmvm static archive for the target architecture, checksum-verified.
RUN set -eux; \
    arch="$(uname -m)"; \
    case "$arch" in \
      x86_64)  sum="$WASMVM_SHA256_X86_64" ;; \
      aarch64) sum="$WASMVM_SHA256_AARCH64" ;; \
      *) echo "unsupported architecture: $arch"; exit 1 ;; \
    esac; \
    wget -q "https://github.com/CosmWasm/wasmvm/releases/download/${WASMVM_VERSION}/libwasmvm_muslc.${arch}.a" -O "/lib/libwasmvm_muslc.${arch}.a"; \
    echo "${sum}  /lib/libwasmvm_muslc.${arch}.a" | sha256sum -c -

WORKDIR /code

# Dependencies first for better layer caching.
COPY go.mod go.sum ./
COPY iavl-fork/go.mod iavl-fork/go.sum ./iavl-fork/
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    set -eux; \
    export LEDGER_ENABLED=true BUILD_TAGS=muslc LINK_STATICALLY=true; \
    if [ -n "${VERSION:-}" ]; then export VERSION; fi; \
    if [ -n "${COMMIT:-}" ]; then export COMMIT; fi; \
    make build; \
    file /code/build/passage | grep -q "statically linked"; \
    /code/build/passage version --long | grep -E "^(name|version|commit|build_tags|cosmos_sdk_version):"

# `docker build --target export --output type=local,dest=build .`
FROM scratch AS export
COPY --from=builder /code/build/passage /passage

FROM alpine:${ALPINE_VERSION}

RUN apk add --no-cache ca-certificates bash jq curl

COPY --from=builder /code/build/passage /usr/local/bin/passage

# Default node home: /root/.passage (mounted by docker-compose.yml).
WORKDIR /root

# p2p, rpc, rest api, grpc
EXPOSE 26656 26657 1317 9090

CMD ["passage"]
