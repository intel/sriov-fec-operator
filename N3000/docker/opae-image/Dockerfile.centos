ARG CENTOS_VER=docker.io/centos:8.2.2004
FROM ${CENTOS_VER}

WORKDIR /root

RUN rpm --import https://www.elrepo.org/RPM-GPG-KEY-elrepo.org
RUN yum install -y dnf-plugins-core epel-release
RUN yum config-manager --set-enabled PowerTools

RUN yum install -y \
        python3 \
        python3-pip \
        python3-devel \
        python3-pybind11 \
        make \
        libuuid-devel \
        json-c-devel \
        gcc \
        clang \
        gcc-c++ \
        hwloc-devel \
        tbb-devel \
        rpm-build \
        rpmdevtools \
        git \
        libedit-devel \
        epel-release

RUN yum install -y cmake3

RUN python3 -m pip install setuptools --upgrade
RUN python3 -m pip install wheel
RUN python3 -m pip install python-pkcs11 cython pyyaml
WORKDIR /root
RUN git clone -b hundeboll/feature/fpga_smartnic_n5010 https://github.com/OPAE/opae-sdk.git /root/opae-sdk

WORKDIR /root/opae-sdk/build
RUN cmake .. --warn-uninitialized -Wno-dev \
             -DCMAKE_INSTALL_PREFIX=/opae \
             -DCMAKE_BUILD_TYPE=Release \
             -DCMAKE_CXX_STANDARD=11 \
             -DSPDLOG_BUILD_EXAMPLE=ON \
             -DSPDLOG_BUILD_EXAMPLE_HO=ON \
             -DSPDLOG_BUILD_WARNINGS=ON \
             -DSPDLOG_BUILD_BENCH=OFF \
             -DSPDLOG_BUILD_TESTS=ON \
             -DCOMPILER_SUPPORTS_CXX14=no \
             -DSPDLOG_BUILD_TESTS_HO=OFF \
             -DSPDLOG_SANITIZE_ADDRESS=On && \
    make VERBOSE=1 install -j4

RUN cd /root/opae-sdk/python/opae.admin && python3 setup.py install --single-version-externally-managed --root=/opae
RUN cd /root/opae-sdk/python/opae.io && python3 setup.py install --single-version-externally-managed --root=/opae
RUN cd /root/opae-sdk/python/pacsign && python3 setup.py install --single-version-externally-managed --root=/opae
RUN cd /root/opae-sdk/tools/extra/fpgadiag && python3 setup.py install --single-version-externally-managed --root=/opae

FROM registry.access.redhat.com/ubi8/ubi-minimal

RUN microdnf install -y python3 python3-pip pciutils rsync kmod net-tools ethtool

COPY --from=0 /opae/bin/* /bin/
COPY --from=0 /opae/lib64/* /usr/lib64/
COPY --from=0 /opae/usr/local /usr/local
