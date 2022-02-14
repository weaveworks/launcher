.PHONY: all clean dep lint agent bootstrap service apps
.SUFFIXES:

DOCKER ?= docker

# Inspiration taken from https://github.com/weaveworks/flux/blob/master/Makefile

# NB because this outputs absolute file names, you have to be careful
# if you're testing out the Makefile with `-W` (pretend a file is
# new); use the full path to the pretend-new file, e.g.,
#  `make -W $PWD/registry/registry.go`
godeps=$(shell go list -f '{{join .Deps "\n"}}' $1 | grep -v /vendor/ | xargs go list -f '{{if not .Standard}}{{ $$dep := . }}{{range .GoFiles}}{{$$dep.Dir}}/{{.}} {{end}}{{end}}' 2>/dev/null)

AGENT_DEPS   := $(call godeps,./agent)
BOOTSTRAP_DEPS := $(call godeps,./bootstrap)
SERVICE_DEPS := $(call godeps,./service)

GIT_VERSION :=$(shell git describe --always --long --dirty)
GIT_HASH :=$(shell git rev-parse HEAD)
IMAGE_TAG:=$(shell ./docker/image-tag)
BASE_TAG?=${IMAGE_TAG}

# Placeholder for build flags. Previously was -i but this is deprecated
# and no longer needed with mod etc.
BUILDFLAGS:=
LDFLAGS:=-ldflags "-X github.com/weaveworks/launcher/pkg/version.Version=$(GIT_VERSION) -X github.com/weaveworks/launcher/pkg/version.Revision=$(GIT_HASH)"

# The actual binaries are compiled as part of their dockerfiles
all: base agent service
base: build/.base.Dockerfile.done
agent: base build/.agent.Dockerfile.done
service: base build/.service.Dockerfile.done

docker/Dockerfile.service: docker/Dockerfile.service.in Makefile
	@echo Generating $@
	@sed -e 's/@@GIT_HASH@@/$(GIT_HASH)/g' < $< > $@.tmp && mv $@.tmp $@

build/.%.Dockerfile.done: docker/Dockerfile.%
	mkdir -p ./build/docker/$*
	cp -r $^ ./build/docker/$*/
	${DOCKER} build --build-arg=revision=$(GIT_HASH) \
									--build-arg=base_tag=$(BASE_TAG) \
									-t weaveworks/launcher-$* \
								  -t weaveworks/launcher-$*:$(IMAGE_TAG) \
								  -f build/docker/$*/Dockerfile.$* \
								  .
	touch $@

#
# Vendoring
#
dep: build/dep.done
build/dep.done:
	go mod download
	mkdir -p ./build
	touch $@

#
# lint
#
lint:
	@./scripts/go-lint.sh

#
# Agent
#

build/.agent.done: dep build/agent build/kubectl

build/agent: $(AGENT_DEPS)
build/agent: agent/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(BUILDFLAGS) -o $@ $(LDFLAGS) ./agent

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

build/.bootstrap.done: dep $(BOOTSTRAP_DEPS)
build/.bootstrap.done: bootstrap/*.go
	for arch in amd64; do \
		for os in linux darwin; do \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build $(BUILDFLAGS) -o "build/bootstrap/bootstrap_"$$os"_"$$arch $(LDFLAGS) ./bootstrap; \
		done; \
	done
	touch $@

#
# Service
#

build/.service.done: dep build/service build/static

build/service: $(SERVICE_DEPS)
build/service: service/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(BUILDFLAGS) -o $@ $(LDFLAGS) ./service

# If we are not in CircleCI, we are local so use launcher-agent
# If we are in CircleCI, only use launcher-agent if we are building main, otherwise
# use build-tmp-public so we can run integration tests.
service/static/agent.yaml: service/static/agent.yaml.in
	@echo Generating $@
	if [ -z "$${CIRCLECI}" -o \( -z "$${CIRCLE_TAG}" -a "$${CIRCLE_BRANCH}" = "main" \) ]; then \
		sed -e 's|@@IMAGE_URL@@|weaveworks/launcher-agent:$(IMAGE_TAG)|g' < $< > $@.tmp && mv $@.tmp $@; \
	else \
		sed -e 's|@@IMAGE_URL@@|weaveworks/build-tmp-public:launcher-agent-$(IMAGE_TAG)|g' < $< > $@.tmp && mv $@.tmp $@; \
	fi

build/static: service/static/* service/static/agent.yaml
	mkdir -p $@
	cp $^ $@


#
# Local integration tests
#

integration-tests: all
	./integration-tests/setup/reset-local-cluster.sh
	./integration-tests/setup/setup-local-cluster.sh
	./integration-tests/tests/install-update-flow.sh

clean:
	rm -rf build cache vendor
	rm -f docker/Dockerfile.service service/static/agent.yaml
