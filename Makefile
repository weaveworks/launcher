.PHONY: all clean dep lint agent bootstrap service
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

GIT_HASH :=$(shell git rev-parse HEAD)
IMAGE_TAG:=$(shell ./docker/image-tag)

# We can't install go packages on CircleCI without being root (or using sudo).
# Because the compilation is done only once there, it doesn't matter if we
# don't install the packages.
ifneq ($(CI),true)
INSTALL_FLAG := -i
endif
BUILDFLAGS   := $(INSTALL_FLAG)

all: dep agent bootstrap service
agent: build/.agent.done
bootstrap: build/.bootstrap.done
service: build/.service.done

docker/Dockerfile.service: docker/Dockerfile.service.in Makefile
	@echo Generating $@
	@sed -e 's/@@GIT_HASH@@/$(GIT_HASH)/g' < $< > $@.tmp && mv $@.tmp $@

build/.%.done: docker/Dockerfile.%
	mkdir -p ./build/docker/$*
	cp -r $^ ./build/docker/$*/
	${DOCKER} build -t quay.io/weaveworks/launcher-$* -t quay.io/weaveworks/launcher-$*:$(IMAGE_TAG) -f build/docker/$*/Dockerfile.$* ./build/docker/$*
	touch $@

#
# Vendoring
#
dep: build/dep.done
build/dep.done:
	go get -u github.com/golang/dep/cmd/dep
	dep ensure
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

build/.agent.done: build/agent build/kubectl

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

build/.bootstrap.done: $(BOOTSTRAP_DEPS)
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

build/.service.done: build/service build/static

build/service: $(SERVICE_DEPS)
build/service: service/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(BUILDFLAGS) -o $@ $(LDFLAGS) ./service

service/static/agent.yaml: service/static/agent.yaml.in
	@echo Generating $@
	# If we are not in CircleCI, we are local so use launcher-agent
	# If we are in CircleCI, only use launcher-agent if we are building master, otherwise
	# use build-tmp-public so we can run integration tests.
	if [ -z "$${CIRCLECI}" -o -z "$${CIRCLE_TAG}" -a "$${CIRCLE_BRANCH}" == "master" ]; then \
		sed -e 's|@@IMAGE_URL@@|quay.io/weaveworks/launcher-agent:$(IMAGE_TAG)|g' < $< > $@.tmp && mv $@.tmp $@; \
	else \
		sed -e 's|@@IMAGE_URL@@|quay.io/weaveworks/build-tmp-public:launcher-agent-$(IMAGE_TAG)|g' < $< > $@.tmp && mv $@.tmp $@; \
	fi

build/static: service/static/* service/static/agent.yaml
	mkdir -p $@
	cp $^ $@


#
# Local integration tests
#

integration-tests: all
	./integration-tests/reset-local-minikube.sh
	./integration-tests/setup-local-minikube.sh
	./integration-tests/run.sh

clean:
	rm -rf build cache vendor
	rm -f docker/Dockerfile.service service/static/agent.yaml
