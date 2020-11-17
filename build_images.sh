#!/bin/bash


# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020 Intel Corporation


set -e

OPAE_VER=${OPAE_VERSION:-1.3.8-2}
KERNEL_VER=${KERNEL_VERSION:-4.18.0-193.14.3.el8_2.x86_64}
IMAGE_VER=${IMAGE_VERSION:-v1.0.0}


IMAGES_TO_BUILD=""
REGISTRY=""
IMAGES_BUILT=()

usage() { echo "Usage: $0 [-r registry] [-i <image1,image2>]" 1>&2; exit 1; }

while getopts ":r:i:" o; do
    case "${o}" in
        r)
            REGISTRY=${OPTARG}
            ;;
        i)
            IMAGES_TO_BUILD=${OPTARG}
            ;;
        *)
            usage
            ;;
    esac
done
shift $((OPTIND-1))

function build_image {
    cont_dir="${1}"
    cont_dockerfile="${2}"
    img_name="${3}"
    build_args="${4}"


    pushd "${cont_dir}" > /dev/null || exit
        if [ -z "${IMAGES_TO_BUILD}" ] || [[ "${IMAGES_TO_BUILD}" =~ ${img_name%:*} ]]; then
            echo "Building ${img_name}"
            # shellcheck disable=SC2086
            podman build . -t "${img_name}" -f "${cont_dockerfile}" ${build_args} > /dev/null
            IMAGES_BUILT+=("${img_name}")
        fi
    popd > /dev/null || exit
}

### BUILD OPAE base image
    OPAE_TAG="${OPAE_VER}"
    OPAE_IMG="opae:${OPAE_TAG}"
    OPAE_BUILD_ARGS="--build-arg=OPAE_VERSION=${OPAE_VER}"
    build_image "N3000/docker/opae-image" "Dockerfile" "${OPAE_IMG}" "${OPAE_BUILD_ARGS}"
### END

### BUILD Driver container
    DRV_TAG="${IMAGE_VER}--${OPAE_VER}--${KERNEL_VER}"
    DRV_IMG="n3000-driver:${DRV_TAG}"
    DRV_BUILD_ARGS="--build-arg=OPAE_VERSION=${OPAE_VER} --build-arg=KERNEL_VERSION=${KERNEL_VER}"
    build_image "N3000/docker/driver-container" "Dockerfile" "${DRV_IMG}" "${DRV_BUILD_ARGS}"
### END

### BUILD N3000 Operator
    N3000_TAG="${IMAGE_VER}"
    N3000_IMG="n3000-operator:${N3000_TAG}"
    N3000_BUILD_ARGS=""
    build_image "N3000" "Dockerfile" "${N3000_IMG}" "${N3000_BUILD_ARGS}"
### END

### BUILD N3000 daemon
    N3000_D_TAG="${IMAGE_VER}--${OPAE_VER}"
    N3000_D_IMG="n3000-daemon:${N3000_D_TAG}"
    N3000_D_BUILD_ARGS="--build-arg=OPAE_VERSION=${OPAE_VER}"
    build_image "N3000" "Dockerfile.daemon" "${N3000_D_IMG}" "${N3000_D_BUILD_ARGS}"
### END

### BUILD Prometheus exporter
    PROMETHEUS_TAG="${IMAGE_VER}--${OPAE_VER}"
    PROMETHEUS_IMG="n3000-monitoring:${PROMETHEUS_TAG}"
    PROMETHEUS_BUILD_ARGS="--build-arg=OPAE_VERSION=${OPAE_VER}"
    build_image "prometheus_fpgainfo_exporter" "Dockerfile" "${PROMETHEUS_IMG}" "${PROMETHEUS_BUILD_ARGS}"
### END

### BUILD SRIOV FEC operator
    SRIOV_TAG="${IMAGE_VER}"
    SRIOV_IMG="sriov-fec-operator:${SRIOV_TAG}"
    SRIOV_BUILD_ARGS=""
    build_image "sriov-fec" "Dockerfile" "${SRIOV_IMG}" "${SRIOV_BUILD_ARGS}"
### END

### BUILD SRIOV FEC daemon
    SRIOV_D_TAG="${IMAGE_VER}"
    SRIOV_D_IMG="sriov-fec-daemon:${SRIOV_D_TAG}"
    SRIOV_D_BUILD_ARGS=""
    build_image "sriov-fec" "Dockerfile.daemon" "${SRIOV_D_IMG}" "${SRIOV_D_BUILD_ARGS}"
### END


if [ -n "${REGISTRY}" ]
then
    for img in "${IMAGES_BUILT[@]}"; do
        echo "Pushing ${img} to ${REGISTRY}"
        podman push localhost/"${img}" "${REGISTRY}/${img}"
    done
fi

echo "Built images:"
printf '%s\n' "${IMAGES_BUILT[@]}"
