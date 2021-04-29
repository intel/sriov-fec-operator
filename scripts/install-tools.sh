#!/bin/bash

set -xe

BASE=$(dirname $(realpath "${BASH_SOURCE[0]}"))
BIN=$(realpath $BASE/../bin)
DOWNLOADS=$(realpath $BASE/../downloads)
REQUIRED_OPERATOR_SDK_VERSION="${1:-v1.4.2}"
SDK_URL="https://github.com/operator-framework/operator-sdk/releases/download"
OPM_URL="https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp/latest-4.6"
OPM_FILE="opm-linux-4.6.26.tar.gz"

if [ ! -e $BIN ] ; then
    mkdir -p $BIN
fi

if [ ! -e $DOWNLOADS ] ; then
    mkdir -p $DOWNLOADS
fi

if [ ! -e $BIN/operator-sdk ] ; then
    curl -sL $SDK_URL/$REQUIRED_OPERATOR_SDK_VERSION/operator-sdk_linux_amd64 -o $BIN/operator-sdk
    chmod +x $BIN/operator-sdk
fi

if [ ! -e $BIN/opm ] ; then
    curl -sL $OPM_URL/$OPM_FILE -o $DOWNLOADS/$OPM_FILE
    tar xvf $DOWNLOADS/$OPM_FILE -C $BIN
fi
