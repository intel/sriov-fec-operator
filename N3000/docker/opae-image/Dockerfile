FROM registry.access.redhat.com/ubi8:latest

RUN dnf install -y \
     python3 json-c

# 1.3.8-2
ARG OPAE_VERSION
ENV OPAE_VERSION=${OPAE_VERSION}

WORKDIR /opae

COPY n3000-${OPAE_VERSION}-rte-el8-setup.sh .

# We extract the package instead of installing directly,
# because there's no way to ignore driver package (and its deps are not fulfilled [dkms])
RUN ./n3000-${OPAE_VERSION}-rte-el8-setup.sh extract && \
        pushd n3000-${OPAE_VERSION}-rte/opae && \
        dnf install -y $(find . \( -iname '*.rpm' -a -not -iname '*driver*' -a -not -iname '*sign*' \)) && \
        dnf clean all && \
        popd && \
        rm -rf ./n3000*
