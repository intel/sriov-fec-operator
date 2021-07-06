```text
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2020-2021 Intel Corporation
```
<!-- omit in toc -->
# OpenNESS Operator for Wireless FEC Accelerators documentation

- [Overview](#overview)
- [OpenNESS Operator for Wireless FEC Accelerators](#openness-operator-for-wireless-fec-accelerators)
  - [Wireless FEC Acceleration management](#wireless-fec-acceleration-management)
    - [FEC Configuration](#fec-configuration)
    - [SRIOV Device Plugin](#sriov-device-plugin)
- [Managing NIC Devices](#managing-nic-devices)
- [Technical Requirements and Dependencies](#technical-requirements-and-dependencies)
- [Deploying the Operator](#deploying-the-operator)
  - [Install the Bundle](#install-the-bundle)
  - [Applying Custom Resources](#applying-custom-resources)
- [Hardware Validation Environment](#hardware-validation-environment)
- [Summary](#summary)
- [Appendix 1 - Developer Notes](#appendix-1---developer-notes)
  - [Uninstalling Previously Installed Operator](#uninstalling-previously-installed-operator)
  - [Setting Up Operator Registry Locally](#setting-up-operator-registry-locally)
- [Appendix 2 - OpenNESS Operator for Wireless FEC Accelerators Examples](#appendix-2---openness-operator-for-wireless-fec-accelerators-examples)
  - [N3000 FEC](#n3000-fec)
    - [Sample CR for Wireless FEC (N3000)](#sample-cr-for-wireless-fec-n3000)
    - [Sample Status for Wireless FEC (N3000)](#sample-status-for-wireless-fec-n3000)
    - [Sample Daemon log for Wireless FEC (N3000)](#sample-daemon-log-for-wireless-fec-n3000)
  - [ACC100 FEC](#acc100-fec)
    - [Sample CR for Wireless FEC (ACC100)](#sample-cr-for-wireless-fec-acc100)
    - [Sample Status for Wireless FEC (ACC100)](#sample-status-for-wireless-fec-acc100)
    - [Sample Daemon log for Wireless FEC (ACC100)](#sample-daemon-log-for-wireless-fec-acc100)

## Overview

This document provides the instructions for using the OpenNESS Operator for Wireless FEC Accelerators in Red Hat's OpenShift Container Platform. This operator was developed with aid of the Special Resource Operator framework based on the Operator SDK project.

## OpenNESS Operator for Wireless FEC Accelerators

The role of the OpenNESS Operator for Intel Wireless FEC Accelerator is to orchestrate and manage the resources/devices exposed by a range of Intel's vRAN FEC acceleration devices/hardware within the OpenShift cluster. The operator is a state machine which will configure the resources and then monitor them and act autonomously based on the user interaction.
The operator design of the OpenNESS Operator for Intel Wireless FEC Accelerator supports the following vRAN FEC accelerators:

* [Intel® PAC N3000 for vRAN Acceleration](https://github.com/otcshare/openshift-operator/blob/master/spec/vran-accelerators-supported-by-operator.md#intel-pac-n3000-for-vran-acceleration)
* [Intel® vRAN Dedicated Accelerator ACC100](https://github.com/otcshare/openshift-operator/blob/master/spec/vran-accelerators-supported-by-operator.md#intel-vran-dedicated-accelerator-acc100)

### Wireless FEC Acceleration management

This operator handles the management of the FEC devices used to accelerate the FEC process in vRAN L1 applications - the FEC devices are provided by a designated hardware (ie. FPGA or eASIC PCI extension cards). It provides functionality to create desired VFs (Virtual Functions) for the FEC device, binds them to appropriate drivers and configures the VF's queues for desired functionality in 4G or 5G deployment. It also deploys an instance of the SR-IOV Network device plugin which manages the FEC VFs as an OpenShift cluster resource and configures this device plugin to detect the resources. The user interacts with the operator by providing a CR (CustomResource). The operator constantly monitors the state of the CR to detect any changes and acts based on the changes detected. The CR is provided per cluster configuration. The components for individual nodes can be configured by specifying appropriate values for each component per "nodeName". Once the CR is applied or updated, the operator/daemon checks if the configuration is already applied, and, if not it binds the PFs to driver, creates desired amount of VFs, binds them to driver and runs the [pf-bb-config utility](https://github.com/intel/pf-bb-config) to configure the VF queues to the desired configuration.

This operator is a common operator for FEC device/resource management for a range on accelerator cards. For specific examples of CRs dedicated to single accelerator card only see:

* [Sample CR for Wireless FEC (N3000)](#sample-cr-for-wireless-fec-n3000)
* [Sample CR for Wireless FEC (ACC100)](#sample-cr-for-wireless-fec-acc100)

The workflow of the SRIOV FEC operator is shown in the following diagram:
![SRIOV FEC Operator Design](images/sriov_fec_operator_acc100.png)

#### FEC Configuration

The Intel's vRAN FEC acceleration devices/hardware expose the FEC PF device which is to be bound to PCI-PF-STUB driver in order to enable creation of the FEC VF devices. Once the FEC PF is bound to the correct driver, the user can create a number of devices to be used in Cloud Native deployment of vRAN to accelerate FEC. Once these devices are created they are to be bound to a user-space driver such as VFIO-PCI in order for them to work and be consumed in vRAN application pods. Before the device can be used by the application, the device needs to be configured - notably the mapping of queues exposed to the VFs - this is done via pf-bb-config application with the input from the CR used as a configuration.

> NOTE: For [Intel® vRAN Dedicated Accelerator ACC100](https://github.com/otcshare/openshift-operator/blob/master/spec/vran-accelerators-supported-by-operator.md#intel-vran-dedicated-accelerator-acc100) it is advised to create all 16 VFs. The card is configured to provide up to 8 queue groups with up to 16 queues per group. The queue groups can be divided between groups allocated to 5G/4G and Uplink/Downlink, it can be configured for 4G or 5G only, or both 4G and 5G at the same time. Each configured VF has access to all the queues. Each of the queue groups has a distinct priority level. The request for given queue group is made from application level (ie. vRAN application leveraging the FEC device).

> NOTE: For [Intel® PAC N3000 for vRAN Acceleration](https://github.com/otcshare/openshift-operator/blob/master/spec/vran-accelerators-supported-by-operator.md#intel-pac-n3000-for-vran-acceleration) the user can create up to 8 VF devices. Each FEC PF device provides a total of 64 queues to be configured, 32 queues for uplink and 32 queues for downlink. The queues would be typically distributed evenly across the VFs.

To get all the nodes containing one of the supported vRAN FEC accelerator devices run the following command (all the commands are run in the `vran-acceleration-operators` namespace):
```shell
[user@ctrl1 /home]# oc get sriovfecnodeconfig
NAME             CONFIGURED
node1            Succeeded
```

To find the PF of the SRIOV FEC accelerator device to be configured, run the following command:

```shell
[user@ctrl1 /home]# oc get sriovfecnodeconfig node1 -o yaml

***
status:
  conditions:
  - lastTransitionTime: "2021-03-19T17:19:37Z"
    message: Configured successfully
    observedGeneration: 1
    reason: ConfigurationSucceeded
    status: "True"
    type: Configured
  inventory:
    sriovAccelerators:
    - deviceID: 0d5c
      driver: ""
      maxVirtualFunctions: 16
      pciAddress: 0000:af:00.0
      vendorID: "8086"
      virtualFunctions: []
```

To configure the FEC device with desired setting create a CR (An example below configures ACC100's 8/8 queue groups for 5G, 4 queue groups for Uplink and another 4 queues groups for Downlink), for the list of CRs applicable to all supported devices see:

* [Sample CR for Wireless FEC (N3000)](#sample-cr-for-wireless-fec-n3000)
* [Sample CR for Wireless FEC (ACC100)](#sample-cr-for-wireless-fec-acc100)

```yaml
apiVersion: sriovfec.intel.com/v1
kind: SriovFecClusterConfig
metadata:
  name: config
spec:
  nodes:
    - nodeName: node1
      physicalFunctions:
        - pciAddress: 0000:af:00.0
          pfDriver: "pci-pf-stub"
          vfDriver: "vfio-pci"
          vfAmount: 16
          bbDevConfig:
            acc100:
              # Programming mode: 0 = VF Programming, 1 = PF Programming
              pfMode: false
              numVfBundles: 16
              maxQueueSize: 1024
              uplink4G:
                numQueueGroups: 0
                numAqsPerGroups: 16
                aqDepthLog2: 4
              downlink4G:
                numQueueGroups: 0
                numAqsPerGroups: 16
                aqDepthLog2: 4
              uplink5G:
                numQueueGroups: 4
                numAqsPerGroups: 16
                aqDepthLog2: 4
              downlink5G:
                numQueueGroups: 4
                numAqsPerGroups: 16
                aqDepthLog2: 4
```

To apply the CR run:

```shell
[user@ctrl1 /home]# oc apply -f <sriovfec_cr_name>.yaml
```

After creation of the CR, the SRIOV FEC daemon starts configuring the FEC device. Once the SRIOV FEC configuration is complete, the following status is reported:

```shell
[user@ctrl1 /home]# oc get sriovfecnodeconfig
NAME             CONFIGURED
node1            Succeeded
```

From SRIOV FEC daemon pod, the user should see logs similar to the output below, if the VF queues were successfully programmed. For a list of sample log outputs for applicable devices see:

* [Sample Daemon log for Wireless FEC (N3000)](#sample-daemon-log-for-wireless-fec-n3000)

* [Sample Daemon log for Wireless FEC (ACC100)](#sample-daemon-log-for-wireless-fec-acc100)

```shell
[user@ctrl1 /home]# oc get pod | grep sriov-fec-daemonset
sriov-fec-daemonset-h4jf8                      1/1     Running   0          19h

[user@ctrl1 /home]# oc logs sriov-fec-daemonset-h4jf8

***
{"level":"Level(-2)","ts":1616798129.251027,"logger":"daemon.drainhelper.cordonAndDrain()","msg":"node drained"}
{"level":"Level(-4)","ts":1616798129.2510319,"logger":"daemon.drainhelper.Run()","msg":"worker function - start"}
{"level":"Level(-4)","ts":1616798129.341839,"logger":"daemon.NodeConfigurator.applyConfig","msg":"current node status","inventory":{"sriovAccelerators":[{"vendorID":"8086","deviceID":"0b32","pciAddress":"0000:20:00.0","driver":"pci-pf-stub","maxVirtualFunctions":1,"virtualFunctions":[{"pciAddress":"0000:20:00.1","driver":"vfio-pci","deviceID":"0b33"}]},{"vendorID":"8086","deviceID":"0d5c","pciAddress":"0000:af:00.0","driver":"pci-pf-stub","maxVirtualFunctions":16,"virtualFunctions":[{"pciAddress":"0000:b0:00.0","driver":"vfio-pci","deviceID":"0d5d"},{"pciAddress":"0000:b0:00.1","driver":"vfio-pci","deviceID":"0d5d"},{"pciAddress":"0000:b0:00.2","driver":"vfio-pci","deviceID":"0d5d"},{"pciAddress":"0000:b0:00.3","driver":"vfio-pci","deviceID":"0d5d"}]}]}}
{"level":"Level(-4)","ts":1616798129.3419566,"logger":"daemon.NodeConfigurator.applyConfig","msg":"configuring PF","requestedConfig":{"pciAddress":"0000:20:00.0","pfDriver":"pci-pf-stub","vfDriver":"vfio-pci","vfAmount":1,"bbDevConfig":{"n3000":{"networkType":"FPGA_5GNR","pfMode":false,"flrTimeout":610,"downlink":{"bandwidth":3,"loadBalance":128,"queues":{"vf0":16,"vf1":16,"vf2":0,"vf3":0,"vf4":0,"vf5":0,"vf6":0,"vf7":0}},"uplink":{"bandwidth":3,"loadBalance":128,"queues":{"vf0":16,"vf1":16,"vf2":0,"vf3":0,"vf4":0,"vf5":0,"vf6":0,"vf7":0}}}}}}
{"level":"Level(-4)","ts":1616798129.3419993,"logger":"daemon.NodeConfigurator.loadModule","msg":"executing command","cmd":"/usr/sbin/chroot /host/ modprobe pci-pf-stub"}
{"level":"Level(-4)","ts":1616798129.3458664,"logger":"daemon.NodeConfigurator.loadModule","msg":"commands output","output":""}
{"level":"Level(-4)","ts":1616798129.345896,"logger":"daemon.NodeConfigurator.loadModule","msg":"executing command","cmd":"/usr/sbin/chroot /host/ modprobe vfio-pci"}
{"level":"Level(-4)","ts":1616798129.3490586,"logger":"daemon.NodeConfigurator.loadModule","msg":"commands output","output":""}
{"level":"Level(-2)","ts":1616798130.3972273,"logger":"daemon.NodeConfigurator","msg":"device is bound to driver","path":"/sys/bus/pci/devices/0000:20:00.0/driver"}
```

The user can observe the change of the cards FEC configuration. The created devices should appear similar to the following output (The '0d5c' is a PF of the FEC device and the '0d5d' is a VF of the FEC device). For a list of sample status output for applicable devices see:

* [Sample Status for Wireless FEC (N3000)](#sample-status-for-wireless-fec-n3000)

* [Sample Status for Wireless FEC (ACC100)](#sample-status-for-wireless-fec-acc100)

```yaml
[user@ctrl1 /home]# oc get sriovfecnodeconfig node1 -o yaml

***
status:
    conditions:
    - lastTransitionTime: "2021-03-19T11:46:22Z"
      message: Configured successfully
      observedGeneration: 1
      reason: Succeeded
      status: "True"
      type: Configured
    inventory:
      sriovAccelerators:
      - deviceID: 0d5c
        driver: pci-pf-stub
        maxVirtualFunctions: 16
        pciAddress: 0000:af:00.0
        vendorID: "8086"
        virtualFunctions:
        - deviceID: 0d5d
          driver: vfio-pci
          pciAddress: 0000:b0:00.0
        - deviceID: 0d5d
          driver: vfio-pci
          pciAddress: 0000:b0:00.1
        - deviceID: 0d5d
          driver: vfio-pci
          pciAddress: 0000:b0:00.2
        - deviceID: 0d5d
          driver: vfio-pci
          pciAddress: 0000:b0:00.3
        - deviceID: 0d5d
          driver: vfio-pci
          pciAddress: 0000:b0:00.4
```

#### SRIOV Device Plugin

As part of the SRIOV FEC operator the K8s SRIOV Network Device plugin is being deployed. The plugin is configured to detect the FEC devices only and is being configured according to the CR. This deployment of the SRIOV Network Device plugin does not manage non-FEC devices. For more information, refer to the documentation for [SRIOV Network Device plugin](https://github.com/openshift/sriov-network-device-plugin). After the deployment of the Operator and update/application of the CR, the user will be able to detect the FEC VFs as allocatable resources in the OpenShift cluster. The output should be similar to this (`intel.com/intel_fec_acc100` or alternative for a different FEC accelerator):

```shell
[user@node1 /home]# oc get node <node_name> -o json | jq '.status.allocatable'
{
  "cpu": "95500m",
  "ephemeral-storage": "898540920981",
  "hugepages-1Gi": "30Gi",
  "intel.com/intel_fec_acc100": "16",
  "memory": "115600160Ki",
  "pods": "250"
}
```

Once the SRIOV operator takes care of setting up and configuring the device, user can test the device using a sample 'test-bbdev' application from the [DPDK project (DPDK 20.11)](https://github.com/DPDK/dpdk/tree/v20.11/app/test-bbdev). An example of a prepared sample application's docker image can be found in [Intel® OpenNESS' project github EdgeApps repo](https://github.com/open-ness/edgeapps/tree/master/applications/fpga-sample-app). OpenNESS is an edge computing software toolkit that enables highly optimized and performant edge platforms to on-board and manage applications and network functions with cloud-like agility across any type of network. For more information, go to [www.openness.org](https://www.openness.org).

With a sample image of the DPDK application, the following pod can be created similar to the following file as an example (`intel.com/intel_fec_acc100` needs to be replaced as needed when different accelerator is used):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: pod-bbdev-sample-app
spec:
  containers:
  - securityContext:
      privileged: false
      capabilities:
        add:
          - IPC_LOCK
          - SYS_NICE
    name: bbdev-sample-app
    image: bbdev-sample-app:1.0
    command: [ "sudo", "/bin/bash", "-c", "--" ]
    args: [ "while true; do sleep 300000; done;" ]
    volumeMounts:
    - mountPath: /hugepages
      name: hugepage
    - name: class
      mountPath: /sys/devices
      readOnly: false
    resources:
      requests:
        intel.com/intel_fec_acc100: '1'
        hugepages-1Gi: 2Gi
        memory: 2Gi
      limits:
        intel.com/intel_fec_acc100: '1'
        hugepages-1Gi: 2Gi
        memory: 2Gi
  volumes:
  - name: hugepage
    emptyDir:
      medium: HugePages
  - hostPath:
      path: "/sys/devices"
    name: class
```

The pod consumes one of the FEC VF resources. Once the pod is created, user can detect the VF allocated to the pod by executing into pods terminal and running:

```shell
[user@ bbdev-sample-app /root]# printenv | grep INTEL_FEC
PCIDEVICE_INTEL_COM_INTEL_FEC_ACC100=0000:b0:00.0
```

With the PCIe B:D.F of the FEC VF allocated to the pod established, user will run the 'test-bbdev' application to test the device (similar output indicating that the tests are passing is expected):

```shell
[user@ bbdev-sample-app /root]# ./test-bbdev.py --testapp-path ./dpdk-test-bbdev -e="-w ${PCIDEVICE_INTEL_COM_INTEL_FEC_ACC100}" -i -n 1 -b 1 -l 1 -c validation -v ldpc_dec_v7813.data

Executing: ./dpdk-test-bbdev -w 0000:b0:00.0 -- -n 1 -l 1 -c validation -i -v ldpc_dec_v7813.data -b 1
EAL: Detected 96 lcore(s)
EAL: Detected 2 NUMA nodes
Option -w, --pci-whitelist is deprecated, use -a, --allow option instead
EAL: Multi-process socket /var/run/dpdk/rte/mp_socket
EAL: Selected IOVA mode 'VA'
EAL: Probing VFIO support...
EAL: VFIO support initialized
EAL:   using IOMMU type 1 (Type 1)
EAL: Probe PCI driver: intel_acc100_vf (8086:d5d) device: 0000:b0:00.0 (socket 1)
EAL: No legacy callbacks, legacy socket not created

 

===========================================================
Starting Test Suite : BBdev Validation Tests
Test vector file = ldpc_dec_v7813.data
Device 0 queue 16 setup failed
Allocated all queues (id=16) at prio0 on dev0
Device 0 queue 32 setup failed
Allocated all queues (id=32) at prio1 on dev0
Device 0 queue 48 setup failed
Allocated all queues (id=48) at prio2 on dev0
Device 0 queue 64 setup failed
Allocated all queues (id=64) at prio3 on dev0
Device 0 queue 64 setup failed
All queues on dev 0 allocated: 64
+ ------------------------------------------------------- +
== test: validation
dev:0000:b0:00.0, burst size: 1, num ops: 1, op type: RTE_BBDEV_OP_LDPC_DEC
Operation latency:
        avg: 23092 cycles, 10.0838 us
        min: 23092 cycles, 10.0838 us
        max: 23092 cycles, 10.0838 us
TestCase [ 0] : validation_tc passed
 + ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~ +
 + Test Suite Summary : BBdev Validation Tests
 + Tests Total :        1
 + Tests Skipped :      0
 + Tests Passed :       1
 + Tests Failed :       0
 + Tests Lasted :       177.67 ms
 + ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~ +
```

## Managing NIC Devices

The management of the NIC SRIOV devices/resources in the OpenShift cluster is out of scope of this operator. The user is expected to deploy an operator/[SRIOV Network Device plugin](https://github.com/openshift/sriov-network-device-plugin) which will handle the orchestration of SRIOV NIC VFs between pods.

## Technical Requirements and Dependencies

The OpenNESS Operator for Wireless FEC Accelerators has the following requirements:

- [Intel® vRAN Dedicated Accelerator ACC100](https://builders.intel.com/docs/networkbuilders/intel-vran-dedicated-accelerator-acc100-product-brief.pdf) (Optional)
- [Intel® FPGA PAC N3000 card](https://www.intel.com/content/www/us/en/programmable/products/boards_and_kits/dev-kits/altera/intel-fpga-pac-n3000/overview.html) (Optional)
- [OpenShift 4.7.8](https://docs.openshift.com/container-platform/4.7/release_notes/ocp-4-7-release-notes.html)
- RT Kernel configured with [Performance Addon Operator](https://access.redhat.com/documentation/en-us/openshift_container_platform/4.6/html/scalability_and_performance/cnf-performance-addon-operator-for-low-latency-nodes).

## Deploying the Operator

The OpenNESS Operator for Wireless FEC Accelerators is easily deployable from the OpenShift or Kubernetes cluster via provisioning and application of the following YAML spec files:

### Install dependencies
If operator is being installed on Kubernetes then run steps marked as (KUBERNETES).
If operator is being installed on Openshift run only (OCP) steps.
(KUBERNETES) Create configmap:
```shell
[user@ctrl1 /home]# cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: vran-acceleration-operators
  name: operator-configuration
data:
  isGeneric: "true"
EOF
```
(KUBERNETES) If Kubernetes doesn't have installed OLM (Operator lifecycle manager) start from installing Operator-sdk (https://olm.operatorframework.io/)
After Operator-sdk installation run following command
```shell
[user@ctrl1 /home]# operator-sdk olm install
```
(KUBERNETES) Install PCIutils on worker nodes
```shell
[user@ctrl1 /home]#  yum install pciutils
```
### Install the Bundle

To install the OpenNESS Operator for Wireless FEC Accelerators operator bundle perform the following steps:

(OCP) Create the project:
```shell
[user@ctrl1 /home]# oc new-project vran-acceleration-operators
```
(KUBERNETES) Create the project:
```shell
[user@ctrl1 /home]# kubectl create namespace vran-acceleration-operators
[user@ctrl1 /home]# kubectl config set-context --current --namespace=vran-acceleration-operators
```
Execute following commands on both OCP and KUBERNETES cluster:

(KUBERNETES) In commands below use `kubectl` instead of `oc`

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
sriov-fec.v1.1.0   OpenNESS SR-IOV Operator for Wireless FEC Accelerators   1.1.0                Succeeded
```

```shell
[user@ctrl1 /home]# oc get pod
NAME                                            READY   STATUS    RESTARTS   AGE                                                                              
                                           
sriov-device-plugin-hkq6f                       1/1     Running   0          35s                                                                              
sriov-fec-controller-manager-78488c4c65-cpknc   2/2     Running   0          44s                                                                              
sriov-fec-daemonset-7h8kb                       1/1     Running   0          35s                                                                              
```

### Applying Custom Resources

Once the operator is successfully deployed, the user interacts with it by creating CRs which will be interpreted by the operators, for examples of CRs see the following section:
- [FEC Configuration](#fec-configuration)

To apply a CR run:

```shell
[user@ctrl1 /home]# oc apply -f <cr-name>
```

To view the status of current CR run (sample output):

```shell
[user@ctrl1 /home]# oc get sriovfecclusterconfig config -o yaml
***
spec:
  nodes:
  - nodeName: node1
    physicalFunctions:
    - bbDevConfig:
        acc100:
          downlink4G:
            aqDepthLog2: 4
            numAqsPerGroups: 16
            numQueueGroups: 0
          downlink5G:
            aqDepthLog2: 4
            numAqsPerGroups: 16
            numQueueGroups: 4
          maxQueueSize: 1024
          numVfBundles: 16
          pfMode: false
          uplink4G:
            aqDepthLog2: 4
            numAqsPerGroups: 16
            numQueueGroups: 0
          uplink5G:
            aqDepthLog2: 4
            numAqsPerGroups: 16
            numQueueGroups: 4
      pciAddress: 0000:af:00.0
      pfDriver: pci-pf-stub
      vfAmount: 16
      vfDriver: vfio-pci
status:
  syncStatus: Succeeded
```

## Hardware Validation Environment

- Intel® vRAN Dedicated Accelerator ACC100
- Intel® FPGA PAC N3000-2
- Intel® FPGA PAC N3000-N
- 2nd Generation Intel® Xeon® processor platform

## Summary

The OpenNESS Operator for Wireless FEC Accelerators is a fully functional tool to manage the vRAN FEC resources autonomously in a Cloud Native OpenShift environment based on the user input.
The operator handles all the necessary actions from creation of FEC resources to configuration and management of the resources within the OpenShift cluster.

## Appendix 1 - Developer Notes

### Uninstalling Previously Installed Operator

If the operator has been previously installed, the user needs to perform the following steps to delete the operator deployment.

Use the following command to identify items to delete:

```shell
[user@ctrl1 /home]# oc get csv -n vran-acceleration-operators

NAME               DISPLAY                                        VERSION   REPLACES   PHASE
sriov-fec.v1.1.0   SRIOV-FEC Operator for Intel® FPGA PAC N3000   1.1.0                Succeeded
```

Then delete the items and the namespace:

```shell
[user@ctrl1 /home]# oc delete csv sriov-fec.v1.1.0
[user@ctrl1 /home]# oc delete ns vran-acceleration-operators
```

### Setting Up Operator Registry Locally

If needed the user can set up a local registry for the operators' images. For more information please see [openshift-pacn3000-operator.md](https://github.com/otcshare/openshift-operator/blob/master/spec/openshift-pacn3000-operator.md#setting-up-operator-registry-locally)

## Appendix 2 - OpenNESS Operator for Wireless FEC Accelerators Examples

### N3000 FEC

#### Sample CR for Wireless FEC (N3000)

```yaml
apiVersion: sriovfec.intel.com/v1
kind: SriovFecClusterConfig
metadata:
  name: config
  namespace: vran-acceleration-operators
spec:
  nodes:
    - nodeName: node1
      physicalFunctions:
        - pciAddress: 0000.1d.00.0
          pfDriver: pci-pf-stub
          vfDriver: vfio-pci
          vfAmount: 2
          bbDevConfig:
            n3000:
              # Network Type: either "FPGA_5GNR" or "FPGA_LTE"
              networkType: "FPGA_5GNR"
              # Programming mode: 0 = VF Programming, 1 = PF Programming
              pfMode: false
              flrTimeout: 610
              downlink:
                bandwidth: 3
                loadBalance: 128
                queues:
                  vf0: 16
                  vf1: 16
                  vf2: 0
                  vf3: 0
                  vf4: 0
                  vf5: 0
                  vf6: 0
                  vf7: 0
              uplink:
                bandwidth: 3
                loadBalance: 128
                queues:
                  vf0: 16
                  vf1: 16
                  vf2: 0
                  vf3: 0
                  vf4: 0
                  vf5: 0
                  vf6: 0
                  vf7: 0
```

#### Sample Status for Wireless FEC (N3000)

```yaml
status:
  conditions:
  - lastTransitionTime: "2020-12-15T17:19:37Z"
    message: Configured successfully
    observedGeneration: 1
    reason: ConfigurationSucceeded
    status: "True"
    type: Configured
  inventory:
    sriovAccelerators:
    - deviceID: 0d8f
      driver: pci-pf-stub
      maxVirtualFunctions: 8
      pciAddress: 0000:1d:00.0
      vendorID: "8086"
      virtualFunctions:
      - deviceID: 0d90
        driver: vfio-pci
        pciAddress: 0000:1d:00.1
      - deviceID: 0d90
        driver: vfio-pci
        pciAddress: 0000:1d:00.2
```

#### Sample Daemon log for Wireless FEC (N3000)

```shell
2020-12-16T12:46:47.720Z        INFO    daemon.NodeConfigurator.applyConfig     configuring PF  {"requestedConfig": {"pciAddress":"0000:1d:00.0","pfDriver":"pci-pf-stub","vfDriver":"vfio-pci","vfAmount":2,"bbDevConfig":{"n3000":{
"networkType":"FPGA_5GNR","pfMode":false,"flrTimeout":610,"downlink":{"bandwidth":3,"loadBalance":128,"queues":{"vf0":16,"vf1":16}},"uplink":{"bandwidth":3,"loadBalance":128,"queues":{"vf0":16,"vf1":16}}}}}}                      
2020-12-16T12:46:47.720Z        INFO    daemon.NodeConfigurator.loadModule      executing command       {"cmd": "/usr/sbin/chroot /host/ modprobe pci-pf-stub"}                                                                      
2020-12-16T12:46:47.724Z        INFO    daemon.NodeConfigurator.loadModule      commands output {"output": ""}                                                                                                                       
2020-12-16T12:46:47.724Z        INFO    daemon.NodeConfigurator.loadModule      executing command       {"cmd": "/usr/sbin/chroot /host/ modprobe vfio-pci"}                                                                         
2020-12-16T12:46:47.727Z        INFO    daemon.NodeConfigurator.loadModule      commands output {"output": ""}                                                                                                                       
2020-12-16T12:46:47.727Z        INFO    daemon.NodeConfigurator device's driver_override path   {"path": "/sys/bus/pci/devices/0000:1d:00.0/driver_override"}                                                                        
2020-12-16T12:46:47.727Z        INFO    daemon.NodeConfigurator driver bind path        {"path": "/sys/bus/pci/drivers/pci-pf-stub/bind"}                                                                                            
2020-12-16T12:46:47.998Z        INFO    daemon.NodeConfigurator device's driver_override path   {"path": "/sys/bus/pci/devices/0000:1d:00.1/driver_override"}                                                                        
2020-12-16T12:46:47.998Z        INFO    daemon.NodeConfigurator driver bind path        {"path": "/sys/bus/pci/drivers/vfio-pci/bind"}                                                                                               
2020-12-16T12:46:47.998Z        INFO    daemon.NodeConfigurator device's driver_override path   {"path": "/sys/bus/pci/devices/0000:1d:00.2/driver_override"}                                                                        
2020-12-16T12:46:47.998Z        INFO    daemon.NodeConfigurator driver bind path        {"path": "/sys/bus/pci/drivers/vfio-pci/bind"}                                                                                               
2020-12-16T12:46:47.999Z        INFO    daemon.NodeConfigurator.applyConfig     executing command       {"cmd": "/sriov_workdir/pf_bb_config FPGA_5GNR -c /sriov_artifacts/0000:1d:00.0.ini -p 0000:1d:00.0"}                        
2020-12-16T12:46:48.017Z        INFO    daemon.NodeConfigurator.applyConfig     commands output {"output": "ERROR: Section (FLR) or name (flr_time_out) is not valid.
FEC FPGA RTL v3.0
UL.DL Weights = 3.3
UL.DL Load Balance = 1
28.128
Queue-PF/VF Mapping Table = READY
Ring Descriptor Size = 256 bytes

--------+-----+-----+-----+-----+-----+-----+-----+-----+-----+
        |  PF | VF0 | VF1 | VF2 | VF3 | VF4 | VF5 | VF6 | VF7 |
--------+-----+-----+-----+-----+-----+-----+-----+-----+-----+
UL-Q'00 |     |  X  |     |     |     |     |     |     |     |
UL-Q'01 |     |  X  |     |     |     |     |     |     |     |
UL-Q'02 |     |  X  |     |     |     |     |     |     |     |
UL-Q'03 |     |  X  |     |     |     |     |     |     |     |
UL-Q'04 |     |  X  |     |     |     |     |     |     |     |
UL-Q'05 |     |  X  |     |     |     |     |     |     |     |
UL-Q'06 |     |  X  |     |     |     |     |     |     |     |
UL-Q'07 |     |  X  |     |     |     |     |     |     |     |
UL-Q'08 |     |  X  |     |     |     |     |     |     |     |
UL-Q'09 |     |  X  |     |     |     |     |     |     |     |
UL-Q'10 |     |  X  |     |     |     |     |     |     |     |
UL-Q'11 |     |  X  |     |     |     |     |     |     |     |
UL-Q'12 |     |  X  |     |     |     |     |     |     |     |
UL-Q'13 |     |  X  |     |     |     |     |     |     |     |
UL-Q'14 |     |  X  |     |     |     |     |     |     |     |
UL-Q'15 |     |  X  |     |     |     |     |     |     |     |
UL-Q'16 |     |     |  X  |     |     |     |     |     |     |
UL-Q'17 |     |     |  X  |     |     |     |     |     |     |
UL-Q'18 |     |     |  X  |     |     |     |     |     |     |
UL-Q'19 |     |     |  X  |     |     |     |     |     |     |
UL-Q'20 |     |     |  X  |     |     |     |     |     |     |
UL-Q'21 |     |     |  X  |     |     |     |     |     |     |
UL-Q'22 |     |     |  X  |     |     |     |     |     |     |
UL-Q'23 |     |     |  X  |     |     |     |     |     |     |
UL-Q'24 |     |     |  X  |     |     |     |     |     |     |
UL-Q'25 |     |     |  X  |     |     |     |     |     |     |
UL-Q'26 |     |     |  X  |     |     |     |     |     |     |
UL-Q'27 |     |     |  X  |     |     |     |     |     |     |
UL-Q'28 |     |     |  X  |     |     |     |     |     |     |
UL-Q'29 |     |     |  X  |     |     |     |     |     |     |
UL-Q'30 |     |     |  X  |     |     |     |     |     |     |
UL-Q'31 |     |     |  X  |     |     |     |     |     |     |
DL-Q'32 |     |  X  |     |     |     |     |     |     |     |
DL-Q'33 |     |  X  |     |     |     |     |     |     |     |
DL-Q'34 |     |  X  |     |     |     |     |     |     |     |
DL-Q'35 |     |  X  |     |     |     |     |     |     |     |
DL-Q'36 |     |  X  |     |     |     |     |     |     |     |
DL-Q'37 |     |  X  |     |     |     |     |     |     |     |
DL-Q'38 |     |  X  |     |     |     |     |     |     |     |
DL-Q'39 |     |  X  |     |     |     |     |     |     |     |
DL-Q'40 |     |  X  |     |     |     |     |     |     |     |
DL-Q'41 |     |  X  |     |     |     |     |     |     |     |
DL-Q'42 |     |  X  |     |     |     |     |     |     |     |
DL-Q'43 |     |  X  |     |     |     |     |     |     |     |
DL-Q'44 |     |  X  |     |     |     |     |     |     |     |
DL-Q'45 |     |  X  |     |     |     |     |     |     |     |
DL-Q'46 |     |  X  |     |     |     |     |     |     |     |
DL-Q'47 |     |  X  |     |     |     |     |     |     |     |
DL-Q'48 |     |     |  X  |     |     |     |     |     |     |
DL-Q'49 |     |     |  X  |     |     |     |     |     |     |
DL-Q'50 |     |     |  X  |     |     |     |     |     |     |
DL-Q'51 |     |     |  X  |     |     |     |     |     |     |
DL-Q'52 |     |     |  X  |     |     |     |     |     |     |
DL-Q'53 |     |     |  X  |     |     |     |     |     |     |
DL-Q'54 |     |     |  X  |     |     |     |     |     |     |
DL-Q'55 |     |     |  X  |     |     |     |     |     |     |
DL-Q'56 |     |     |  X  |     |     |     |     |     |     |
DL-Q'57 |     |     |  X  |     |     |     |     |     |     |
DL-Q'58 |     |     |  X  |     |     |     |     |     |     |
DL-Q'59 |     |     |  X  |     |     |     |     |     |     |
DL-Q'60 |     |     |  X  |     |     |     |     |     |     |
DL-Q'61 |     |     |  X  |     |     |     |     |     |     |
DL-Q'62 |     |     |  X  |     |     |     |     |     |     |
DL-Q'63 |     |     |  X  |     |     |     |     |     |     |
--------+-----+-----+-----+-----+-----+-----+-----+-----+-----+

Mode of operation = VF-mode
FPGA_5GNR PF [0000:1d:00.0] configuration complete!"}
2020-12-16T12:46:48.017Z        INFO    daemon.NodeConfigurator.enableMasterBus executing command       {"cmd": "/usr/sbin/chroot /host/ setpci -v -s 0000:1d:00.0 COMMAND"}
2020-12-16T12:46:48.037Z        INFO    daemon.NodeConfigurator.enableMasterBus commands output {"output": "0000:1d:00.0 @04 = 0102\n"}
2020-12-16T12:46:48.037Z        INFO    daemon.NodeConfigurator.enableMasterBus executing command       {"cmd": "/usr/sbin/chroot /host/ setpci -v -s 0000:1d:00.0 COMMAND=0106"}
2020-12-16T12:46:48.054Z        INFO    daemon.NodeConfigurator.enableMasterBus commands output {"output": "0000:1d:00.0 @04 0106\n"}
2020-12-16T12:46:48.054Z        INFO    daemon.NodeConfigurator.enableMasterBus MasterBus set   {"pci": "0000:1d:00.0", "output": "0000:1d:00.0 @04 0106\n"}
2020-12-16T12:46:48.160Z        INFO    daemon.drainhelper.Run()        worker function - end   {"performUncordon": true}
```

### ACC100 FEC

#### Sample CR for Wireless FEC (ACC100)

```yaml
apiVersion: sriovfec.intel.com/v1
kind: SriovFecClusterConfig
metadata:
  name: config
spec:
  nodes:
    - nodeName: node1
      physicalFunctions:
        - pciAddress: 0000:af:00.0
          pfDriver: "pci-pf-stub"
          vfDriver: "vfio-pci"
          vfAmount: 16
          bbDevConfig:
            acc100:
              # Programming mode: 0 = VF Programming, 1 = PF Programming
              pfMode: false
              numVfBundles: 16
              maxQueueSize: 1024
              uplink4G:
                numQueueGroups: 0
                numAqsPerGroups: 16
                aqDepthLog2: 4
              downlink4G:
                numQueueGroups: 0
                numAqsPerGroups: 16
                aqDepthLog2: 4
              uplink5G:
                numQueueGroups: 4
                numAqsPerGroups: 16
                aqDepthLog2: 4
              downlink5G:
                numQueueGroups: 4
                numAqsPerGroups: 16
                aqDepthLog2: 4
```

#### Sample Status for Wireless FEC (ACC100)

```yaml
status:
    conditions:
    - lastTransitionTime: "2021-03-19T11:46:22Z"
      message: Configured successfully
      observedGeneration: 1
      reason: Succeeded
      status: "True"
      type: Configured
    inventory:
      sriovAccelerators:
      - deviceID: 0d5c
        driver: pci-pf-stub
        maxVirtualFunctions: 16
        pciAddress: 0000:af:00.0
        vendorID: "8086"
        virtualFunctions:
        - deviceID: 0d5d
          driver: vfio-pci
          pciAddress: 0000:b0:00.0
        - deviceID: 0d5d
          driver: vfio-pci
          pciAddress: 0000:b0:00.1
        - deviceID: 0d5d
          driver: vfio-pci
          pciAddress: 0000:b0:00.2
        - deviceID: 0d5d
          driver: vfio-pci
          pciAddress: 0000:b0:00.3
        - deviceID: 0d5d
          driver: vfio-pci
          pciAddress: 0000:b0:00.4
```

#### Sample Daemon log for Wireless FEC (ACC100)

```shell
{"level":"Level(-2)","ts":1616794345.4786215,"logger":"daemon.drainhelper.cordonAndDrain()","msg":"node drained"}
{"level":"Level(-4)","ts":1616794345.4786265,"logger":"daemon.drainhelper.Run()","msg":"worker function - start"}
{"level":"Level(-4)","ts":1616794345.5762916,"logger":"daemon.NodeConfigurator.applyConfig","msg":"current node status","inventory":{"sriovAccelerat
ors":[{"vendorID":"8086","deviceID":"0b32","pciAddress":"0000:20:00.0","driver":"","maxVirtualFunctions":1,"virtualFunctions":[]},{"vendorID":"8086"
,"deviceID":"0d5c","pciAddress":"0000:af:00.0","driver":"","maxVirtualFunctions":16,"virtualFunctions":[]}]}}
{"level":"Level(-4)","ts":1616794345.5763638,"logger":"daemon.NodeConfigurator.applyConfig","msg":"configuring PF","requestedConfig":{"pciAddress":"
0000:af:00.0","pfDriver":"pci-pf-stub","vfDriver":"vfio-pci","vfAmount":2,"bbDevConfig":{"acc100":{"pfMode":false,"numVfBundles":16,"maxQueueSize":1
024,"uplink4G":{"numQueueGroups":4,"numAqsPerGroups":16,"aqDepthLog2":4},"downlink4G":{"numQueueGroups":4,"numAqsPerGroups":16,"aqDepthLog2":4},"uplink5G":{"numQueueGroups":0,"numAqsPerGroups":16,"aqDepthLog2":4},"downlink5G":{"numQueueGroups":0,"numAqsPerGroups":16,"aqDepthLog2":4}}}}}
{"level":"Level(-4)","ts":1616794345.5774765,"logger":"daemon.NodeConfigurator.loadModule","msg":"executing command","cmd":"/usr/sbin/chroot /host/ modprobe pci-pf-stub"}
{"level":"Level(-4)","ts":1616794345.5842702,"logger":"daemon.NodeConfigurator.loadModule","msg":"commands output","output":""}
{"level":"Level(-4)","ts":1616794345.5843055,"logger":"daemon.NodeConfigurator.loadModule","msg":"executing command","cmd":"/usr/sbin/chroot /host/ modprobe vfio-pci"}
{"level":"Level(-4)","ts":1616794345.6090655,"logger":"daemon.NodeConfigurator.loadModule","msg":"commands output","output":""}
{"level":"Level(-2)","ts":1616794345.6091156,"logger":"daemon.NodeConfigurator","msg":"device's driver_override path","path":"/sys/bus/pci/devices/0000:af:00.0/driver_override"}
{"level":"Level(-2)","ts":1616794345.6091807,"logger":"daemon.NodeConfigurator","msg":"driver bind path","path":"/sys/bus/pci/drivers/pci-pf-stub/bind"}
{"level":"Level(-2)","ts":1616794345.7488534,"logger":"daemon.NodeConfigurator","msg":"device's driver_override path","path":"/sys/bus/pci/devices/0000:b0:00.0/driver_override"}
{"level":"Level(-2)","ts":1616794345.748938,"logger":"daemon.NodeConfigurator","msg":"driver bind path","path":"/sys/bus/pci/drivers/vfio-pci/bind"}
{"level":"Level(-2)","ts":1616794345.7492096,"logger":"daemon.NodeConfigurator","msg":"device's driver_override path","path":"/sys/bus/pci/devices/0000:b0:00.1/driver_override"}
{"level":"Level(-2)","ts":1616794345.7492566,"logger":"daemon.NodeConfigurator","msg":"driver bind path","path":"/sys/bus/pci/drivers/vfio-pci/bind"}
{"level":"Level(-4)","ts":1616794345.74968,"logger":"daemon.NodeConfigurator.applyConfig","msg":"executing command","cmd":"/sriov_workdir/pf_bb_config ACC100 -c /sriov_artifacts/0000:af:00.0.ini -p 0000:af:00.0"}
{"level":"Level(-4)","ts":1616794346.5203931,"logger":"daemon.NodeConfigurator.applyConfig","msg":"commands output","output":"Queue Groups: 0 5GUL, 0 5GDL, 4 4GUL, 4 4GDL\nNumber of 5GUL engines 8\nConfiguration in VF mode\nPF ACC100 configuration complete\nACC100 PF [0000:af:00.0] configuration complete!\n\n"}
{"level":"Level(-4)","ts":1616794346.520459,"logger":"daemon.NodeConfigurator.enableMasterBus","msg":"executing command","cmd":"/usr/sbin/chroot /host/ setpci -v -s 0000:af:00.0 COMMAND"}
{"level":"Level(-4)","ts":1616794346.5458736,"logger":"daemon.NodeConfigurator.enableMasterBus","msg":"commands output","output":"0000:af:00.0 @04 = 0142\n"}
{"level":"Level(-4)","ts":1616794346.5459251,"logger":"daemon.NodeConfigurator.enableMasterBus","msg":"executing command","cmd":"/usr/sbin/chroot /host/ setpci -v -s 0000:af:00.0 COMMAND=0146"}
{"level":"Level(-4)","ts":1616794346.5795262,"logger":"daemon.NodeConfigurator.enableMasterBus","msg":"commands output","output":"0000:af:00.0 @04 0146\n"}
{"level":"Level(-2)","ts":1616794346.5795407,"logger":"daemon.NodeConfigurator.enableMasterBus","msg":"MasterBus set","pci":"0000:af:00.0","output":"0000:af:00.0 @04 0146\n"}
{"level":"Level(-4)","ts":1616794346.6867144,"logger":"daemon.drainhelper.Run()","msg":"worker function - end","performUncordon":true}
{"level":"Level(-4)","ts":1616794346.6867719,"logger":"daemon.drainhelper.Run()","msg":"uncordoning node"}
{"level":"Level(-4)","ts":1616794346.6896322,"logger":"daemon.drainhelper.uncordon()","msg":"starting uncordon attempts"}
{"level":"Level(-2)","ts":1616794346.69735,"logger":"daemon.drainhelper.uncordon()","msg":"node uncordoned"}
{"level":"Level(-4)","ts":1616794346.6973662,"logger":"daemon.drainhelper.Run()","msg":"cancelling the context to finish the leadership"}
{"level":"Level(-4)","ts":1616794346.7029872,"logger":"daemon.drainhelper.Run()","msg":"stopped leading"}
{"level":"Level(-4)","ts":1616794346.7030034,"logger":"daemon.drainhelper","msg":"releasing the lock (bug mitigation)"}
{"level":"Level(-4)","ts":1616794346.8040674,"logger":"daemon.updateInventory","msg":"obtained inventory","inv":{"sriovAccelerators":[{"vendorID":"8086","deviceID":"0b32","pciAddress":"0000:20:00.0","driver":"","maxVirtualFunctions":1,"virtualFunctions":[]},{"vendorID":"8086","deviceID":"0d5c","pciAddress":"0000:af:00.0","driver":"pci-pf-stub","maxVirtualFunctions":16,"virtualFunctions":[{"pciAddress":"0000:b0:00.0","driver":"vfio-pci","deviceID":"0d5d"},{"pciAddress":"0000:b0:00.1","driver":"vfio-pci","deviceID":"0d5d"}]}]}}
{"level":"Level(-4)","ts":1616794346.9058325,"logger":"daemon","msg":"Update ignored, generation unchanged"}
{"level":"Level(-2)","ts":1616794346.9065044,"logger":"daemon.Reconcile","msg":"Reconciled","namespace":"vran-acceleration-operators","name":"pg-itengdvs02r.altera.com"}
```
