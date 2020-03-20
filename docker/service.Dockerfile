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

WORKDIR /

COPY --from=build /go/src/github.com/weaveworks/launcher/build/service /launcher-service

RUN mkdir static

COPY --from=build /go/src/github.com/weaveworks/launcher/build/static/install.sh /static/

COPY --from=build /go/src/github.com/weaveworks/launcher/build/static/agent.yaml /static/

EXPOSE 80

ENTRYPOINT ["/launcher-service", "--bootstrap-version=936c2cf3123d720b1357d95992d5c4b648be5a39"]

LABEL maintainer="Weaveworks <help@weave.works>" \
    org.opencontainers.image.title="launcher-service" \
    org.opencontainers.image.source="https://github.com/weaveworks/launcher" \
    org.opencontainers.image.revision="${REVISION}" \
    org.opencontainers.image.vendor="Weaveworks"