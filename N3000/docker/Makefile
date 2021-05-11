.PHONY: all image push

KMODVER := eea9cbc
KVER_RHEL82 = 4.18.0-193.el8.x86_64
KVER_RHEL83 = 4.18.0-240.el8.x86_64
RHEL82_RPM := https://vault.centos.org/8.2.2004/BaseOS/x86_64/os/Packages/kernel-devel-4.18.0-193.el8.x86_64.rpm
RHEL83_RPM := http://mirror.centos.org/centos/8/BaseOS/x86_64/os/Packages/kernel-devel-4.18.0-240.el8.x86_64.rpm

BUILD_ARGS_RHEL82 = --build-arg CENTOS_VER=docker.io/centos:8.2.2004 --build-arg RPM_URL=$(RHEL82_RPM) --build-arg KMODVER=$(KMODVER) --build-arg KVER=$(KVER_RHEL82)
BUILD_ARGS_RHEL83 = --build-arg CENTOS_VER=docker.io/centos:8.3.2011 --build-arg RPM_URL=$(RHEL83_RPM) --build-arg KMODVER=$(KMODVER) --build-arg KVER=$(KVER_RHEL83)

BUILDTOOL ?= podman
IMAGE_NAME = dfl-kmod
REPOS ?= quay.io/ryan_raasch

all: image push

image:
	make -C driver-container
	make -C opae-image

push: image
	make -C driver-container push
	make -C opae-image push
