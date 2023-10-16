FROM golang:1.20.5-alpine AS build-env

# Set up dependencies
ENV PACKAGES bash curl make git libc-dev gcc linux-headers eudev-dev python3

# ADD . /code
WORKDIR /code

COPY . .

ADD https://github.com/CosmWasm/wasmvm/releases/download/v1.2.1/libwasmvm_muslc.x86_64.a /lib/libwasmvm_muslc.a

RUN apk add --no-cache $PACKAGES && \
    BUILD_TAGS=muslc LINK_STATICALLY=true make install && \
    rm -rf /var/cache/apk/*

FROM alpine:edge

RUN apk add --update ca-certificates

WORKDIR /code

COPY --from=build-env /go/bin/passage /usr/local/bin/passage

CMD ["passage"]