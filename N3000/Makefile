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

OPERATOR_SDK = ../bin/operator-sdk

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"

all: manager daemon

.PHONY: push-driver image-daemon push-daemon image-operator push-operator image

# Run tests
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: manifests generate fmt vet ## Run tests.
	mkdir -p $(ENVTEST_ASSETS_DIR)
	test -f $(ENVTEST_ASSETS_DIR)/setup-envtest.sh || curl -sSLo $(ENVTEST_ASSETS_DIR)/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.7.0/hack/setup-envtest.sh
	source $(ENVTEST_ASSETS_DIR)/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); NAMESPACE=default go test ./... -coverprofile cover.out

test-cov: generate fmt vet manifests
	mkdir -p $(ENVTEST_ASSETS_DIR)
	test -f $(ENVTEST_ASSETS_DIR)/setup-envtest.sh || curl -sSLo $(ENVTEST_ASSETS_DIR)/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.7.0/hack/setup-envtest.sh
	source $(ENVTEST_ASSETS_DIR)/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); NAMESPACE=default ginkgo -v -r -cover -coverprofile=coverage.out -outputdir=.
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
	cd config/manager && $(KUSTOMIZE) edit set image operator=$(N3000_OPERATOR_IMAGE)
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

# find or download controller-gen
# download controller-gen if necessary
CONTROLLER_GEN = $(shell pwd)/../bin/controller-gen
controller-gen:
	make -C .. check-operator-sdk-version

KUSTOMIZE = $(shell pwd)/../bin/kustomize
kustomize:
	make -C .. check-operator-sdk-version

image-daemon:
	cp ../LICENSE TEMP_LICENSE_COPY
	$(PODMAN) build . -f Dockerfile.daemon -t $(DAEMON_IMAGE) $(CONTAINER_BUILD_ARGS)

push-daemon:
	$(PODMAN) push $(DAEMON_IMAGE) $(CONTAINER_PUSH_ARGS)

push-labeler:
image-labeler:
	make -C ../labeler $(subst -labeler,,$@)

push-prometheus_fpgainfo_exporter:
image-prometheus_fpgainfo_exporter:
	make -C ../prometheus_fpgainfo_exporter $(subst -prometheus_fpgainfo_exporter,,$@)

image-operator:
	cp ../LICENSE TEMP_LICENSE_COPY
	$(PODMAN) build . -t $(OPERATOR_IMAGE) $(CONTAINER_BUILD_ARGS)

push-operator:
	$(PODMAN) push $(OPERATOR_IMAGE) $(CONTAINER_PUSH_ARGS)

# Build all the images
image: image-daemon image-bundle image-operator image-labeler image-bundle image-prometheus_fpgainfo_exporter
image-bundle: image-daemon image-operator image-labeler image-prometheus_fpgainfo_exporter

# Push all the images
.PHONY: push
push: push-daemon push-operator push-labeler push-bundle push-fpgainfo_exporter

check-operator-sdk-version:
	make -C .. check-operator-sdk-version

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: image check-operator-sdk-version manifests kustomize
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image operator=$(OPERATOR_IMAGE)
	$(KUSTOMIZE) build config/manifests | envsubst | $(OPERATOR_SDK) generate bundle -q --overwrite --version $(subst v,,$(VERSION)) $(BUNDLE_METADATA_OPTS)
	$(OPERATOR_SDK) bundle validate ./bundle
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
