.PHONY: all clean agent bootstrap service

DOCKER ?= docker

# Inspiration taken from https://github.com/weaveworks/flux/blob/master/Makefile

# NB because this outputs absolute file names, you have to be careful
# if you're testing out the Makefile with `-W` (pretend a file is
# new); use the full path to the pretend-new file, e.g.,
#  `make -W $PWD/registry/registry.go`
godeps=$(shell go list -f '{{join .Deps "\n"}}' $1 | grep -v /vendor/ | xargs go list -f '{{if not .Standard}}{{ $$dep := . }}{{range .GoFiles}}{{$$dep.Dir}}/{{.}} {{end}}{{end}}')

AGENT_DEPS   := $(call godeps,./agent)
BOOTSTRAP_DEPS := $(call godeps,./bootstrap)
SERVICE_DEPS := $(call godeps,./service)

IMAGE_TAG:=$(shell ./docker/image-tag)

all: agent bootstrap service
agent: build/.agent.done
bootstrap: build/.bootstrap.done
service: build/.service.done

build/.%.done: docker/Dockerfile.%
	mkdir -p ./build/docker/$*
	cp $^ ./build/docker/$*/
	${DOCKER} build -t quay.io/weaveworks/launcher-$* -t quay.io/weaveworks/launcher-$*:$(IMAGE_TAG) -f build/docker/$*/Dockerfile.$* ./build/docker/$*
	touch $@

#
# Agent
#

build/.agent.done: build/agent build/kubectl

build/agent: $(AGENT_DEPS)
build/agent: agent/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ $(LDFLAGS) ./agent

include docker/kubectl.version

build/kubectl: cache/kubectl-$(KUBECTL_VERSION) docker/kubectl.version
	cp cache/kubectl-$(KUBECTL_VERSION) $@
	strip $@
	chmod a+x $@

cache/kubectl-$(KUBECTL_VERSION):
	mkdir -p cache
	curl -L -o $@ "https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VERSION)/bin/linux/amd64/kubectl"

# Bootstrap
#

build/.bootstrap.done: $(BOOTSTRAP_DEPS)
build/.bootstrap.done: bootstrap/*.go
	for arch in amd64; do \
		for os in linux darwin; do \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -o "build/bootstrap/bootstrap_"$$os"_"$$arch $(LDFLAGS) ./bootstrap; \
		done; \
	done
	touch $@

#
# Service
#

build/.service.done: build/service

build/service: $(SERVICE_DEPS)
build/service: service/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ $(LDFLAGS) ./service

clean:
	rm -rf build cache