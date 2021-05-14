.PHONY: all image push

include ../../../makefile.top

KMODVER := eea9cbc
KVER_RHEL82 = 4.18.0-193.el8.x86_64
KVER_RHEL83 = 4.18.0-240.el8.x86_64
RHEL82_RPM := https://vault.centos.org/8.2.2004/BaseOS/x86_64/os/Packages/kernel-devel-4.18.0-193.el8.x86_64.rpm
RHEL83_RPM := http://mirror.centos.org/centos/8/BaseOS/x86_64/os/Packages/kernel-devel-4.18.0-240.el8.x86_64.rpm

BUILD_ARGS_RHEL82 = --build-arg CENTOS_VER=docker.io/centos:8.2.2004 --build-arg RPM_URL=$(RHEL82_RPM) --build-arg KMODVER=$(KMODVER) --build-arg KVER=$(KVER_RHEL82)
BUILD_ARGS_RHEL83 = --build-arg CENTOS_VER=docker.io/centos:8.3.2011 --build-arg RPM_URL=$(RHEL83_RPM) --build-arg KMODVER=$(KMODVER) --build-arg KVER=$(KVER_RHEL83)

BUILDTOOL ?= podman
IMAGE_NAME := dfl-kmod

CONTAINER_BUILD_ARGS += $(BUILD_ARGS_RHEL82)
TAG := $(KMODVER)-$(KVER_RHEL82)
IMAGE := $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(TAG)

ifeq ($(RHEL_VER),rhel83)
TAG := $(KMODVER)-$(KVER_RHEL83)
IMAGE := $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(TAG)
BUILD_ARGS := $(BUILD_ARGS_RHEL83)
endif

all: image push

image:
	$(BUILDTOOL) build . $(CONTAINER_BUILD_ARGS) -t $(IMAGE)

push: image
	$(BUILDTOOL) push $(IMAGE) $(CONTAINER_PUSH_ARGS)
