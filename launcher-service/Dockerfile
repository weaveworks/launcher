FROM alpine:3.6
MAINTAINER Weaveworks Inc <help@weave.works>
WORKDIR /
COPY launcher-service /
EXPOSE 80
ENTRYPOINT ["/launcher-service", "--bootstrap-version=5d95af2"]
