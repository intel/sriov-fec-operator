```text
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2020-2025 Intel Corporation
```
## Technical Requirements and Dependencies

The SRIOV-FEC Operator for Wireless FEC Accelerators has the following requirements:

- [Intel® vRAN Dedicated Accelerator ACC100](https://builders.intel.com/docs/networkbuilders/intel-vran-dedicated-accelerator-acc100-product-brief.pdf)
- [Kubernetes 1.22](https://kubernetes.io/blog/2021/08/04/kubernetes-1-22-release-announcement/)
- RT Kernel configured for OS [Centos 7](https://linuxsoft.cern.ch/cern/centos/7/rt/x86_64/repoview/kernel-rt.html) or [Ubuntu](https://askubuntu.com/questions/1349568/installing-real-time-patch-for-ubuntu-20-04)
- [Configured kernel parameters](https://wiki.ubuntu.com/Kernel/KernelBootParameters#Permanently_Add_a_Kernel_Boot_Parameter): 
  - Always required: `"intel_iommu=on", "iommu=pt"`
  - sriov-fec:2.3.0 comes with initial support of `vfio-pci` driver for ACC100. Configurations leveraging `vfio-pci` require following kernel parameters:
    - vfio_pci.enable_sriov=1
    - vfio_pci.disable_idle_d3=1
- BIOS with enabled settings "Intel® Virtualization Technology for Directed I/O" (VT-d), "Single Root I/O Virtualization" (SR-IOV) and "Input–Output Memory Management Unit" (IOMMU)

### Building images
Prerequisite: Make sure that the images used by the operator are pushed to IMAGE_REGISTRY and all nodes in cluster have access to IMAGE_REGISTRY

The operator-sdk CLI is required - see [Getting started with the Operator SDK](https://docs.openshift.com/container-platform/4.6/operators/operator_sdk/osdk-getting-started.html#osdk-installing-cli_osdk-getting-started).

Install OPM (if not already installed):

```shell
# export RELEASE_VERSION=v1.21.0
# curl -LO https://github.com/operator-framework/operator-registry/releases/download/${RELEASE_VERSION}/linux-amd64-opm
# chmod +x linux-amd64-opm
# sudo mkdir -p /usr/local/bin/
# sudo cp linux-amd64-opm /usr/local/bin/opm
# rm -f linux-amd64-opm
```

Determine local registry address:

```shell
# export IMAGE_REGISTRY=<IP_ADDRESS>:<PORT>
```

Navigate to operator directory:

```shell
# cd sriov-fec-operator
```

Export `VERSION` variable

```shell
# export VERSION=2.2.0
```

Build and push images to local registry:

```shell
# IMAGE_REGISTRY=${IMAGE_REGISTRY} TLS_VERIFY=false VERSION=${VERSION} make build_all
```

### Install dependencies

If Kubernetes doesn't have installed OLM (Operator Lifecycle Manager - https://olm.operatorframework.io/) start from installing Operator SDK (https://sdk.operatorframework.io/)

After Operator-sdk installation run following command
```shell
[user@ctrl1 /home]# operator-sdk olm install
```
Install PCIutils on worker nodes
```shell
[user@ctrl1 /home]# yum install pciutils
```
### Install the Operator

To install the SRIOV-FEC Operator for Wireless FEC Accelerators operator bundle perform the following steps:

Create the namespace for project:
```shell
[user@ctrl1 /home]# kubectl create namespace vran-acceleration-operators
[user@ctrl1 /home]# kubectl config set-context --current --namespace=vran-acceleration-operators
```

Install operator using `bundle` image
```
# operator-sdk run bundle ${IMAGE_REGISTRY}/sriov-fec-bundle:v${VERSION} --namespace vran-acceleration-operators --install-mode OwnNamespace
```

Verify that the operator is installed and pods are running:

```shell
[user@ctrl1 /home]# kubectl get csv
NAME               DISPLAY                                                        VERSION   REPLACES   PHASE
sriov-fec.v2.2.0   SR-IOV Operator for Wireless FEC Accelerators              2.2.0                Succeeded
```

```shell
[user@ctrl1 /home]# kubectl get pod
NAME                                            READY   STATUS    RESTARTS   AGE                                                                              
                                           
sriov-device-plugin-hkq6f                       1/1     Running   0          35s                                                                              
sriov-fec-controller-manager-78488c4c65-cpknc   2/2     Running   0          44s                                                                              
sriov-fec-daemonset-7h8kb                       1/1     Running   0          35s                                                                              
```

### Configuration for telemetry

To automatically gather metrics on Kubernetes, you have to install [kube-prometheus](https://github.com/prometheus-operator/kube-prometheus) or [prometheus-operator](https://github.com/prometheus-operator/prometheus-operator)
Assuming that operator is deployed in `vran-acceleration-operators` and Prometheus stack is deployed in `monitoring` namespace, you will have to apply following CRs:

```
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: prometheus-k8s
  namespace: vran-acceleration-operators
rules:
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
  - list
  - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: prometheus-k8s
  namespace: vran-acceleration-operators
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: prometheus-k8s
subjects:
- kind: ServiceAccount
  name: prometheus-k8s
  namespace: monitoring
---
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: bbdevconfig
  namespace: monitoring
spec:
  namespaceSelector:
    matchNames:
    - vran-acceleration-operators
  podMetricsEndpoints:
  - port: bbdevconfig
    path: /bbdevconfig
    interval: 1m
    relabelings:
    - action: replace
      sourceLabels:
      - __meta_kubernetes_pod_node_name
      targetLabel: instance
  selector:
    matchLabels:
      app: sriov-fec-daemonset
```
If any of the operators is deployed in different namespace, then modify namespace accordingly.

### Uninstalling Previously Installed Operator

To uninstall operator execute command

```shell
[user@ctrl1 /home]# operator-sdk cleanup sriov-fec --namespace vran-acceleration-operators
```
