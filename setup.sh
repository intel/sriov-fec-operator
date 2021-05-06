#!/bin/bash

set -x

base=$(dirname $(realpath "${BASH_SOURCE[0]}"))

WEB_PORT=8080

if $(oc get ns -A | grep -q vran-acceleration-operators) ; then
    oc delete ns vran-acceleration-operators
fi

if $(oc get ns -A | grep -q vran-acceleration-operators) ; then
    oc delete ns nfd
fi

sleep 10

oc create ns vran-acceleration-operators

releases=/net/bohr/var/fiberblaze/releases/LightningCreek/ofs-fim/N5010
install_dir=/disks/openshift-provision/install_dir

tar xvf $releases/0_0_1/N5010_ofs-fim_PR_gbs_0_0_1.tar.gz -C $install_dir --wildcards "*_unsigned.bin"

#oc create ns vran-acceleration-operators
#oc create serviceaccount -n vran-acceleration-operators controller-manager
#oc adm policy add-scc-to-user privileged -n vran-acceleration-operators -z controller-manager

sleep 2

oc apply -k $base/N3000/config/default

sleep 60

oc apply -f $base/N3000/config/samples/fpga_v1_n3000cluster.yaml

if ! $(docker ps -a | grep -q static-file-server) ; then
    docker run -d --name static-file-server --rm  -v ${install_dir}:/web -p ${WEB_PORT}:${WEB_PORT} -u $(id -u):$(id -g) halverneus/static-file-server:latest
fi
