## Technical Requirements and Dependencies

The SEO Operator for Wireless FEC Accelerators has the following requirements:

- [Intel® vRAN Dedicated Accelerator ACC100](https://builders.intel.com/docs/networkbuilders/intel-vran-dedicated-accelerator-acc100-product-brief.pdf)
- [OpenShift 4.10.x](https://docs.openshift.com/container-platform/4.10/release_notes/ocp-4-10-release-notes.html)
- RT Kernel configured with [Performance Addon Operator](https://access.redhat.com/documentation/en-us/openshift_container_platform/4.6/html/scalability_and_performance/cnf-performance-addon-operator-for-low-latency-nodes).
- sriov-fec:2.3.0 comes with initial support of `vfio-pci` driver for ACC100. Configurations leveraging `vfio-pci` require following kernel parameters:
    - vfio_pci.enable_sriov=1
    - vfio_pci.disable_idle_d3=1


### Install the Bundle

To install the SEO Operator for Wireless FEC Accelerators operator bundle perform the following steps:

Create the project:
```shell
[user@ctrl1 /home]# oc new-project vran-acceleration-operators
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
sriov-fec.v2.2.0   SEO SR-IOV Operator for Wireless FEC Accelerators              2.2.0                Succeeded
```

```shell
[user@ctrl1 /home]# oc get pod
NAME                                            READY   STATUS    RESTARTS   AGE                                                                              
                                           
sriov-device-plugin-hkq6f                       1/1     Running   0          35s                                                                              
sriov-fec-controller-manager-78488c4c65-cpknc   2/2     Running   0          44s                                                                              
sriov-fec-daemonset-7h8kb                       1/1     Running   0          35s                                                                              
```