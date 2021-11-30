# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020 Intel Corporation

FROM registry.access.redhat.com/ubi8:8.5-200 as builder

ARG KERNEL_VERSION
ENV KERNEL_VERSION=$KERNEL_VERSION

ARG KERNEL_SOURCE
ENV KERNEL_SOURCE=$KERNEL_SOURCE

ARG OPAE_VERSION
ENV OPAE_VERSION=$OPAE_VERSION

ARG OCP_VERSION=4.6
ENV OCP_VERSION=$OCP_VERSION

ARG RELEASE_VERSION=8.2
ENV RELEASE_VERSION=$RELEASE_VERSION

COPY enable_repositories.sh ./
RUN ./enable_repositories.sh

COPY files ./files

RUN dnf install --disablerepo=* --enablerepo=ubi-8-appstream --enablerepo=ubi-8-baseos  -y  \
    make gcc patch cpio kmod

RUN if [[ "${KERNEL_SOURCE}" == *"file"* ]]; then \
        rpm -Uvh --nodeps ./files/kernel/*.rpm; \
        dnf install -y elfutils-libelf-devel; \
        dnf clean all; \
    elif [[ "${KERNEL_SOURCE}" == *"yum"* ]]; then \
        [[ "${KERNEL_VERSION}" == *"rt"* ]] && RT="rt-" || RT="" && \
        dnf install -y \
        kernel-${RT}devel-${KERNEL_VERSION} \
        kernel-${RT}core-${KERNEL_VERSION} \
        elfutils-libelf-devel && \
        dnf clean all; \
    fi

WORKDIR /opae-prepare
ARG OPAE_VERSION=$OPAE_VERSION

COPY files/opae/n3000-${OPAE_VERSION}-rte-el8-setup.sh Makefile.patch ./

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
    cp /opae-prepare/n3000-$OPAE_VERSION-rte/opae/opae-intel-fpga-driver-*.src.rpm /opae-drivers/ && \
    rm -rf /opae-prepare

FROM registry.access.redhat.com/ubi8:8.5-200

ARG VERSION
### Required OpenShift Labels
LABEL name="OPAE Intel® FPGA Linux Device Drivers container($OPAE_VERSION)" \
    vendor="Intel Corporation" \
    version=$VERSION \
    release="1" \
    summary="OPAE Intel® FPGA Linux Device Drivers for kernel ${KERNEL_VERSION}" \
    description="The OPAE Intel® FPGA driver provides interfaces for userspace applications \
to configure, enumerate, open, and access FPGA accelerators on platforms \
equipped with Intel® FPGA solutions and enables system-level management \
functions such as FPGA reconfiguration, power management, and virtualization.\
https://opae.github.io/latest/docs/drv_arch/drv_arch.html"

COPY TEMP_LICENSE_COPY /licenses/LICENSE
WORKDIR /srcrpms
RUN dnf download --disablerepo=* --enablerepo=ubi-8-baseos --source kmod
WORKDIR /driver-workdir
RUN dnf install --disablerepo=* --enablerepo=ubi-8-baseos --setopt=install_weak_deps=False -y kmod
USER 1001
COPY --from=builder /opae-drivers/opae-intel-fpga-driver-*.src.rpm  /opae-drivers/
COPY --chown=1001 entrypoint.sh .
RUN chmod +x /driver-workdir/entrypoint.sh

COPY --from=builder /opae-drivers/${KERNEL_VERSION} /opae-drivers/${KERNEL_VERSION}

ENTRYPOINT ["/bin/bash"]
