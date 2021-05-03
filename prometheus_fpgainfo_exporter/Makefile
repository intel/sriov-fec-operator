# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2021 Intel Corporation
include ../makefile.top

.PHONY: image push

all: exporter image

# Run tests
test: fmt vet
	go test ./... -coverprofile cover.out

test-cov: fmt vet
	ginkgo -v -r -cover -coverprofile=coverage.out -outputdir=.
	go tool cover -html=coverage.out

# Build fpgainfo exporter
exporter: fmt vet
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o fpgainfo_exporter main.go

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Build/push exporter image
image:
	cp ../LICENSE TEMP_LICENSE_COPY
	$(PODMAN) build . -t $(MONITORING_IMAGE) $(CONTAINER_BUILD_ARGS)

push: image
	$(PODMAN) push $(MONITORING_IMAGE) $(CONTAINER_PUSH_ARGS)

build_all: image push
