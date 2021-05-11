.PHONY: all image push

BUILD_ARGS_RHEL82 = --build-arg CENTOS_VER=docker.io/centos:8.2.2004
BUILD_ARGS_RHEL83 = --build-arg CENTOS_VER=docker.io/centos:8.3.2011
RHEL_VER ?= rhel82

BUILD_ARGS := $(BUILD_ARGS_RHEL82)
ifeq ($(RHEL_VER),rhel83)
BUILD_ARGS := $(BUILD_ARGS_RHEL83)
endif

BUILDTOOL?= podman
IMAGE_NAME := opae
REPOS ?= quay.io/ryan_raasch
VER ?= v2.0.0

IMAGE := $(REPOS)/$(IMAGE_NAME):$(VER)

all: image push

image:
	$(BUILDTOOL) build . $(BUILD_ARGS) -t $(IMAGE)

push: image
	$(BUILDTOOL) push $(IMAGE)

