FROM golang:1.13-alpine as build

ARG TARGETPLATFORM

ENV GO111MODULE=off \
    CGO_ENABLED=0

RUN apk add --no-cache git make bash curl binutils build-base

WORKDIR /go/src/github.com/weaveworks/launcher

RUN export GOOS=$(echo ${TARGETPLATFORM} | cut -d / -f1) && \
    export GOARCH=$(echo ${TARGETPLATFORM} | cut -d / -f2) && \
    GOARM=$(echo ${TARGETPLATFORM} | cut -d / -f3); export GOARM=${GOARM:1} && \
    git clone --depth 1 https://github.com/xunholy/launcher.git . && make all

FROM alpine:3.7

ARG REVISION

RUN apk add --no-cache ca-certificates

COPY --from=build /go/src/github.com/weaveworks/launcher/build/agent /usr/bin/launcher-agent

COPY --from=build /go/src/github.com/weaveworks/launcher/build/kubectl /usr/bin/kubectl

ENTRYPOINT ["/usr/bin/launcher-agent"]

CMD ["-help"]


LABEL maintainer="Weaveworks <help@weave.works>" \
    org.opencontainers.image.title="launcher-agent" \
    org.opencontainers.image.source="https://github.com/weaveworks/launcher" \
    org.opencontainers.image.revision="${REVISION}" \
    org.opencontainers.image.vendor="Weaveworks"
