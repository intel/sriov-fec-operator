# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020-2023 Intel Corporation

# Default k8s command-line tool exec
export CLI_EXEC?=oc
# Container format for podman. Required to build containers with "ManifestType": "application/vnd.oci.image.manifest.v2+json",
export BUILDAH_FORMAT=docker
# Current Operator version
VERSION ?= 2.6.1
# Supported channels
CHANNELS ?= stable
# Default channel
DEFAULT_CHANNEL ?= stable

# Operator image registry
IMAGE_REGISTRY ?= registry.connect.redhat.com/intel
CONTAINER_TOOL ?= podman

FUZZ_TIME?=10m

# Add suffix directly to IMAGE_REGISTRY to enable empty registry(local images)
ifneq ($(and $(strip $(IMAGE_REGISTRY)), $(filter-out %/, $(IMAGE_REGISTRY))),)
override IMAGE_REGISTRY:=$(addsuffix /,$(IMAGE_REGISTRY))
endif
# tls verify flag for pushing images
TLS_VERIFY ?= false

REQUIRED_OPERATOR_SDK_VERSION ?= v1.25.2

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

ifeq ($(CONTAINER_TOOL),podman)
 export SRIOV_FEC_NETWORK_DEVICE_PLUGIN_IMAGE ?= registry.redhat.io/openshift4/ose-sriov-network-device-plugin:v4.11
 export KUBE_RBAC_PROXY_IMAGE ?= registry.redhat.io/openshift4/ose-kube-rbac-proxy@sha256:e3dad360d0351237a16593ca0862652809c41a2127c2f98b9e0a559568efbd10
else
 export SRIOV_FEC_NETWORK_DEVICE_PLUGIN_IMAGE ?= quay.io/openshift/origin-sriov-network-device-plugin:4.12
 export KUBE_RBAC_PROXY_IMAGE ?= gcr.io/kubebuilder/kube-rbac-proxy:v0.13.1
endif

CONTROLLER_TOOLS_VERSION ?= v0.9.2
ENVTEST_K8S_VERSION ?= 1.24
KUSTOMIZE_VERSION ?= 4.5.7
OPM_VERSION ?= 1.26.2
KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"

LOCALBIN ?= $(shell pwd)/bin$(REQUIRED_OPERATOR_SDK_VERSION)
## Tool Binaries
OPM ?= $(LOCALBIN)/opm
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

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

.PHONY: all
all: manager daemon labeler

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	test -s $(LOCALBIN)/kustomize || { curl -s $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }


.PHONY: opm
opm: $(OPM) ## Download opm locally if necessary.
$(OPM): $(LOCALBIN)
	test -s $(LOCALBIN)/opm || curl https://github.com/operator-framework/operator-registry/releases/download/v$(OPM_VERSION)/linux-amd64-opm -Lo $(LOCALBIN)/opm && chmod +x $(LOCALBIN)/opm

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" SRIOV_FEC_NAMESPACE=default go test ./... -coverprofile cover.out

TEST_PACKAGES := $(shell find . -name "*_test.go")

.PHONY: fuzz
fuzz:
	@for pkg in ${TEST_PACKAGES} ; do            \
		for target in `grep -oh -EI 'Fuzz([A-Z][a-z]*)+' $$pkg` ; do     \
			echo "Executing $$pkg#$$target";     \
			KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" SRIOV_FEC_NAMESPACE=default go test -fuzz=$$target $$pkg/.. -fuzztime=${FUZZ_TIME} || true; \
		done                              \
	done

# Build manager binary
.PHONY: manager
manager: generate fmt vet
	go build -race -o bin/manager main.go

#Build daemon binary
.PHONY: daemon
daemon: generate fmt vet
	go build -race -o bin/daemon cmd/daemon/main.go

#Build labeler binary
.PHONY: labeler
labeler: generate fmt vet
	go build -race -o bin/labeler cmd/labeler/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
run: generate fmt vet manifests
	go run ./main.go

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

# Install CRDs into a cluster
.PHONY: install
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | $(CLI_EXEC) apply -f -

# Uninstall CRDs from a cluster
.PHONY: uninstall
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | $(CLI_EXEC) delete --ignore-not-found=$(ignore-not-found) -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
.PHONY: deploy
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image sriov-fec-operator=$(SRIOV_FEC_OPERATOR_IMAGE)
	$(KUSTOMIZE) build config/default | envsubst | $(CLI_EXEC) apply -f -

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	FOLDER=. COPYRIGHT_FILE=COPYRIGHT ./copyright.sh

# Run go fmt against code
.PHONY: fmt
fmt:
	go fmt ./...

# Run go vet against code
.PHONY: vet
vet:
	go vet ./...

# Generate code
.PHONY: generate
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build/Push daemon image
.PHONY: image-sriov-fec-daemon
image-sriov-fec-daemon:
	cp LICENSE TEMP_LICENSE_COPY
	$(CONTAINER_TOOL) build . -f Dockerfile.daemon -t $(SRIOV_FEC_DAEMON_IMAGE) --build-arg=VERSION=$(IMG_VERSION)
	$(CONTAINER_TOOL) tag $(SRIOV_FEC_DAEMON_IMAGE) ghcr.io/intel-collab/sriov-fec-daemon:$(VERSION)
	
.PHONY: push-sriov-fec-daemon
podman-push-sriov-fec-daemon:
	podman push $(SRIOV_FEC_DAEMON_IMAGE) --tls-verify=$(TLS_VERIFY)

.PHONY: docker-push-sriov-fec-daemon
docker-push-sriov-fec-daemon:
	docker push $(SRIOV_FEC_DAEMON_IMAGE)

# Build/Push labeler image
.PHONY: image-sriov-fec-labeler
image-sriov-fec-labeler:
	cp LICENSE TEMP_LICENSE_COPY
	$(CONTAINER_TOOL) build . -f Dockerfile.labeler -t ${SRIOV_FEC_LABELER_IMAGE} --build-arg=VERSION=$(IMG_VERSION)
	$(CONTAINER_TOOL) tag $(SRIOV_FEC_LABELER_IMAGE) ghcr.io/intel-collab/sriov-fec-labeler:$(VERSION)

.PHONY: push-sriov-fec-labeler
podman-push-sriov-fec-labeler:
	podman push ${SRIOV_FEC_LABELER_IMAGE} --tls-verify=$(TLS_VERIFY)

.PHONY: docker-push-sriov-fec-labeler
docker-push-sriov-fec-labeler:
	docker push ${SRIOV_FEC_LABELER_IMAGE}

# Build/Push operator image
.PHONY: image-sriov-fec-operator
image-sriov-fec-operator:
	cp LICENSE TEMP_LICENSE_COPY
	$(CONTAINER_TOOL) build . -t $(SRIOV_FEC_OPERATOR_IMAGE) --build-arg=VERSION=$(IMG_VERSION)
	$(CONTAINER_TOOL) tag $(SRIOV_FEC_OPERATOR_IMAGE) ghcr.io/intel-collab/sriov-fec-operator:$(VERSION)

.PHONY: podman-push-sriov-fec-operator
podman-push-sriov-fec-operator:
	podman push $(SRIOV_FEC_OPERATOR_IMAGE) --tls-verify=$(TLS_VERIFY)

.PHONY: docker-push-sriov-fec-operator
docker-push-sriov-fec-operator:
	docker push $(SRIOV_FEC_OPERATOR_IMAGE)

# Build all the images
.PHONY: image
image: image-sriov-fec-daemon image-sriov-fec-labeler image-sriov-fec-operator image-bundle

# Push all the images
.PHONY: push
push: $(CONTAINER_TOOL)-push-sriov-fec-daemon $(CONTAINER_TOOL)-push-sriov-fec-labeler $(CONTAINER_TOOL)-push-sriov-fec-operator $(CONTAINER_TOOL)-push-bundle

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: check-operator-sdk-version manifests kustomize
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image sriov-fec-operator=$(SRIOV_FEC_OPERATOR_IMAGE)
	$(KUSTOMIZE) build config/manifests | envsubst | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle
	FOLDER=. COPYRIGHT_FILE=COPYRIGHT ./copyright.sh
	cat COPYRIGHT bundle.Dockerfile >bundle.tmp
	printf "\nLABEL com.redhat.openshift.versions=\"=v4.10-v4.12\"\n" >> bundle.tmp
	printf "\nCOPY TEMP_LICENSE_COPY /licenses/LICENSE\n" >> bundle.tmp
	mv bundle.tmp bundle.Dockerfile

# Build/Push the bundle image.
.PHONY: image-bundle
image-bundle: bundle
	cp LICENSE TEMP_LICENSE_COPY
	$(CONTAINER_TOOL) build -f bundle.Dockerfile -t $(BUNDLE_IMG) .
	$(CONTAINER_TOOL) tag $(BUNDLE_IMG) ghcr.io/intel-collab/sriov-fec-bundle:$(VERSION)

.PHONY: podman-push-bundle
podman-push-bundle:
	podman push $(BUNDLE_IMG) --tls-verify=$(TLS_VERIFY)
	
# push images into github container registry in Intel-Collab
.PHONY: push-sriov-fec-daemon-ghcr
push-sriov-fec-daemon-ghcr:
	$(CONTAINER_TOOL) push ghcr.io/intel-collab/sriov-fec-daemon:$(VERSION)  

.PHONY: push-sriov-fec-operator-ghcr
push-sriov-fec-operator-ghcr:
	$(CONTAINER_TOOL) push ghcr.io/intel-collab/sriov-fec-operator:$(VERSION)

.PHONY: push-sriov-fec-bundle-ghcr
push-sriov-fec-bundle-ghcr:
	$(CONTAINER_TOOL) push ghcr.io/intel-collab/sriov-fec-bundle:$(VERSION)

.PHONY: push-sriov-fec-labeler-ghcr
push-sriov-fec-labeler-ghcr:
	$(CONTAINER_TOOL) push ghcr.io/intel-collab/sriov-fec-labeler:$(VERSION)

.PHONY: push-all-ghcr
push-all-ghcr: push-sriov-fec-daemon-ghcr push-sriov-fec-operator-ghcr push-sriov-fec-bundle-ghcr push-sriov-fec-labeler-ghcr

# pull images from github container registry
.PHONY: pull-sriov-fec-daemon-ghcr
pull-sriov-fec-daemon-ghcr:
	docker pull ghcr.io/intel-collab/sriov-fec-daemon:$(VERSION)  

.PHONY: pull-sriov-fec-operator-ghcr
pull-sriov-fec-operator-ghcr:
	docker pull ghcr.io/intel-collab/sriov-fec-operator:$(VERSION)

.PHONY: pull-bundle-ghcr	
pull-bundle-ghcr:
	docker pull ghcr.io/intel-collab/sriov-fec-bundle:$(VERSION)

.PHONY: pull-labeler-ghcr
pull-labeler-ghcr:
	docker pull ghcr.io/intel-collab/sriov-fec-labeler:$(VERSION)

.PHONY: pull-all-ghcr     
pull-all-ghcr: pull-sriov-fec-daemon-ghcr pull-sriov-fec-operator-ghcr pull-bundle-ghcr pull-labeler-ghcr

.PHONY: scan-sriov-fec-daemon-image
scan-sriov-fec-daemon-image:
	snyk container test ghcr.io/intel-collab/sriov-fec-daemon:$(VERSION) || true
	snyk container monitor --exclude-base-image-vulns ghcr.io/intel-collab/sriov-fec-daemon:$(VERSION)

.PHONY: scan-sriov-fec-operator-image
scan-sriov-fec-operator-image:i
	snyk container test ghcr.io/intel-collab/sriov-fec-operator:$(VERSION) || true
	snyk container monitor --exclude-base-image-vulns ghcr.io/intel-collab/sriov-fec-operator:$(VERSION)

.PHONY: scan-sriov-fec-bundle-image
scan-sriov-fec-bundle-image:
	snyk container test ghcr.io/intel-collab/sriov-fec-bundle:$(VERSION) || true
	snyk container monitor --exclude-base-image-vulns ghcr.io/intel-collab/sriov-fec-bundle:$(VERSION)

.PHONY: scan-sriov-fec-labeler-image
scan-sriov-fec-labeler-image:
	snyk container test ghcr.io/intel-collab/sriov-fec-labeler:$(VERSION) || true
	snyk container monitor --exclude-base-image-vulns ghcr.io/intel-collab/sriov-fec-labeler:$(VERSION)

.PHONY: scan_all
scan_all: scan-sriov-fec-daemon-image scan-sriov-fec-operator-image scan-sriov-fec-bundle-image scan-sriov-fec-labeler-image

.PHONY: docker-push-bundle
docker-push-bundle:
	docker push $(BUNDLE_IMG)
.PHONY: build
build: image push

.PHONY: build_all
build_all: build
	$(MAKE) VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) build_index

.PHONY: build_index
build_index: opm
	rm -fr sriov-fec-index
	mkdir sriov-fec-index
	$(OPM) init sriov-fec --default-channel=stable --output yaml > sriov-fec-index/index.yaml
ifeq ($(TLS_VERIFY), false)
	$(OPM) render $(BUNDLE_IMG) --output=yaml --skip-tls-verify >> sriov-fec-index/index.yaml
else
	$(OPM) render $(BUNDLE_IMG) --output=yaml >> sriov-fec-index/index.yaml
endif
	echo -e "---\nschema: olm.channel\npackage: sriov-fec\nname: stable\nentries:\n- name: sriov-fec.v$(VERSION)" >> sriov-fec-index/index.yaml
	$(OPM) validate sriov-fec-index
	$(CONTAINER_TOOL) build . -f sriov-fec-index.Dockerfile -t localhost/sriov-fec-index:$(VERSION)
	$(MAKE) VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) $(CONTAINER_TOOL)_push_index

.PHONY: podman_push_index
podman_push_index:
	podman push localhost/sriov-fec-index:$(VERSION) $(IMAGE_REGISTRY)sriov-fec-index:$(VERSION) --tls-verify=$(TLS_VERIFY)

.PHONY: docker_push_index
docker_push_index:
	docker tag localhost/sriov-fec-index:$(VERSION) $(IMAGE_REGISTRY)sriov-fec-index:$(VERSION)
	docker push $(IMAGE_REGISTRY)sriov-fec-index:$(VERSION)

.PHONY: install_operator_sdk
install_operator_sdk:
	curl -LO https://github.com/operator-framework/operator-sdk/releases/download/$(REQUIRED_OPERATOR_SDK_VERSION)/operator-sdk_linux_amd64
	chmod +x operator-sdk_linux_amd64 && sudo mv operator-sdk_linux_amd64 /usr/bin/operator-sdk

OPERATOR_SDK_INSTALLED := $(shell command -v operator-sdk version 2> /dev/null)
.PHONY: check-operator-sdk-version
check-operator-sdk-version:
ifndef OPERATOR_SDK_INSTALLED
	$(info operator-sdk is not installed - downloading it)
	$(MAKE) REQUIRED_OPERATOR_SDK_VERSION=$(REQUIRED_OPERATOR_SDK_VERSION) install_operator_sdk
else
ifneq ($(shell operator-sdk version | awk -F',' '{print $$1}' | awk -F'[""]' '{print $$2}'), $(REQUIRED_OPERATOR_SDK_VERSION))
	$(info updating operator-sdk to $(REQUIRED_OPERATOR_SDK_VERSION))
	$(MAKE) REQUIRED_OPERATOR_SDK_VERSION=$(REQUIRED_OPERATOR_SDK_VERSION) install_operator_sdk
endif
endif
