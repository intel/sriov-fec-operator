# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020-2021 Intel Corporation

IMAGE_REGISTRY ?= registry.connect.redhat.com/intel
REQUIRED_OPERATOR_SDK_VERSION ?= v1.4.2
VERSION ?= 1.1.0
TLS_VERIFY ?= false

build_all:
	(cd sriov-fec && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) build_all)
	(cd N3000 && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) build_all)
	(cd labeler && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) build_all)
	(cd prometheus_fpgainfo_exporter && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) build_all)
	make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) build_index

build_index:
	$(PWD)/bin/opm index add --bundles $(IMAGE_REGISTRY)/sriov-fec-bundle:$(VERSION),$(IMAGE_REGISTRY)/n3000-bundle:$(VERSION) --tag localhost/n3000-operators-index:$(VERSION) $(if ifeq $(TLS_VERIFY) false, --skip-tls) -c podman --mode=semver
	podman push localhost/n3000-operators-index:$(VERSION) $(IMAGE_REGISTRY)/n3000-operators-index:$(VERSION) --tls-verify=$(TLS_VERIFY)

image:
	(cd sriov-fec && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) image)
	(cd N3000 && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) image)
	(cd labeler && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) image)
	(cd prometheus_fpgainfo_exporter && make VERSION=$(VERSION) IMAGE_REGISTRY=$(IMAGE_REGISTRY) TLS_VERIFY=$(TLS_VERIFY) image)

clean-tools:
	rm -rf downloads bin

OPERATOR_SDK_INSTALLED := $(shell command -v bin/operator-sdk version 2> /dev/null)
check-operator-sdk-version:
ifndef OPERATOR_SDK_INSTALLED
	$(info operator-sdk is not installed - downloading it)
	scripts/install-tools.sh $(REQUIRED_OPERATOR_SDK_VERSION)
else
ifneq ($(shell bin/operator-sdk version | awk -F',' '{print $$1}' | awk -F'[""]' '{print $$2}'), $(REQUIRED_OPERATOR_SDK_VERSION))
	$(info updating operator-sdk to $(REQUIRED_OPERATOR_SDK_VERSION))
	scripts/install-tools.sh $(REQUIRED_OPERATOR_SDK_VERSION)
endif
endif
