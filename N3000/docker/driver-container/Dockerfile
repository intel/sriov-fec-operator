FROM registry.access.redhat.com/ubi8:latest

ARG KERNEL_VERSION
ENV KERNEL_VERSION=$KERNEL_VERSION

ARG OPAE_VERSION
ENV OPAE_VERSION=$OPAE_VERSION

RUN dnf install -y  \
    kernel-devel-${KERNEL_VERSION} \
    kernel-headers-${KERNEL_VERSION} \
    kernel-core-${KERNEL_VERSION} \
    kernel-modules-${KERNEL_VERSION} \
    elfutils-libelf-devel make gcc patch cpio kmod && \
    dnf clean all

WORKDIR /opae-prepare
COPY n3000-1.3.8-2-rte-el8-setup.sh Makefile.patch ./

# opae-intel-fpga-driver is extracted from opae package
# it's unpacked, Makefile is patched (to allow building for other kernel)
# after building, modules are copied to /lib/modules/$KERNEL_VERSION/extra
RUN ./n3000-1.3.8-2-rte-el8-setup.sh extract && \
    mkdir intel-fpga-driver && \
    cp ./n3000-1.3.8-2-rte/opae/opae-intel-fpga-driver-*.src.rpm ./ && \
    cd intel-fpga-driver && \
    rpm2cpio ../opae-intel-fpga-driver-*.src.rpm | cpio -idmv && \
    tar xf opae-intel-fpga-driver-*.tar.gz && \
    rm -rfv opae-intel-fpga-driver*.tar.gz opae-intel-fpga-driver*.spec && \
    cd opae-intel-fpga-driver-* && \
    cp ../../Makefile.patch . && \
    patch < Makefile.patch && \
    ln -s /usr/src/kernels/${KERNEL_VERSION} /lib/modules/${KERNEL_VERSION}/build && \
    make KERNELVER=${KERNEL_VERSION} && \
    mkdir /lib/modules/${KERNEL_VERSION}/extra && \
    cp -v *.ko /lib/modules/${KERNEL_VERSION}/extra && \
    depmod -a ${KERNEL_VERSION} && \
    rm -rf /opae-prepare

WORKDIR /
COPY entrypoint.sh .
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/bin/bash"]
