# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020 Intel Corporation

FROM registry.access.redhat.com/ubi8:latest as builder

ARG KERNEL_VERSION
ENV KERNEL_VERSION=$KERNEL_VERSION

ARG OPAE_VERSION
ENV OPAE_VERSION=$OPAE_VERSION

RUN dnf install -y  \
    kernel-devel-${KERNEL_VERSION} \
    kernel-headers-${KERNEL_VERSION} \
    kernel-core-${KERNEL_VERSION} \
    elfutils-libelf-devel make gcc patch cpio kmod && \
    dnf clean all

WORKDIR /opae-prepare
ARG OPAE_VERSION=$OPAE_VERSION

COPY n3000-$OPAE_VERSION-rte-el8-setup.sh Makefile.patch ./

# opae-intel-fpga-driver is extracted from opae package
# it's unpacked, Makefile is patched (to allow building for other kernel)
# after building, modules are copied to /lib/modules/$KERNEL_VERSION/extra
RUN ./n3000-$OPAE_VERSION-rte-el8-setup.sh extract && \
    mkdir intel-fpga-driver && \
    cp ./n3000-$OPAE_VERSION-rte/opae/opae-intel-fpga-driver-*.src.rpm ./ && \
    cd intel-fpga-driver && \
    rpm2cpio ../opae-intel-fpga-driver-*.src.rpm | cpio -idmv && \
    tar xf opae-intel-fpga-driver-*.tar.gz && \
    rm -rfv opae-intel-fpga-driver*.tar.gz opae-intel-fpga-driver*.spec && \
    cd opae-intel-fpga-driver-* && \
    cp ../../Makefile.patch . && \
    patch < Makefile.patch && \
    ln -s /usr/src/kernels/${KERNEL_VERSION} /lib/modules/${KERNEL_VERSION}/build && \
    make KERNELVER=${KERNEL_VERSION} && \
    mkdir -p /opae-drivers/${KERNEL_VERSION} && \
    cp -v *.ko /opae-drivers/${KERNEL_VERSION} && \
    rm -rf /opae-prepare

FROM registry.access.redhat.com/ubi8:latest
WORKDIR /driver-workdir
RUN dnf install --disablerepo=* --enablerepo=ubi-8-baseos -y kmod
COPY --from=builder /opae-drivers/${KERNEL_VERSION} /opae-drivers/${KERNEL_VERSION}
COPY entrypoint.sh .
RUN chmod +x /driver-workdir/entrypoint.sh

ENTRYPOINT ["/bin/bash"]
