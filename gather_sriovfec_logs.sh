#!/bin/bash
K8S_BIN="${1:-oc}"
NAMESPACE="${2:-vran-acceleration-operators}"
CLUSTER_CONFIG="sfcc"
NODE_CONFIG="sfnc"
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

# ClusterConfig
echo "Getting information about ClusterConfigs in ${NAMESPACE}"
mkdir -p clusterConfigs
"${K8S_BIN}" get "${CLUSTER_CONFIG}" -n "${NAMESPACE}" > cluster_configs_in_namespace
clusterConfigs=$("${K8S_BIN}" -n "${NAMESPACE}" get "${CLUSTER_CONFIG}"  --ignore-not-found=true -o custom-columns=NAME:.metadata.name --no-headers=true)
# shellcheck disable=SC2068
for clusterConfig in ${clusterConfigs[@]}; do
   "${K8S_BIN}" -n "${NAMESPACE}" describe "${CLUSTER_CONFIG}" "${clusterConfig}" > clusterConfigs/"${clusterConfig}"
   "${K8S_BIN}" -n "${NAMESPACE}" get "${CLUSTER_CONFIG}" "${clusterConfig}" -o yaml > clusterConfigs/"${clusterConfig}".yaml
done

# NodeConfig
echo "Getting information about NodeConfigs in ${NAMESPACE}"
mkdir -p nodeConfigs
"${K8S_BIN}" get "${NODE_CONFIG}" -n "${NAMESPACE}" > node_configs_in_namespace
nodeConfigs=$("${K8S_BIN}" -n "${NAMESPACE}" get "${NODE_CONFIG}"  --ignore-not-found=true -o custom-columns=NAME:.metadata.name --no-headers=true)
# shellcheck disable=SC2068
for nodeConfig in ${nodeConfigs[@]}; do
   "${K8S_BIN}" -n "${NAMESPACE}" describe "${NODE_CONFIG}" "${nodeConfig}" > nodeConfigs/"${nodeConfig}"
   "${K8S_BIN}" -n "${NAMESPACE}" get "${NODE_CONFIG}" "${nodeConfig}" -o yaml > nodeConfigs/"${nodeConfig}".yaml
done

# system configuration logs
echo "Getting information about system configurations in ${NAMESPACE}"
mkdir -p systemLogs
pods=$("${K8S_BIN}" -n "${NAMESPACE}" get pods -o custom-columns=NAME:.metadata.name --no-headers=true | grep sriov-fec-daemonset)
# shellcheck disable=SC2068
for pod in ${pods[@]}; do
   nodeName=$("${K8S_BIN}" -n "${NAMESPACE}" get pod "${pod}" -o custom-columns=NODE:.spec.nodeName --no-headers=true)
   "${K8S_BIN}" -n "${NAMESPACE}" exec -it "${pod}" -- bash -c "chroot /host dmesg" > systemLogs/dmesg-"${nodeName}".log
   "${K8S_BIN}" -n "${NAMESPACE}" exec -it "${pod}" -- bash -c "chroot /host lspci -vvv" > systemLogs/lspci-"${nodeName}".log
   telemetryFiles=$("${K8S_BIN}" -n "${NAMESPACE}" exec -it "${pod}" -- bash -c "ls -f -A1 /var/log/|grep pf_bb_cfg| tr -d '\n'")
   for telemetryFiles in ${telemetryFiles[@]}; do
      "${K8S_BIN}" -n "${NAMESPACE}" exec -it "${pod}" -- bash -c "cat /var/log/${telemetryFiles}" > systemLogs/"${nodeName}"-"${telemetryFiles}"
   done
done

cd ../
tar -zcvf sriov-fec.logs.tar.gz "${DIR}"
echo "Please attach 'sriov-fec.logs.tar.gz' to bug report. If you had to apply some configs and deleted them to reproduce issue, attach them as well."
