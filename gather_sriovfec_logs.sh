#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020-2024 Intel Corporation

K8S_BIN="${1:-oc}"
NAMESPACE="${2:-vran-acceleration-operators}"
SRIOV_FEC_CLUSTER_CONFIG="sfcc"
SRIOV_FEC_NODE_CONFIG="sfnc"
SRIOV_VRB_CLUSTER_CONFIG="svcc"
SRIOV_VRB_NODE_CONFIG="svnc"
DIR=sriov-fec-$(hostname)-$(date)

mkdir -p "${DIR}"
cd "${DIR}" || exit

"${K8S_BIN}" version > "${K8S_BIN}"_version
"${K8S_BIN}" get csv -n "${NAMESPACE}" > csv_in_namespace

# nodes
echo "Getting information about nodes"
"${K8S_BIN}" get nodes -o wide -n "${NAMESPACE}" > nodes_in_namespace
mkdir -p nodes
nodes=$("${K8S_BIN}" get nodes -o custom-columns=NAME:.metadata.name --no-headers=true)
# shellcheck disable=SC2068
for node in ${nodes[@]}; do
   "${K8S_BIN}" describe node "${node}" > nodes/"${node}"
   "${K8S_BIN}" get node "${node}" -o yaml > nodes/"${node}".yaml
done

# pods
echo "Getting information about pods in ${NAMESPACE}"
"${K8S_BIN}" get all -n "${NAMESPACE}" -o wide > resources_in_namespace
mkdir -p pods
pods=$("${K8S_BIN}" -n "${NAMESPACE}" get pods -o custom-columns=NAME:.metadata.name --no-headers=true)
# shellcheck disable=SC2068
for pod in ${pods[@]}; do
   "${K8S_BIN}" -n "${NAMESPACE}" logs --all-containers=true "${pod}" > pods/"${pod}".log
   "${K8S_BIN}" -n "${NAMESPACE}" get pod "${pod}" -o yaml > pods/"${pod}".yaml
done

# SriovFecClusterConfig
echo "Getting information about SriovFecClusterConfigs in ${NAMESPACE}"
mkdir -p sriovfecclusterConfigs
"${K8S_BIN}" get "${SRIOV_FEC_CLUSTER_CONFIG}" -n "${NAMESPACE}" > sriov_fec_cluster_configs_in_namespace
sriovfecclusterConfigs=$("${K8S_BIN}" -n "${NAMESPACE}" get "${SRIOV_FEC_CLUSTER_CONFIG}"  --ignore-not-found=true -o custom-columns=NAME:.metadata.name --no-headers=true)
# shellcheck disable=SC2068
for sriovfecclusterConfig in ${sriovfecclusterConfigs[@]}; do
   "${K8S_BIN}" -n "${NAMESPACE}" describe "${SRIOV_FEC_CLUSTER_CONFIG}" "${sriovfecclusterConfig}" > sriovfecclusterConfigs/"${sriovfecclusterConfig}"
   "${K8S_BIN}" -n "${NAMESPACE}" get "${SRIOV_FEC_CLUSTER_CONFIG}" "${sriovfecclusterConfig}" -o yaml > sriovfecclusterConfigs/"${sriovfecclusterConfig}".yaml
done

# SriovVrbClusterConfig
echo "Getting information about SriovVrbClusterConfigs in ${NAMESPACE}"
mkdir -p sriovvrbclusterConfigs
"${K8S_BIN}" get "${SRIOV_VRB_CLUSTER_CONFIG}" -n "${NAMESPACE}" > sriov_vrb_cluster_configs_in_namespace
sriovvrbclusterConfigs=$("${K8S_BIN}" -n "${NAMESPACE}" get "${SRIOV_VRB_CLUSTER_CONFIG}"  --ignore-not-found=true -o custom-columns=NAME:.metadata.name --no-headers=true)
# shellcheck disable=SC2068
for sriovvrbclusterConfig in ${sriovvrbclusterConfigs[@]}; do
   "${K8S_BIN}" -n "${NAMESPACE}" describe "${SRIOV_VRB_CLUSTER_CONFIG}" "${sriovvrbclusterConfig}" > sriovvrbclusterConfigs/"${sriovvrbclusterConfig}"
   "${K8S_BIN}" -n "${NAMESPACE}" get "${SRIOV_VRB_CLUSTER_CONFIG}" "${sriovvrbclusterConfig}" -o yaml > sriovvrbclusterConfigs/"${sriovvrbclusterConfig}".yaml
done

# SriovFecNodeConfig
echo "Getting information about SriovFecNodeConfigs in ${NAMESPACE}"
mkdir -p sriovfecnodeConfigs
"${K8S_BIN}" get "${SRIOV_FEC_NODE_CONFIG}" -n "${NAMESPACE}" > sriov_fec_node_configs_in_namespace
sriovfecnodeConfigs=$("${K8S_BIN}" -n "${NAMESPACE}" get "${SRIOV_FEC_NODE_CONFIG}"  --ignore-not-found=true -o custom-columns=NAME:.metadata.name --no-headers=true)
# shellcheck disable=SC2068
for sriovfecnodeConfig in ${sriovfecnodeConfigs[@]}; do
   "${K8S_BIN}" -n "${NAMESPACE}" describe "${SRIOV_FEC_NODE_CONFIG}" "${sriovfecnodeConfig}" > sriovfecnodeConfigs/"${sriovfecnodeConfig}"
   "${K8S_BIN}" -n "${NAMESPACE}" get "${SRIOV_FEC_NODE_CONFIG}" "${sriovfecnodeConfig}" -o yaml > sriovfecnodeConfigs/"${sriovfecnodeConfig}".yaml
done

# SriovVrbNodeConfig
echo "Getting information about SriovVrbNodeConfigs in ${NAMESPACE}"
mkdir -p sriovvrbnodeConfigs
"${K8S_BIN}" get "${SRIOV_VRB_NODE_CONFIG}" -n "${NAMESPACE}" > sriov_vrb_node_configs_in_namespace
sriovvrbnodeConfigs=$("${K8S_BIN}" -n "${NAMESPACE}" get "${SRIOV_VRB_NODE_CONFIG}"  --ignore-not-found=true -o custom-columns=NAME:.metadata.name --no-headers=true)
# shellcheck disable=SC2068
for sriovvrbnodeConfig in ${sriovvrbnodeConfigs[@]}; do
   "${K8S_BIN}" -n "${NAMESPACE}" describe "${SRIOV_VRB_NODE_CONFIG}" "${sriovvrbnodeConfig}" > sriovvrbnodeConfigs/"${sriovvrbnodeConfig}"
   "${K8S_BIN}" -n "${NAMESPACE}" get "${SRIOV_VRB_NODE_CONFIG}" "${sriovvrbnodeConfig}" -o yaml > sriovvrbnodeConfigs/"${sriovvrbnodeConfig}".yaml
done

# system configuration logs
echo "Getting information about system configurations in ${NAMESPACE}"
mkdir -p systemLogs
pods=$("${K8S_BIN}" -n "${NAMESPACE}" get pods -o custom-columns=NAME:.metadata.name --no-headers=true | grep sriov-fec-daemonset)
# shellcheck disable=SC2068
for pod in ${pods[@]}; do
   nodeName=$("${K8S_BIN}" -n "${NAMESPACE}" get pod "${pod}" -o custom-columns=NODE:.spec.nodeName --no-headers=true)
   "${K8S_BIN}" -n "${NAMESPACE}" exec -it "${pod}" -- bash -c "chroot / lspci -vvv" > systemLogs/lspci-"${nodeName}".log
   telemetryFiles=$("${K8S_BIN}" -n "${NAMESPACE}" exec -it "${pod}" -- bash -c "ls -f -A1 /var/log/"|grep pf_bb_cfg| tr -d '\r')
   for telemetryFiles in ${telemetryFiles[@]}; do
      "${K8S_BIN}" -n "${NAMESPACE}" exec -it "${pod}" -- bash -c "cat /var/log/${telemetryFiles}" > systemLogs/"${nodeName}"-"${telemetryFiles}"
   done
done

cd ../
tar -zcvf sriov-fec.logs.tar.gz "${DIR}"
echo "Please attach 'sriov-fec.logs.tar.gz' to bug report. If you had to apply some configs and deleted them to reproduce issue, attach them as well."
