# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2021 Intel Corporation

# Current exporter  version
VERSION ?= 1.0.0
# Current OPAE package version
OPAE_VERSION ?= 1.3.8-2
# Image registry
IMAGE_REGISTRY ?= registry.connect.redhat.com/intel

# Add suffix directly to IMAGE_REGISTRY to enable empty registry(local images)
ifneq ($(strip $(IMAGE_REGISTRY)),)
override IMAGE_REGISTRY:=$(addsuffix /,$(IMAGE_REGISTRY))
endif

# tls verify flag for pushing images
TLS_VERIFY ?= true

IMG_VERSION := v$(VERSION)

# Images URLs to use for all building/pushing image targets
export N3000_MONITORING_IMAGE ?= $(IMAGE_REGISTRY)n3000-monitoring:$(IMG_VERSION)--$(OPAE_VERSION)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

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
.PHONY: image
image:
	cp ../LICENSE TEMP_LICENSE_COPY
	podman build . -t $(N3000_MONITORING_IMAGE) --build-arg=OPAE_VERSION=$(OPAE_VERSION) --build-arg=VERSION=$(IMG_VERSION)

.PHONY: push-image
push-image:
	podman push $(N3000_MONITORING_IMAGE) --tls-verify=$(TLS_VERIFY)

build_all: image push-image
