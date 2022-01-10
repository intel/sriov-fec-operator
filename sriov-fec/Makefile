# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020 Intel Corporation

# Default k8s command-line tool exec
export CLI_EXEC?=oc
# Current Operator version
VERSION ?= 2.1.0
# Supported channels
CHANNELS ?= stable
# Default channel
DEFAULT_CHANNEL ?= stable

# Operator image registry
IMAGE_REGISTRY ?= registry.connect.redhat.com/intel
ifeq (,$(shell which kubectl))
 CONTAINER_TOOL ?= podman
else
 CONTAINER_TOOL ?= docker
endif

# Add suffix directly to IMAGE_REGISTRY to enable empty registry(local images)
ifneq ($(strip $(IMAGE_REGISTRY)),)
override IMAGE_REGISTRY:=$(addsuffix /,$(IMAGE_REGISTRY))
endif
# tls verify flag for pushing images
TLS_VERIFY ?= true

IMAGE_TAG_BASE ?= $(IMAGE_REGISTRY)sriov-fec
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# Options for 'image-bundle'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

IMG_VERSION := v$(VERSION)

OS = $(shell go env GOOS)
ARCH = $(shell go env GOARCH)

# Images URLs to use for all building/pushing image targets
export SRIOV_FEC_OPERATOR_IMAGE ?= $(IMAGE_REGISTRY)sriov-fec-operator:$(IMG_VERSION)
export SRIOV_FEC_DAEMON_IMAGE ?= $(IMAGE_REGISTRY)sriov-fec-daemon:$(IMG_VERSION)
export SRIOV_FEC_LABELER_IMAGE ?= $(IMAGE_REGISTRY)n3000-labeler:$(IMG_VERSION)

ifeq (,$(shell which kubectl))
 export SRIOV_FEC_NETWORK_DEVICE_PLUGIN_IMAGE ?= registry.redhat.io/openshift4/ose-sriov-network-device-plugin:v4.9
 export KUBE_RBAC_PROXY_IMAGE ?= registry.redhat.io/openshift4/ose-kube-rbac-proxy@sha256:86e5fa1fa294987114be200890c2e516501e424aee0fb98ece25c95e7716295b
else
 export SRIOV_FEC_NETWORK_DEVICE_PLUGIN_IMAGE ?= quay.io/openshift/origin-sriov-network-device-plugin:4.9.0
 export KUBE_RBAC_PROXY_IMAGE ?= gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0
endif

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

all: manager daemon

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

test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" SRIOV_FEC_NAMESPACE=default go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

#Build daemon binary
daemon: generate fmt vet
	go build -o bin/daemon cmd/daemon/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | $(CLI_EXEC) apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | $(CLI_EXEC) delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image sriov-fec-operator=$(SRIOV_FEC_OPERATOR_IMAGE)
	$(KUSTOMIZE) build config/default | envsubst | $(CLI_EXEC) apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	FOLDER=. COPYRIGHT_FILE=../COPYRIGHT ../copyright.sh

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

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

# find or download controller-gen
# download controller-gen if necessary
CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0)

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v3@v3.8.7)

# Build/Push daemon image
.PHONY: image-sriov-fec-daemon
image-sriov-fec-daemon:
	cp ../LICENSE TEMP_LICENSE_COPY
	$(CONTAINER_TOOL) build . -f Dockerfile.daemon -t $(SRIOV_FEC_DAEMON_IMAGE) --build-arg=VERSION=$(IMG_VERSION)

.PHONY: push-sriov-fec-daemon
podman-push-sriov-fec-daemon:
	podman push $(SRIOV_FEC_DAEMON_IMAGE) --tls-verify=$(TLS_VERIFY)

.PHONY: docker-push-sriov-fec-daemon
docker-push-sriov-fec-daemon:
	docker push $(SRIOV_FEC_DAEMON_IMAGE)

# Build/Push operator image
.PHONY: image-sriov-fec-operator
image-sriov-fec-operator:
	cp ../LICENSE TEMP_LICENSE_COPY
	$(CONTAINER_TOOL) build . -t $(SRIOV_FEC_OPERATOR_IMAGE) --build-arg=VERSION=$(IMG_VERSION)

.PHONY: podman-push-sriov-fec-operator
podman-push-sriov-fec-operator:
	podman push $(SRIOV_FEC_OPERATOR_IMAGE) --tls-verify=$(TLS_VERIFY)

.PHONY: docker-push-sriov-fec-operator
docker-push-sriov-fec-operator:
	docker push $(SRIOV_FEC_OPERATOR_IMAGE)

# Build all the images
.PHONY: image
image: image-sriov-fec-daemon image-sriov-fec-operator image-bundle

# Push all the images
.PHONY: push
push: $(CONTAINER_TOOL)-push-sriov-fec-daemon $(CONTAINER_TOOL)-push-sriov-fec-operator $(CONTAINER_TOOL)-push-bundle

check-operator-sdk-version:
	(cd .. && make check-operator-sdk-version)

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: check-operator-sdk-version manifests kustomize
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image sriov-fec-operator=$(SRIOV_FEC_OPERATOR_IMAGE)
	$(KUSTOMIZE) build config/manifests | envsubst | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle
	FOLDER=. COPYRIGHT_FILE=../COPYRIGHT ../copyright.sh
	cat ../COPYRIGHT bundle.Dockerfile >bundle.tmp
	printf "\nLABEL com.redhat.openshift.versions=\"=v4.9\"\n" >> bundle.tmp
	printf "\nCOPY TEMP_LICENSE_COPY /licenses/LICENSE\n" >> bundle.tmp
	mv bundle.tmp bundle.Dockerfile

# Build/Push the bundle image.
.PHONY: image-bundle
image-bundle: bundle
	$(CONTAINER_TOOL) build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: podman-push-bundle
podman-push-bundle:
	podman push $(BUNDLE_IMG) --tls-verify=$(TLS_VERIFY)

.PHONY: docker-push-bundle
docker-push-bundle:
	docker push $(BUNDLE_IMG)

build_all: image push

## Push image with the manager.
podman-image-push:
	podman push ${IMG} --tls-verify=$(TLS_VERIFY)

docker-image-push:
	docker push ${IMG}

.PHONY: opm
OPM = ./bin/opm
opm:
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.15.1/$(OS)-$(ARCH)-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif
BUNDLE_IMGS ?= $(BUNDLE_IMG)
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION) ifneq ($(origin CATALOG_BASE_IMG), undefined) FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG) endif
.PHONY: catalog-build
catalog-build: opm
	$(OPM) index add --container-tool $(CONTAINER_TOOL) --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

.PHONY: catalog-push
catalog-push: ## Push the catalog image.
	$(MAKE) $(CONTAINER_TOOL)-image-push IMG=$(CATALOG_IMG)
