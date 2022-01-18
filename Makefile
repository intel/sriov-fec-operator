# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020-2021 Intel Corporation

IMAGE_REGISTRY ?= registry.connect.redhat.com/intel
REQUIRED_OPERATOR_SDK_VERSION ?= v1.14.0
VERSION ?= 2.1.0
TLS_VERIFY ?= false
CONTAINER_TOOL ?= podman

build_all:
	(cd sriov-fec && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) build_all)
	(cd N3000 && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) build_all)
	(cd labeler && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) build_all)
	(cd prometheus_fpgainfo_exporter && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) build_all)
	make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) build_index

build_without_n3000:
	(cd sriov-fec && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) CONTAINER_TOOL=$(CONTAINER_TOOL) build_all)
	(cd labeler && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) CONTAINER_TOOL=$(CONTAINER_TOOL) build_all)
	make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) CONTAINER_TOOL=$(CONTAINER_TOOL) build_index

build_index:
	opm index add --bundles $(IMAGE_REGISTRY)/sriov-fec-bundle:v$(VERSION) --tag localhost/n3000-operators-index:$(VERSION) $(if ifeq $(TLS_VERIFY) false, --skip-tls) -c $(CONTAINER_TOOL) --mode=semver
	make TLS_VERIFY=$(TLS_VERIFY) $(CONTAINER_TOOL)_push_index

podman_push_index:
	podman push localhost/n3000-operators-index:$(VERSION) $(IMAGE_REGISTRY)/n3000-operators-index:$(VERSION) --tls-verify=$(TLS_VERIFY)

docker_push_index:
	docker tag localhost/n3000-operators-index:$(VERSION) $(IMAGE_REGISTRY)/n3000-operators-index:$(VERSION)  
	docker push $(IMAGE_REGISTRY)/n3000-operators-index:$(VERSION)

image:
	(cd sriov-fec && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) image)
	(cd N3000 && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) image)
	(cd labeler && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) image)
	(cd prometheus_fpgainfo_exporter && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) image)

install_operator_sdk:
	curl -LO https://github.com/operator-framework/operator-sdk/releases/download/$(REQUIRED_OPERATOR_SDK_VERSION)/operator-sdk_linux_amd64
	chmod +x operator-sdk_linux_amd64 && sudo mv operator-sdk_linux_amd64 /usr/bin/operator-sdk

OPERATOR_SDK_INSTALLED := $(shell command -v operator-sdk version 2> /dev/null)
check-operator-sdk-version:
ifndef OPERATOR_SDK_INSTALLED
	$(info operator-sdk is not installed - downloading it)
	make install_operator_sdk
else
ifneq ($(shell operator-sdk version | awk -F',' '{print $$1}' | awk -F'[""]' '{print $$2}'), $(REQUIRED_OPERATOR_SDK_VERSION))
	$(info updating operator-sdk to $(REQUIRED_OPERATOR_SDK_VERSION))
	make install_operator_sdk
endif
endif
