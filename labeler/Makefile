# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020 Intel Corporation

VERSION ?= 2.1.0
IMG_VERSION := v$(VERSION)
TLS_VERIFY ?= true

# Operator image registry
IMAGE_REGISTRY ?= registry.connect.redhat.com/intel
CONTAINER_TOOL ?= podman
# Add suffix directly to IMAGE_REGISTRY to enable empty registry(local images)
ifneq ($(strip $(IMAGE_REGISTRY)),)
override IMAGE_REGISTRY:=$(addsuffix /,$(IMAGE_REGISTRY))
endif

export LABELER_IMAGE ?= $(IMAGE_REGISTRY)n3000-labeler:$(IMG_VERSION)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Build labeler binary
labeler: fmt vet
	go build -o bin/labeler ./main.go

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Build the image
image:
	cp ../LICENSE TEMP_LICENSE_COPY
	$(CONTAINER_TOOL) build . -t ${LABELER_IMAGE} --build-arg=VERSION=$(IMG_VERSION)

# Push the image
podman-push:
	podman push ${LABELER_IMAGE} --tls-verify=$(TLS_VERIFY)

docker-push:
	docker push ${LABELER_IMAGE}

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.22
ENVTEST = $(shell pwd)/bin/setup-envtest
envtest: ## Download envtest-setup locally if necessary.
	echo $(ENVTEST)
	$(call go-get-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

test: fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" NAMESPACE=default go test ./... -coverprofile cover.out

build_all: image $(CONTAINER_TOOL)-push
