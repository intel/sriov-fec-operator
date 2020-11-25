#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020 Intel Corporation

set -e

function enable_repo() {
    dnf config-manager --set-enabled "${1}" || true
    if ! dnf makecache; then
        dnf config-manager --set-disabled "${1}"
        echo "${1} not enabled"
    fi
    echo "${1} enabled"
}

[ -z "${RELEASE_VERSION}" ] && echo "RELEASE_VERSION not set" && exit 1
[ -z "${OCP_VERSION}" ] && echo "OCP_VERSION not set" && exit 1
[ -z "${KERNEL_VERSION}" ] && echo "KERNEL_VERSION not set" && exit 1

echo "${RELEASE_VERSION}" > /etc/yum/vars/releasever

echo "Setting install_weak_deps=False globally"
dnf config-manager --setopt=install_weak_deps=False --save

enable_repo rhocp-"${OCP_VERSION}"-for-rhel-8-x86_64-rpms
enable_repo rhel-8-for-x86_64-baseos-eus-rpms


if [[ "${KERNEL_VERSION}" == *"rt"* ]]; then
    enable_repo rhel-8-for-x86_64-nfv-rpms
fi

echo "Update..."
dnf update -y
