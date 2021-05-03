# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020-2021 Intel Corporation
include ../makefile.top

# Default k8s command-line tool exec
# Supported channels
CHANNELS ?= stable
# Default channel
DEFAULT_CHANNEL ?= stable
# Kernel version for driver container
KERNEL_VERSION ?= 4.18.0-193.41.1.el8_2.x86_64
# Where to get the kernel dependencies from
# Possible values:
#  yum - download the dependencies from upstream repositories
#  file - load the RPM dependencies from files/kernel
KERNEL_SOURCE ?= yum

# Options for 'image-bundle'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"

all: manager daemon

.PHONY: push-n3000-driver image-n3000-daemon push-n3000-daemon image-n3000-operator push-n3000-operator image

# Run tests
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: manifests generate fmt vet ## Run tests.
	mkdir -p $(ENVTEST_ASSETS_DIR)
	test -f $(ENVTEST_ASSETS_DIR)/setup-envtest.sh || curl -sSLo $(ENVTEST_ASSETS_DIR)/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.7.0/hack/setup-envtest.sh
	source $(ENVTEST_ASSETS_DIR)/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); N3000_NAMESPACE=default go test ./... -coverprofile cover.out

test-cov: generate fmt vet manifests
	mkdir -p $(ENVTEST_ASSETS_DIR)
	test -f $(ENVTEST_ASSETS_DIR)/setup-envtest.sh || curl -sSLo $(ENVTEST_ASSETS_DIR)/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.7.0/hack/setup-envtest.sh
	source $(ENVTEST_ASSETS_DIR)/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); N3000_NAMESPACE=default ginkgo -v -r -cover -coverprofile=coverage.out -outputdir=.
	sed '1!{/^mode/d;)' coverage.out > coverage.out.fix
	go tool cover -html=coverage.out.fix

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

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
	cd config/manager && $(KUSTOMIZE) edit set image n3000-operator=$(N3000_OPERATOR_IMAGE)
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

# Build opae image
.PHONY: image-opae-rte
image-opae-rte:
	cp -r files docker/opae-image
	cp ../LICENSE docker/opae-image/TEMP_LICENSE_COPY
	cd docker/opae-image && $(PODMAN) build . -t $(N3000_OPAE_IMAGE) $(CONTAINER_BUILD_ARGS)

# Build opae image
.PHONY: image-opae
image-opae:
	cp -r files docker/opae-image
	cp ../LICENSE docker/opae-image/TEMP_LICENSE_COPY
	cd docker/opae-image && $(PODMAN) build . $(CONTAINER_BUILD_ARGS) -t $(N3000_OPAE_IMAGE_RHEL) -f Dockerfile.centos

# Build/Push driver image
.PHONY: image-n3000-driver
image-n3000-driver:
ifeq ($(BUILD_DRIVER_CONTAINER),yes)
	cp -r files docker/driver-container
	cp ../LICENSE docker/driver-container/TEMP_LICENSE_COPY
	cd docker/driver-container && $(PODMAN) build . -t $(N3000_DRIVER_IMAGE) --build-arg=VERSION=$(IMG_VERSION) --build-arg=OPAE_VERSION=$(OPAE_VERSION) --build-arg=KERNEL_VERSION=$(KERNEL_VERSION) --build-arg=KERNEL_SOURCE=$(KERNEL_SOURCE)
endif

push-n3000-driver:
ifeq ($(BUILD_DRIVER_CONTAINER),yes)
	$(PODMAN) push $(N3000_DRIVER_IMAGE) $(CONTAINER_PUSH_ARGS)
endif

image-n3000-daemon: image-opae
	cp ../LICENSE TEMP_LICENSE_COPY
	$(PODMAN) build . -f Dockerfile.daemon -t $(N3000_DAEMON_IMAGE) $(CONTAINER_BUILD_ARGS)

push-n3000-daemon:
	$(PODMAN) push $(N3000_DAEMON_IMAGE) $(CONTAINER_PUSH_ARGS)

image-n3000-operator:
	cp ../LICENSE TEMP_LICENSE_COPY
	$(PODMAN) build . -t $(N3000_OPERATOR_IMAGE) $(CONTAINER_BUILD_ARGS)

push-n3000-operator:
	$(PODMAN) push $(N3000_OPERATOR_IMAGE) $(CONTAINER_PUSH_ARGS)

# Build all the images
image: image-n3000-driver image-n3000-daemon image-n3000-operator image-bundle

# Push all the images
.PHONY: push
push: push-n3000-driver push-n3000-daemon push-n3000-operator push-bundle

check-operator-sdk-version:
	make -C .. check-operator-sdk-version

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: check-operator-sdk-version manifests kustomize
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image n3000-operator=$(N3000_OPERATOR_IMAGE)
	$(KUSTOMIZE) build config/manifests | envsubst | operator-sdk generate bundle -q --overwrite --version $(subst v,,$(VERSION)) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle
	FOLDER=. COPYRIGHT_FILE=../COPYRIGHT ../copyright.sh
	cat ../COPYRIGHT bundle.Dockerfile >bundle.tmp
	printf "\nLABEL com.redhat.openshift.versions=\"=v4.6\"\n" >> bundle.tmp
	printf "\nCOPY TEMP_LICENSE_COPY /licenses/LICENSE\n" >> bundle.tmp
	mv bundle.tmp bundle.Dockerfile

# Build/Push the bundle image.
.PHONY: image-bundle
image-bundle: bundle
	$(PODMAN) build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: push-bundle
push-bundle:
	$(PODMAN) push $(BUNDLE_IMG) $(CONTAINER_PUSH_ARGS)

build_all: image push
