# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020 Intel Corporation

FROM registry.access.redhat.com/ubi8:latest

RUN dnf install --disablerepo=*  --enablerepo=ubi-8-appstream --enablerepo=ubi-8-baseos -y \
     python3 json-c procps-ng cpio

WORKDIR /srcrpms
RUN dnf download --disablerepo=* --enablerepo=ubi-8-appstream --enablerepo=ubi-8-baseos --source procps-ng cpio

RUN pip3 install intelhex

ARG OPAE_VERSION
ENV OPAE_VERSION=${OPAE_VERSION}

COPY TEMP_LICENSE_COPY /licenses/LICENSE

WORKDIR /opae

COPY files/opae/n3000-${OPAE_VERSION}-rte-el8-setup.sh .

# We extract the package instead of installing directly,
# because there's no way to ignore driver package (and its deps are not fulfilled [dkms])
RUN ./n3000-${OPAE_VERSION}-rte-el8-setup.sh extract && \
     pushd n3000-${OPAE_VERSION}-rte/opae && \
     rpm2cpio opae-tools-extra*.rpm | cpio -idv ./usr/bin/fpgadiag ./usr/bin/mactest && \
     mv ./usr/bin/* /usr/bin/ && rm -rf ./usr/ && \
     dnf install --disablerepo=* --enablerepo=ubi-8-appstream --enablerepo=ubi-8-baseos -y \
     $(find . \( -iname '*.rpm' -a -not -iname '*driver*' -a -not -iname '*sign*' -a -not -iname '*opae-tools-extra*' \)) && \
     dnf clean all && \
     popd && \
     rm -rf ./n3000*
