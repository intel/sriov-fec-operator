## SPDX-License-Identifier: Apache-2.0
## Copyright (c) 2020-2024 Intel Corporation

## Technical Requirements and Dependencies

The SRIOV-FEC Operator for Wireless FEC Accelerators has the following requirements:

- [Intel® vRAN Dedicated Accelerator ACC100](https://builders.intel.com/docs/networkbuilders/intel-vran-dedicated-accelerator-acc100-product-brief.pdf)
- [OpenShift 4.10.x](https://docs.openshift.com/container-platform/4.10/release_notes/ocp-4-10-release-notes.html)
- RT Kernel configured with [Performance Addon Operator](https://access.redhat.com/documentation/en-us/openshift_container_platform/4.6/html/scalability_and_performance/cnf-performance-addon-operator-for-low-latency-nodes).
- sriov-fec:2.3.0 comes with initial support of `vfio-pci` driver for ACC100. Configurations leveraging `vfio-pci` require following kernel parameters:
    - vfio_pci.enable_sriov=1
    - vfio_pci.disable_idle_d3=1
- BIOS with enabled settings "Intel® Virtualization Technology for Directed I/O" (VT-d), "Single Root I/O Virtualization" (SR-IOV) and "Input–Output Memory Management Unit" (IOMMU)

### Install the Bundle

To install the SRIOV-FEC Operator for Wireless FEC Accelerators operator bundle perform the following steps:

Create the project:
```shell
[user@ctrl1 /home]# oc new-project vran-acceleration-operators
```

**Optional:** Annotate the project to enable management workload partitioning:
```
oc annotate namespace vran-acceleration-operators workload.openshift.io/allowed=management
```

Execute following commands on cluster:

Create an operator group and the subscriptions (all the commands are run in the `vran-acceleration-operators` namespace):

```shell
[user@ctrl1 /home]#  cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: vran-operators
  namespace: vran-acceleration-operators
spec:
  targetNamespaces:
    - vran-acceleration-operators
EOF
```

```shell
[user@ctrl1 /home]#  cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: sriov-fec-subscription
  namespace: vran-acceleration-operators
spec:
  channel: stable
  name: sriov-fec
  source: certified-operators
  sourceNamespace: openshift-marketplace
EOF
```

Verify that the operators are installed and pods are running:

```shell
[user@ctrl1 /home]# oc get csv
NAME               DISPLAY                                                        VERSION   REPLACES   PHASE
sriov-fec.v2.2.0   SR-IOV Operator for Wireless FEC Accelerators              2.2.0                Succeeded
```

```shell
[user@ctrl1 /home]# oc get pod
NAME                                            READY   STATUS    RESTARTS   AGE                                                                              
                                           
sriov-device-plugin-hkq6f                       1/1     Running   0          35s                                                                              
sriov-fec-controller-manager-78488c4c65-cpknc   2/2     Running   0          44s                                                                              
sriov-fec-daemonset-7h8kb                       1/1     Running   0          35s                                                                              
```
### Configuration for telemetry

Openshift comes with pre-installed [kube-prometheus](https://github.com/prometheus-operator/kube-prometheus).

Assuming that operator is deployed in `vran-acceleration-operators` and Prometheus stack is deployed in `openshift-monitoring` namespace, you will have to apply following CRs:

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
  namespace: openshift-monitoring
---
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: bbdevconfig
  namespace: openshift-monitoring
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

If the operator has been previously installed, the user needs to perform the following steps to delete the operator deployment.

Use the following command to identify items to delete:

```shell
[user@ctrl1 /home]# oc get csv -n vran-acceleration-operators

NAME               DISPLAY                                             VERSION   REPLACES   PHASE
sriov-fec.v2.2.0   SR-IOV Operator for Wireless FEC Accelerators   2.2.0                Succeeded
```

```shell
[user@ctrl1 /home]# oc get subscription
NAME                     PACKAGE     SOURCE            CHANNEL
sriov-fec-subscription   sriov-fec   intel-operators   stable
```

Then delete the items and the namespace:

```shell
[user@ctrl1 /home]# oc delete csv sriov-fec.v2.2.0
[user@ctrl1 /home]# oc delete sub sriov-fec-subscription
[user@ctrl1 /home]# oc delete ns vran-acceleration-operators
```
