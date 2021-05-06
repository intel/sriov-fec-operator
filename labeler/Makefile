# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020 Intel Corporation

include ../makefile.top

# Build labeler binary
labeler: fmt vet
	go build -o bin/labeler ./main.go

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Build the docker image
image:
	cp ../LICENSE TEMP_LICENSE_COPY
	$(PODMAN) build . -t $(N3000_LABELER_IMAGE)  $(CONTAINER_BUILD_ARGS)

# Push the docker image
push:
	$(PODMAN) push $(N3000_LABELER_IMAGE)  $(CONTAINER_PUSH_ARGS)

# Run tests
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: fmt vet
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/master/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); NAMESPACE=default go test ./... -coverprofile cover.out

build_all: image push
