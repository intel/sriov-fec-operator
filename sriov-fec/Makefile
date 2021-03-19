# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020 Intel Corporation

# Default k8s command-line tool exec
export CLI_EXEC?=oc
# Current Operator version
VERSION ?= 1.1.0
# Supported channels
CHANNELS ?= stable
# Default channel
DEFAULT_CHANNEL ?= stable

# Operator image registry
IMAGE_REGISTRY ?= registry.connect.redhat.com/intel
# Add suffix directly to IMAGE_REGISTRY to enable empty registry(local images)
ifneq ($(strip $(IMAGE_REGISTRY)),)
override IMAGE_REGISTRY:=$(addsuffix /,$(IMAGE_REGISTRY))
endif
# tls verify flag for pushing images
TLS_VERIFY ?= true

# Default bundle image tag
BUNDLE_IMG ?= $(IMAGE_REGISTRY)sriov-fec-bundle:$(VERSION)
# Options for 'image-bundle'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

IMG_VERSION := v$(VERSION)

# Images URLs to use for all building/pushing image targets
export SRIOV_FEC_OPERATOR_IMAGE ?= $(IMAGE_REGISTRY)sriov-fec-operator:$(IMG_VERSION)
export SRIOV_FEC_DAEMON_IMAGE ?= $(IMAGE_REGISTRY)sriov-fec-daemon:$(IMG_VERSION)
export SRIOV_FEC_LABELER_IMAGE ?= $(IMAGE_REGISTRY)n3000-labeler:$(IMG_VERSION)

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager daemon

# Run tests
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: manifests generate fmt vet ## Run tests.
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.7.0/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); SRIOV_FEC_NAMESPACE=default go test ./... -coverprofile cover.out

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
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

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
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1)

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v3@v3.8.7)

# Build/Push daemon image
.PHONY: image-sriov-fec-daemon
image-sriov-fec-daemon:
	cp ../LICENSE TEMP_LICENSE_COPY
	podman build . -f Dockerfile.daemon -t $(SRIOV_FEC_DAEMON_IMAGE) --build-arg=VERSION=$(IMG_VERSION)

.PHONY: push-sriov-fec-daemon
push-sriov-fec-daemon:
	podman push $(SRIOV_FEC_DAEMON_IMAGE) --tls-verify=$(TLS_VERIFY)

# Build/Push operator image
.PHONY: image-sriov-fec-operator
image-sriov-fec-operator:
	cp ../LICENSE TEMP_LICENSE_COPY
	podman build . -t $(SRIOV_FEC_OPERATOR_IMAGE) --build-arg=VERSION=$(IMG_VERSION)

.PHONY: push-sriov-fec-operator
push-sriov-fec-operator:
	podman push $(SRIOV_FEC_OPERATOR_IMAGE) --tls-verify=$(TLS_VERIFY)

# Build all the images
.PHONY: image
image: image-sriov-fec-daemon image-sriov-fec-operator image-bundle

# Push all the images
.PHONY: push
push: push-sriov-fec-daemon push-sriov-fec-operator push-bundle

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
	printf "\nLABEL com.redhat.openshift.versions=\"=v4.6\"\n" >> bundle.tmp
	printf "\nCOPY TEMP_LICENSE_COPY /licenses/LICENSE\n" >> bundle.tmp
	mv bundle.tmp bundle.Dockerfile

# Build/Push the bundle image.
.PHONY: image-bundle
image-bundle: bundle
	podman build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: push-bundle
push-bundle:
	podman push $(BUNDLE_IMG) --tls-verify=$(TLS_VERIFY)

build_all: image push
