#!/bin/bash


# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020 Intel Corporation


set -e

IMAGES_TO_BUILD=""

usage() { echo "Usage: $0 [-r registry] [-i <image1,image2>] [-t]" 1>&2; exit 1; }

while getopts ":r:i:t" o; do
    case "${o}" in
        r)
            export IMAGE_REGISTRY=${OPTARG}
            ;;
        i)
            IMAGES_TO_BUILD=${OPTARG}
            ;;
        t)
            export TLS_VERIFY="false"
            ;;
        *)
            usage
            ;;
    esac
done
shift $((OPTIND-1))

function build_image {
    dir="${1}"
    img_name="${2}"

    pushd "${dir}" > /dev/null || exit
        if [ -z "${IMAGES_TO_BUILD}" ] || [[ "${IMAGES_TO_BUILD}" =~ ${img_name%:*} ]]; then
            echo "Building ${img_name}"
            make "image-${img_name}"
            if [ -n "${IMAGE_REGISTRY}" ]
            then
                echo "Pushing ${img_name}"
                make "push-${img_name}"
            fi
        fi
    popd > /dev/null || exit
}

### BUILD Driver container
    build_image "N3000" "n3000-driver"
### END

### BUILD N3000 Operator
    build_image "N3000" "n3000-operator"
### END

### BUILD N3000 daemon
    build_image "N3000" "n3000-daemon"
### END

### BUILD Prometheus exporter
    build_image "prometheus_fpgainfo_exporter" "n3000-monitoring"
### END

### BUILD SRIOV FEC operator
    build_image "sriov-fec" "sriov-fec-operator"
### END

### BUILD SRIOV FEC daemon
    build_image "sriov-fec" "sriov-fec-daemon"
### END

### BUILD Node labeler daemon
    build_image "N3000" "n3000-labeler"
### END

### BUILD N3000 Bundle
    build_image "N3000" "bundle"
### END

### BUILD SRIOV FEC Bundle
    build_image "sriov-fec" "bundle"
### END
