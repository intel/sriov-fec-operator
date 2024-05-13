```text
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2020-2024 Intel Corporation
```
<!-- omit in toc -->
# SRIOV-FEC Operator for Wireless FEC Accelerators

- [Overview](#overview)
- [SRIOV-FEC Operator for Wireless FEC Accelerators](#sriov-fec-operator-for-wireless-fec-accelerators-1)
  - [Wireless FEC Acceleration management](#wireless-fec-acceleration-management)
    - [FEC Configuration](#fec-configuration)
    - [SRIOV Device Plugin](#sriov-device-plugin)
- [VFIO\_PCI Driver](#vfio_pci-driver)
  - [Secure Boot](#secure-boot)
  - [Vfio Token](#vfio-token)
- [Deploying the Operator](#deploying-the-operator)
  - [Applying Custom Resources](#applying-custom-resources)
  - [Telemetry](#telemetry)
- [Hardware Validation Environment](#hardware-validation-environment)
- [Summary](#summary)
- [Appendix 1 - Developer Notes](#appendix-1---developer-notes)
  - [Running operator on SNO](#running-operator-on-sno)
- [Appendix 2 - SRIOV-FEC Operator for Wireless FEC Accelerators Examples](#appendix-2---sriov-fec-operator-for-wireless-fec-accelerators-examples)
  - [Intel® vRAN Dedicated Accelerator ACC100](#intel-vran-dedicated-accelerator-acc100)
    - [Sample CR for ACC100](#sample-cr-for-acc100)
    - [Sample Status for ACC100](#sample-status-for-acc100)
    - [Resource name for ACC100](#resource-name-for-acc100)
    - [Sample Daemon log for ACC100](#sample-daemon-log-for-acc100)
  - [Intel® vRAN Boost accelerator VRB1(ACC200)](#intel-vran-boost-accelerator-vrb1acc200)
    - [Sample CR for VRB1(ACC200)](#sample-cr-for-vrb1acc200)
      - [sriov-fec CR API](#sriov-fec-cr-api)
      - [sriov-vrb CR API](#sriov-vrb-cr-api)
    - [Sample Status for VRB1(ACC200)](#sample-status-for-vrb1acc200)
    - [Resource name for VRB1](#resource-name-for-vrb1)
  - [Intel® vRAN Boost accelerator VRB2](#intel-vran-boost-accelerator-vrb2)
    - [Sample CR for VRB2](#sample-cr-for-vrb2)
    - [Sample Status for VRB2](#sample-status-for-vrb2)
    - [Resource name for VRB2](#resource-name-for-vrb2)
- [Appendix 3 - Update srs\_fft\_windows\_coeffient file](#appendix-3---update-srs_fft_windows_coeffient-file)
- [Appendix 4 - Gathering logs for bug report](#appendix-4---gathering-logs-for-bug-report)

## Overview

This document provides the instructions for using the SRIOV-FEC Operator for Wireless FEC Accelerators in Red Hat's OpenShift Container Platform and Kubernetes. This operator was developed with aid of the Operator SDK project.

## SRIOV-FEC Operator for Wireless FEC Accelerators

The role of the SRIOV-FEC Operator for Intel® vRAN Boost Accelerator is to orchestrate and manage the resources/devices exposed by a range of Intel® vRAN Boost acceleration devices/hardware within a Kubernetes cluster. The operator is a state machine which will configure the resources and then monitor them and act autonomously based on the user interaction.
The operator design of the SRIOV-FEC Operator supports the following Intel® vRAN Boost accelerators:

* [Intel® vRAN Dedicated Accelerator ACC100](https://github.com/intel/sriov-fec-operator/blob/main/spec/vran-accelerators-supported-by-operator.md#intel-vran-dedicated-accelerator-acc100)
* [Intel® vRAN Boost v1.0 (VRB1) Accelerator](https://github.com/intel/sriov-fec-operator/blob/main/spec/vran-accelerators-supported-by-operator.md#intel-vran-boost-v10-vrb1-accelerator)
* [Intel® vRAN Boost v2.0 (VRB2) Accelerator](https://github.com/intel/sriov-fec-operator/blob/main/spec/vran-accelerators-supported-by-operator.md#intel-vran-boost-v20-vrb2-accelerator)


### Wireless FEC Acceleration management

This operator handles the management of the FEC devices used to accelerate the FEC process in vRAN L1 applications - the FEC devices are provided by a designated hardware (ie. Intel® vRAN Dedicated Accelerator ACC100).
It provides functionality to create desired VFs (Virtual Functions) for the FEC device, binds them to appropriate drivers and configures the VF's queues for desired functionality in 4G or 5G deployment. 
It also deploys an instance of the [SR-IOV Network Device Plugin](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin) which manages the FEC VFs as an OpenShift cluster resource and configures this device plugin to detect the resources. 
The user interacts with the operator by providing a CR (CustomResource). 
The operator constantly monitors the state of the CR to detect any changes and acts based on the changes detected. 
The CR is provided per cluster configuration. The components for individual nodes can be configured by specifying appropriate values for each component per "nodeSelector".
Once the CR is applied or updated, the operator/daemon checks if the configuration is already applied, and, if not it binds the PFs to driver, creates desired amount of VFs, binds them to driver and runs the [pf-bb-config utility](https://github.com/intel/pf-bb-config) to configure the VF queues to the desired configuration.

This operator is a common operator for management for a range on Intel® vRAN Boost accelerator cards. For specific examples of CRs dedicated to single accelerator card only see:

* [Sample CR for Wireless FEC (ACC100)](#sample-cr-for-wireless-fec-acc100)

The workflow of the SRIOV FEC operator is shown in the following diagram:
![SRIOV FEC Operator Design](images/sriov_fec_operator_acc100.png)

#### FEC Configuration

The Intel's vRAN FEC acceleration devices/hardware expose the FEC PF device which is to be bound to PCI-PF-STUB, IGB_UIO or VFIO-PCI in order to enable creation of the FEC VF devices. Once the FEC PF is bound to the correct driver, the user can create a number of devices to be used in Cloud Native deployment of vRAN to accelerate FEC. Once these VF devices are created they need to be bound to a user-space driver such as VFIO-PCI in order for them to work and be consumed in vRAN application pods. Before the device can be used by the application, the device needs to be configured - notably the mapping of queues exposed to the VFs - this is done via pf-bb-config application with the input from the CR used as a configuration.

> NOTE: For [Intel® vRAN Dedicated Accelerator ACC100](https://github.com/intel/sriov-fec-operator/blob/main/spec/vran-accelerators-supported-by-operator.md#intel-vran-dedicated-accelerator-acc100) it is advised to create all 16 VFs. The card is configured to provide up to 8 queue groups with up to 16 queues per group. The queue groups can be divided between groups allocated to 5G/4G and Uplink/Downlink, it can be configured for 4G or 5G only, or both 4G and 5G at the same time. Each configured VF has access to all the queues. Each of the queue groups has a distinct priority level. The request for given queue group is made from application level (ie. vRAN application leveraging the FEC device).

- To get all the nodes containing one of the supported vRAN FEC accelerator devices run the following command (all the commands are run in the `vran-acceleration-operators` namespace, if operator is used on Kubernetes then use `oc` instead of `kubectl`):

```shell
[user@ctrl1 /home]# kubectl get sriovfecnodeconfig
NAME             CONFIGURED
node1            Succeeded
```

- To find the PF of the SRIOV FEC accelerator device to be configured, run the following command:

```shell
[user@ctrl1 /home]# kubectl get sriovfecnodeconfig node1 -o yaml

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

* [Sample CR for Wireless FEC (ACC100)](#sample-cr-for-wireless-fec-acc100)

```yaml
apiVersion: sriovfec.intel.com/v2
kind: SriovFecClusterConfig
metadata:
  name: config
spec:
  priority: 1
  drainSkip: true
  nodeSelector:
    kubernetes.io/hostname: node01
  acceleratorSelector:
    pciAddress: 0000:af:00.0    
  physicalFunction:
    pfDriver: "vfio-pci"
    vfDriver: "vfio-pci"
    vfAmount: 2
    bbDevConfig:
      acc100:
        # Programming mode: 0 = VF Programming, 1 = PF Programming
        pfMode: false
        numVfBundles: 2
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

- To apply the CR run:

```shell
[user@ctrl1 /home]# kubectl apply -f <sriovfec_cr_name>.yaml
```

After creation of the CR, the SRIOV FEC daemon starts configuring the accelerator device. Once the configuration is complete, the following status is reported:

```shell
[user@ctrl1 /home]# kubectl get sriovfecnodeconfig
NAME             CONFIGURED
node1            Succeeded
```

From SRIOV FEC daemon pod, the user should see logs similar to the output below, if the VF queues were successfully programmed. For a list of sample log outputs for applicable devices see:

* [Sample Daemon log for Wireless FEC (ACC100)](#sample-daemon-log-for-wireless-fec-acc100)

```shell
[user@ctrl1 /home]# kubectl get pod | grep sriov-fec-daemonset
sriov-fec-daemonset-h4jf8                      1/1     Running   0          19h

[user@ctrl1 /home]# kubectl logs sriov-fec-daemonset-h4jf8

***
{"level":"Level(-2)","ts":1616798129.251027,"logger":"daemon.drainhelper.cordonAndDrain()","msg":"node drained"}
{"level":"Level(-4)","ts":1616798129.2510319,"logger":"daemon.drainhelper.Run()","msg":"worker function - start"}
{"level":"Level(-4)","ts":1616798129.341839,"logger":"daemon.NodeConfigurator.applyConfig","msg":"current node status","inventory":{"sriovAccelerators":[{"vendorID":"8086","deviceID":"0b32","pciAddress":"0000:20:00.0","driver":"pci-pf-stub","maxVirtualFunctions":1,"virtualFunctions":[{"pciAddress":"0000:20:00.1","driver":"vfio-pci","deviceID":"0b33"}]},{"vendorID":"8086","deviceID":"0d5c","pciAddress":"0000:af:00.0","driver":"pci-pf-stub","maxVirtualFunctions":16,"virtualFunctions":[{"pciAddress":"0000:b0:00.0","driver":"vfio-pci","deviceID":"0d5d"},{"pciAddress":"0000:b0:00.1","driver":"vfio-pci","deviceID":"0d5d"},{"pciAddress":"0000:b0:00.2","driver":"vfio-pci","deviceID":"0d5d"},{"pciAddress":"0000:b0:00.3","driver":"vfio-pci","deviceID":"0d5d"}]}]}}
{"level":"Level(-4)","ts":1616798129.3419566,"logger":"daemon.NodeConfigurator.applyConfig","msg":"configuring PF","requestedConfig":{"pciAddress":"0000:20:00.0","pfDriver":"pci-pf-stub","vfDriver":"vfio-pci","vfAmount":1,"bbDevConfig":{"acc100":{"numVfBundles":1,"maxQueueSize":1024,"uplink4G":{"numQueueGroups":0,"numAqsPerGroups":16,"aqDepthLog2":4},"downlink4G":{"numQueueGroups":0,"numAqsPerGroups":16,"aqDepthLog2":4},"uplink5G":{"numQueueGroups":4,"numAqsPerGroups":16,"aqDepthLog2":4},"downlink5G":{"numQueueGroups":4,"numAqsPerGroups":16,"aqDepthLog2":4}}}}{"level":"Level(-4)","ts":1616798129.3419993,"logger":"daemon.NodeConfigurator.loadModule","msg":"executing command","cmd":"modprobe pci-pf-stub"}
{"level":"Level(-4)","ts":1616798129.3458664,"logger":"daemon.NodeConfigurator.loadModule","msg":"commands output","output":""}
{"level":"Level(-4)","ts":1616798129.345896,"logger":"daemon.NodeConfigurator.loadModule","msg":"executing command","cmd":"modprobe vfio-pci"}
{"level":"Level(-4)","ts":1616798129.3490586,"logger":"daemon.NodeConfigurator.loadModule","msg":"commands output","output":""}
{"level":"Level(-2)","ts":1616798130.3972273,"logger":"daemon.NodeConfigurator","msg":"device is bound to driver","path":"/sys/bus/pci/devices/0000:20:00.0/driver"}
```

The user can observe the change of the cards FEC configuration. The created devices should appear similar to the following output (The '0d5c' is a PF of the FEC device and the '0d5d' is a VF of the FEC device). For a list of sample status output for applicable devices see:

* [Sample Status for Wireless FEC (ACC100)](#sample-status-for-wireless-fec-acc100)

```yaml
[user@ctrl1 /home]# kubectl get sriovfecnodeconfig node1 -o yaml

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
        driver: vfio-pci
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
```

#### SRIOV Device Plugin

As part of the SRIOV FEC operator the K8s SRIOV Network Device plugin is being deployed. The plugin is configured to detect the FEC devices only and is being configured according to the CR. This deployment of the SRIOV Network Device plugin does not manage non-FEC devices. For more information, refer to the documentation for [SRIOV Network Device plugin](https://github.com/openshift/sriov-network-device-plugin). After the deployment of the Operator and update/application of the CR, the user will be able to detect the FEC VFs as allocatable resources in the OpenShift cluster. The output should be similar to this (`intel.com/intel_fec_acc100` or alternative for a different FEC accelerator):

```shell
[user@node1 /home]# kubectl get node <node_name> -o json | jq '.status.allocatable'
{
  "cpu": "95500m",
  "ephemeral-storage": "898540920981",
  "hugepages-1Gi": "30Gi",
  "intel.com/intel_fec_acc100": "2",
  "memory": "115600160Ki",
  "pods": "250"
}
```

Once the SRIOV operator takes care of setting up and configuring the device, user can test the device using a sample 'test-bbdev' application from the [DPDK project (DPDK 20.11)](https://github.com/DPDK/dpdk/tree/v20.11/app/test-bbdev). An example of a prepared sample application's docker image can be found in [Intel® SEO project github EdgeApps repo](https://github.com/intel/edgeapps/tree/master/applications/fpga-sample-app). SEO is an edge computing software toolkit that enables highly optimized and performant edge platforms to on-board and manage applications and network functions with cloud-like agility across any type of network. For more information, go to [www.intel.github.io](https://intel.github.io/).

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
PCIDEVICE_INTEL_COM_INTEL_FEC_ACC100=0000:cb:00.0
```

With the PCIe B:D.F of the FEC VF allocated to the pod established, user will run the 'test-bbdev' application to test the device (similar output indicating that the tests are passing is expected):

Depending on PfDriver that you are using, you might have to add additional parameters to DPDK applications to make them work.

`pci-pf-stub` and `igb_uio` don't require additional parameters.
```shell
[root@pod-bbdev-sample-app ~]# ./test-bbdev.py --testapp-path ./dpdk-test-bbdev -e="-a 0000:cb:00.0" -n 1 -b 1 -l 1 -c validation
```
For `vfio-pci` uuid token is required (as described above), so `--vfio-vf-token` parameter is required.
```shell
[root@pod-bbdev-sample-app ~]# ./test-bbdev.py --testapp-path ./dpdk-test-bbdev -e="-a 0000:cb:00.0 --vfio-vf-token '02bddbbf-bbb0-4d79-886b-91bad3fbb510'" -n 1 -b 1 -l 1 -c validation
 -v ldpc_dec_v7813.data
Executing: ./dpdk-test-bbdev -a 0000:cb:00.0 --vfio-vf-token 02bddbbf-bbb0-4d79-886b-91bad3fbb510 -- -n 1 -l 1 -c validation -v ldpc_dec_v7813.data -b 1
EAL: Detected CPU lcores: 128
EAL: Detected NUMA nodes: 2
EAL: Detected static linkage of DPDK
EAL: Multi-process socket /var/run/dpdk/rte/mp_socket
EAL: Selected IOVA mode 'VA'
EAL: 2048 hugepages of size 2097152 reserved, but no mounted hugetlbfs found for that size
EAL: VFIO support initialized
EAL: Using IOMMU type 1 (Type 1)
EAL: Probe PCI driver: intel_acc100_vf (8086:d5d) device: 0000:cb:00.0 (socket 1)
TELEMETRY: No legacy callbacks, legacy socket not created

===========================================================
Starting Test Suite : BBdev Validation Tests
Test vector file = ldpc_dec_v7813.data
Allocated all queues (id=16) at prio0 on dev0
Allocated all queues (id=32) at prio1 on dev0
Allocated all queues (id=48) at prio2 on dev0
Allocated all queues (id=64) at prio3 on dev0
All queues on dev 0 allocated: 64
+ ------------------------------------------------------- +
== test: validation
dev:0000:cb:00.0, burst size: 1, num ops: 1, op type: RTE_BBDEV_OP_LDPC_DEC
Operation latency:
        avg: 15358 cycles, 10.2387 us
        min: 15358 cycles, 10.2387 us
        max: 15358 cycles, 10.2387 us
TestCase [ 0] : validation_tc passed
 + ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~ +
 + Test Suite Summary : BBdev Validation Tests
 + Tests Total :        1
 + Tests Skipped :      0
 + Tests Passed :       1
 + Tests Failed :       0
 + Tests Lasted :       109.327 ms
 + ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~ +
```

## VFIO_PCI Driver

### Secure Boot

Until SRIOV-FEC operator 2.3.0, `pf-bb-config` application which comes as part of SRIOV-FEC operator distribution, 
relied on MMIO access to the PF of the ACC100 (access through mmap of the PF PCIe BAR config space using igb_uio and/or pf_pci_stub drivers).

In case of enabled secure boot, this access is blocked by kernel through a feature called [lockdown](https://man7.org/linux/man-pages/man7/kernel_lockdown.7.html).
Lockdown mode automatically prevents relying on igb_uio and/or pf_pci_stub drivers due to the direct mmap.
In other words: when secure boot is enabled, this legacy usage is not supported.

To be able to support this special mode, `pf_bb_config` application has been enhanced (v22.03) and now it could use more secure approach relying on vfio-pci.  

| ![SRIOV FEC Operator Design](images/vfio/vfio-pci.svg) |
|------------------------------------------------------|

SRIOV-FEC operator 2.3.0 leverages enhancements provided by `pf_bb_config` and it provides support for vfio-pci driver. 
It means operator would work correctly on a platforms where secure boot is enabled.

'vfio-pci' driver support implemented in SRIOV-FEC operator 2.3.0 is visualized by diagram below:


| ![SRIOV FEC Operator Design](images/vfio/vfio-pci-support-in-operator.svg) |
|-------------------------------------------------------------------------------|

Previously supported drivers `pci-pf-stub` and `igb_uio` are still supported by an operator, but they cannot be used together with secure boot feature.

### Vfio Token

Please be aware that usage of `vfio-pci` driver requires following arguments added to the kernel:
 - vfio_pci.enable_sriov=1
 - vfio_pci.disable_idle_d3=1

If `vfio-pci` PF driver is used, then access to VF requires `UUID` token. Token is identical for all nodes in cluster, has default value of `02bddbbf-bbb0-4d79-886b-91bad3fbb510` and could be changed by
    setting `SRIOV_FEC_VFIO_TOKEN` in `subscription.spec.config.env` field. Applications that are using VFs should provide token via EAL parameters - e.g
    `./test-bbdev.py -e="--vfio-vf-token=02bddbbf-bbb0-4d79-886b-91bad3fbb510 -a0000:f7:00.1"`

The `VFIO_TOKEN` can be fetched by "secret" using following commands, but this will be deprecated in future release due to security concern.

```shell
kubectl get secret vfio-token -o jsonpath='{.data.VFIO_TOKEN}' | base64 --decode
```

Sriov-network-device-plugin v4.14 has the capability to inject the VFIO token as an environment variable to the application pod. FEC Operator pointing to v4.14, leverages this feature to pass the VFIO token in more secured method to the application pods. You can use following commands to get the `VFIO_TOKEN` in application pods.

```shell
#Premise you have successfully configured fec-operator settings and enter an application pod
[root@pod:/home]# env | grep VFIO_TOKEN
PCIDEVICE_INTEL_COM_INTEL_FEC_ACC100_INFO={"0000:8b:01.5":{"extra":{"VFIO_TOKEN":"02bddbbf-bbb0-4d79-886b-91bad3fbb510"},"generic":{"deviceID":"0000:8b:01.5"},"vfio":{"dev-mount":"/dev/vfio/178","mount":"/dev/vfio/vfio"}},"0000:8b:01.7":{"extra":{"VFIO_TOKEN":"02bddbbf-bbb0-4d79-886b-91bad3fbb510"},"generic":{"deviceID":"0000:8b:01.7"},"vfio":{"dev-mount":"/dev/vfio/180","mount":"/dev/vfio/vfio"}}}
[root@pod:/home]# export VFIO_TOKEN=02bddbbf-bbb0-4d79-886b-91bad3fbb510
```

## Deploying the Operator

The SRIOV-FEC Operator for Wireless FEC Accelerators is easily deployable from the OpenShift or Kubernetes cluster via provisioning and application of the following YAML spec files:

If operator is being installed on OpenShift, then follow [deployment steps for OpenShift](openshift-deployment.md).
Otherwise follow [steps for Kubernetes](kubernetes-deployment.md).

### Applying Custom Resources

Once the operator is successfully deployed, the user interacts with it by creating CRs which will be interpreted by the operators, for examples of CRs see the following section:
- [FEC Configuration](#fec-configuration)

To apply a CR run:

```shell
[user@ctrl1 /home]# kubectl apply -f <cr-name>
```

To view the status of current CR run (sample output):

```shell
[user@ctrl1 /home]# kubectl get sriovfecclusterconfig config -o yaml
***
spec:
  priority: 1
  nodeSelector:
    kubernetes.io/hostname: node1
  acceleratorSelector:
    pciAddress: 0000:af:00.0    
  physicalFunction:  
    bbDevConfig:
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
        numVfBundles: 2
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
    pfDriver: vfio-pci
    vfAmount: 2
    vfDriver: vfio-pci
status:
  syncStatus: Succeeded
```

### Telemetry
Operator exposes telemetry from pf-bb-config application for any supported card which uses `vfio-pci` PF driver in Prometheus format.
      It is available in `daemonset` container under `:8080/bbdevconfig` endpoint.
      By default endpoint updates metrics every 15 second, however this interval could be modified by
      changing value of `SRIOV_FEC_METRIC_GATHER_INTERVAL` env var in operators subscription.

There are 5 available metrics:
- bytes_processed_per_vfs - represents number of bytes that are processed by VF
  - `pci_address` - represents unique BDF for VF
  - `queue_type` - represents queue type for VF. Available values: `5GDL`, `5GUL`, `FFT`
- code_blocks_per_vfs - number of code blocks processed by VF
  - `pci_address` - represents unique BDF for VF
  - `queue_type` - represents queue type for VF. Available values: `5GDL`, `5GUL`, `FFT`
- counters_per_engine - number of code blocks processed by Engine
  - `engine_id` - represents integer ID of engine on card
  - `pci_address` - represents unique BDF for card on which engine is located
  - `queue_type` - represents queue type for VF. Available values: `5GDL`, `5GUL`, `FFT`
- vf_count - describes number of configured VFs on card
  - `pci_address` - represents unique BDF for PF
  - `status` - represents current status of SriovFecNodeConfig. Available values: `InProgress`, `Succeeded`, `Failed`, `Ignored`
- vf_status - equals to 1 if `status` is `RTE_BBDEV_DEV_CONFIGURED` or `RTE_BBDEV_DEV_ACTIVE` and 0 otherwise
  - `pci_address` - represents unique BDF for VF
  - `status` - represents status as exposed by pf-bb-config. Available values: `RTE_BBDEV_DEV_NOSTATUS`, `RTE_BBDEV_DEV_NOT_SUPPORTED`, `RTE_BBDEV_DEV_RESET`,
    `RTE_BBDEV_DEV_CONFIGURED`, `RTE_BBDEV_DEV_ACTIVE`, `RTE_BBDEV_DEV_FATAL_ERR`, `RTE_BBDEV_DEV_RESTART_REQ`, `RTE_BBDEV_DEV_RECONFIG_REQ`, `RTE_BBDEV_DEV_CORRECT_ERR`

If SriovFecNodeConfig for node is in `Succeeded` state, then all those metrics are exposed
```
bytes_processed_per_vfs{pci_address="0000:cb:00.0",queue_type="5GUL"} 0
bytes_processed_per_vfs{pci_address="0000:cb:00.0",queue_type="5GDL"} 0
code_blocks_per_vfs{pci_address="0000:cb:00.0",queue_type="5GUL"} 0
code_blocks_per_vfs{pci_address="0000:cb:00.0",queue_type="5GDL"} 0
counters_per_engine{engine_id="0",pci_address="0000:ca:00.0",queue_type="5GUL"} 0
vf_count{pci_address="0000:ca:00.0",status="Succeeded"} 1
vf_status{pci_address="0000:cb:00.0",status="RTE_BBDEV_DEV_CONFIGURED"} 1
```
Otherwise only a `vf_count` metric is exposed
```
vf_count{pci_address="0000:ca:00.0",status="Failed"} 0
```

## Hardware Validation Environment

- 3nd Gen Intel® Xeon® processor platform with Intel® vRAN Dedicated Accelerator ACC100.
- 4th Gen Intel® Xeon® Scalable processors with Intel® vRAN Boost accelerator VRB1 (also known as ACC200)
- Intel® Xeon® Granite Rapids-D with Intel® vRAN Boost accelerator VRB2

## Summary

The SRIOV-FEC Operator for Wireless FEC Accelerators is a fully functional tool to manage the Intel® vRAN Boost accelerator resources autonomously in a Cloud Native environment based on the user input.
The operator handles all the necessary actions from creation of FEC resources to configuration and management of the resources within a cluster.

## Appendix 1 - Developer Notes

### Running operator on SNO

If user needs to run operator on Single Node Cluster, then user should provide ClusterConfigs (which are described in following chapters) with `spec.drainSkip: true` to avoid node draining, because it is impossible to drain node if there's only 1 node.

>NOTE: If user run multiple workloads on same node (even in Multi Node Cluster), it is recommended to configure the CR with `spec.drainSkip: true`.

## Appendix 2 - SRIOV-FEC Operator for Wireless FEC Accelerators Examples

### Intel® vRAN Dedicated Accelerator ACC100

#### Sample CR for ACC100

```yaml
apiVersion: sriovfec.intel.com/v2
kind: SriovFecClusterConfig
metadata:
  name: config
  namespace: vran-acceleration-operators
spec:
  priority: 1
  drainSkip: true
  nodeSelector:
    kubernetes.io/hostname: node1
  acceleratorSelector:
    pciAddress: 0000:af:00.0
  physicalFunction:  
    pfDriver: "vfio-pci"
    vfDriver: "vfio-pci"
    vfAmount: 2
    bbDevConfig:
      acc100:
        # Programming mode: 0 = VF Programming, 1 = PF Programming
        pfMode: false
        numVfBundles: 2
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

#### Sample Status for ACC100

```bash
$ kubectly get sriovFecNodeConfig -o yaml
# or using short name
$ kubectly get sfnc -o yaml
```

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
        driver: vfio-pci
        maxVirtualFunctions: 2
        pciAddress: 0000:af:00.0
        vendorID: "8086"
        virtualFunctions:
        - deviceID: 0d5d
          driver: vfio-pci
          pciAddress: 0000:b0:00.0
        - deviceID: 0d5d
          driver: vfio-pci
          pciAddress: 0000:b0:00.1
```

#### Resource name for ACC100
```yaml
        "allocatable": {
            ...
            "intel.com/intel_fec_acc100": "2",
            ...
        },
```

#### Sample Daemon log for ACC100

```shell
{"level":"Level(-2)","ts":1616794345.4786215,"logger":"daemon.drainhelper.cordonAndDrain()","msg":"node drained"}
{"level":"Level(-4)","ts":1616794345.4786265,"logger":"daemon.drainhelper.Run()","msg":"worker function - start"}
{"level":"Level(-4)","ts":1616794345.5762916,"logger":"daemon.NodeConfigurator.applyConfig","msg":"current node status","inventory":{"sriovAccelerat
ors":[{"vendorID":"8086","deviceID":"0b32","pciAddress":"0000:20:00.0","driver":"","maxVirtualFunctions":1,"virtualFunctions":[]},{"vendorID":"8086"
,"deviceID":"0d5c","pciAddress":"0000:af:00.0","driver":"","maxVirtualFunctions":16,"virtualFunctions":[]}]}}
{"level":"Level(-4)","ts":1616794345.5763638,"logger":"daemon.NodeConfigurator.applyConfig","msg":"configuring PF","requestedConfig":{"pciAddress":"
0000:af:00.0","pfDriver":"pci-pf-stub","vfDriver":"vfio-pci","vfAmount":2,"bbDevConfig":{"acc100":{"pfMode":false,"numVfBundles":16,"maxQueueSize":1
024,"uplink4G":{"numQueueGroups":4,"numAqsPerGroups":16,"aqDepthLog2":4},"downlink4G":{"numQueueGroups":4,"numAqsPerGroups":16,"aqDepthLog2":4},"uplink5G":{"numQueueGroups":0,"numAqsPerGroups":16,"aqDepthLog2":4},"downlink5G":{"numQueueGroups":0,"numAqsPerGroups":16,"aqDepthLog2":4}}}}}
{"level":"Level(-4)","ts":1616794345.5774765,"logger":"daemon.NodeConfigurator.loadModule","msg":"executing command","cmd":"modprobe pci-pf-stub"}
{"level":"Level(-4)","ts":1616794345.5842702,"logger":"daemon.NodeConfigurator.loadModule","msg":"commands output","output":""}
{"level":"Level(-4)","ts":1616794345.5843055,"logger":"daemon.NodeConfigurator.loadModule","msg":"executing command","cmd":"modprobe vfio-pci"}
{"level":"Level(-4)","ts":1616794345.6090655,"logger":"daemon.NodeConfigurator.loadModule","msg":"commands output","output":""}
{"level":"Level(-2)","ts":1616794345.6091156,"logger":"daemon.NodeConfigurator","msg":"device's driver_override path","path":"/sys/bus/pci/devices/0000:af:00.0/driver_override"}
{"level":"Level(-2)","ts":1616794345.6091807,"logger":"daemon.NodeConfigurator","msg":"driver bind path","path":"/sys/bus/pci/drivers/pci-pf-stub/bind"}
{"level":"Level(-2)","ts":1616794345.7488534,"logger":"daemon.NodeConfigurator","msg":"device's driver_override path","path":"/sys/bus/pci/devices/0000:b0:00.0/driver_override"}
{"level":"Level(-2)","ts":1616794345.748938,"logger":"daemon.NodeConfigurator","msg":"driver bind path","path":"/sys/bus/pci/drivers/vfio-pci/bind"}
{"level":"Level(-2)","ts":1616794345.7492096,"logger":"daemon.NodeConfigurator","msg":"device's driver_override path","path":"/sys/bus/pci/devices/0000:b0:00.1/driver_override"}
{"level":"Level(-2)","ts":1616794345.7492566,"logger":"daemon.NodeConfigurator","msg":"driver bind path","path":"/sys/bus/pci/drivers/vfio-pci/bind"}
{"level":"Level(-4)","ts":1616794345.74968,"logger":"daemon.NodeConfigurator.applyConfig","msg":"executing command","cmd":"/sriov_workdir/pf_bb_config ACC100 -c /sriov_workdir/0000:af:00.0.ini -p 0000:af:00.0"}
{"level":"Level(-4)","ts":1616794346.5203931,"logger":"daemon.NodeConfigurator.applyConfig","msg":"commands output","output":"Queue Groups: 0 5GUL, 0 5GDL, 4 4GUL, 4 4GDL\nNumber of 5GUL engines 8\nConfiguration in VF mode\nPF ACC100 configuration complete\nACC100 PF [0000:af:00.0] configuration complete!\n\n"}
{"level":"Level(-4)","ts":1616794346.520459,"logger":"daemon.NodeConfigurator.enableMasterBus","msg":"executing command","cmd":"setpci -v -s 0000:af:00.0 COMMAND"}
{"level":"Level(-4)","ts":1616794346.5458736,"logger":"daemon.NodeConfigurator.enableMasterBus","msg":"commands output","output":"0000:af:00.0 @04 = 0142\n"}
{"level":"Level(-4)","ts":1616794346.5459251,"logger":"daemon.NodeConfigurator.enableMasterBus","msg":"executing command","cmd":"setpci -v -s 0000:af:00.0 COMMAND=0146"}
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

### Intel® vRAN Boost accelerator VRB1(ACC200)

#### Sample CR for VRB1(ACC200)

ACC200 introduces new section - `qfft`

SRIOV-FEC Operator does also support sriov-vrb CR API end point to configure the VRB1(ACC200) device. User should choose only one CR API end point to configure the device, but not both.

There is no change in the CR `spec` for both the APIs. The new CR API is introduced to align the naming with new generation of accelerators called vRAN Boost Accelerators a.k.a VRB Accelerators.

##### sriov-fec CR API

```yaml
apiVersion: sriovfec.intel.com/v2
kind: SriovFecClusterConfig
metadata:
  name: config
  namespace: vran-acceleration-operators
spec:
  priority: 1
  drainSkip: true
  acceleratorSelector:
    pciAddress: 0000:f7:00.0
  nodeSelector:
    kubernetes.io/hostname: node01
  physicalFunction:
    bbDevConfig:
      acc200:
        downlink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        downlink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 0
        maxQueueSize: 1024
        numVfBundles: 2
        pfMode: false
        qfft:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        uplink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        uplink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 0
    pfDriver: vfio-pci
    vfAmount: 2
    vfDriver: vfio-pci
```

##### sriov-vrb CR API

```yaml
apiVersion: sriovvrb.intel.com/v1
kind: SriovVrbClusterConfig
metadata:
  name: config
  namespace: vran-acceleration-operators
spec:
  priority: 1
  drainSkip: true
  acceleratorSelector:
    pciAddress: 0000:f7:00.0
  nodeSelector:
    kubernetes.io/hostname: node01
  physicalFunction:
    bbDevConfig:
      vrb1:
        downlink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        downlink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 0
        maxQueueSize: 1024
        numVfBundles: 2
        pfMode: false
        qfft:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        uplink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        uplink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 0
    pfDriver: vfio-pci
    vfAmount: 2
    vfDriver: vfio-pci
```

#### Sample Status for VRB1(ACC200)

```bash
### Using sriov-fec API
$ kubectly get sriovFecNodeConfig -o yaml
# or using short name
$ kubectly get sfnc -o yaml


### Using sriov-vrb API
$ kubectly get sriovVrbNodeConfig -o yaml
# or using short name
$ kubectly get vrbnc -o yaml
```

```yaml
status:
  conditions:
    - lastTransitionTime: "2023-10-25T12:43:29Z"
      message: Configured successfully
      observedGeneration: 5
      reason: Succeeded
      status: "True"
      type: Configured
  inventory:
    sriovAccelerators:
      - deviceID: 57c0
        driver: vfio-pci
        maxVirtualFunctions: 16
        pciAddress: 0000:f7:00.0
        vendorID: "8086"
        virtualFunctions:
          - deviceID: 57c1
            driver: vfio-pci
            pciAddress: 0000:f7:00.1
            driver: vfio-pci
            pciAddress: 0000:f7:00.2
```

#### Resource name for VRB1

```yaml
        "allocatable": {
            ...
            "intel.com/intel_fec_acc200": "2",
            ...
        },
```

### Intel® vRAN Boost accelerator VRB2

#### Sample CR for VRB2

VRB2 introduces new section - `qmld`

```yaml
apiVersion: sriovvrb.intel.com/v1
kind: SriovVrbClusterConfig
metadata:
  name: config
  namespace: vran-acceleration-operators
spec:
  priority: 1
  drainSkip: true
  acceleratorSelector:
    pciAddress: 0000:07:00.0
  nodeSelector:
    kubernetes.io/hostname: node01
  physicalFunction:
    bbDevConfig:
      vrb2:
        downlink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        downlink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 0
        maxQueueSize: 1024
        numVfBundles: 2
        pfMode: false
        qfft:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        qmld:
          aqDepthLog2: 4
          numAqsPerGroups: 64
          numQueueGroups: 4
        uplink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        uplink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 0
    pfDriver: vfio-pci
    vfAmount: 2
    vfDriver: vfio-pci
```

#### Sample Status for VRB2
```bash
$ kubectly get sriovVrbNodeConfig -o yaml
#or using short name
$ kubectly get vrbnc -o yaml
```

```yaml
status:
  conditions:
    - lastTransitionTime: "2024-1-05T12:43:29Z"
      message: Configured successfully
      observedGeneration: 4
      reason: Succeeded
      status: "True"
      type: Configured
  inventory:
    sriovAccelerators:
      - deviceID: 57c2
        driver: vfio-pci
        maxVirtualFunctions: 64
        pciAddress: 0000:07:00.0
        vendorID: "8086"
        virtualFunctions:
          - deviceID: 57c3
            driver: vfio-pci
            pciAddress: 0000:f7:00.1
          - deviceID: 57c3
            driver: vfio-pci
            pciAddress: 0000:f7:00.2
```

#### Resource name for VRB2

```yaml
        "allocatable": {
            ...
            "intel.com/intel_vrb_vrb2": "2",
            ...
        },
```

## Appendix 3 - Update srs_fft_windows_coeffient file

The VRB1 and VRB2 Accelerators has FFT engines includes an internal memory used to define the semi-static windowing parameters. The content of that table is loaded from binary files `srs_fft_windows_coefficient.bin` (up to 1024 points across 16 windows and 7 iDFT sizes).

Operator includes a default `srs_fft_windows_coefficent.bin` file, which is good for all the deployment configuration. In the case of user wants to update the default file, SRIOV-FEC Operator provide support for updating the `srs_fft_windows_coefficent.bin` file through CR.

An optional configuration parameter block under `spec.physicalFunction.bbDevConfig.vrb1` or `spec.physicalFunction.bbDevConfig.acc200` for VRB1 CR and `spec.physiclaFunction.bbDevConfig.vrb2` for VRB2 CR will help to update with user provided `srs_fft_windows_coefficent.bin` file.

```yaml
        fftLut:
          fftUrl: "http://<fqdn-of-http-server>/srs_fft_windows_coefficient.tar.gz"
          fftChecksum: "<SHA1 of srs_fft_windows_coefficient.tar.gz>"
```

Before applying the CR on the requested node, Operator's daemonset functionality will download the given `srs_fft_windows_coefficent.tar.gz` file using HTTP or HTTPS and verify the Checksum and untar it.

>NOTE-1: Make sure the reachability of HTTP server from the Operator daemonset pod on the worker node. A simple method could deploying a local http server in a pod in the cluster to update the `srs_fft_windows_coefficent.bin` file.

>NOTE-2: File must be in `.tar.gz` file.
```bash
tar -zcvf srs_fft_windows_coefficient.tar.gz srs_fft_windows_coefficient.bin
```

>NOTE-3: Since operator does not have persistent volume, file will be reset to default after reboot of node or operator is re-deployed. However, if the previously configured CR exists and http serve link is also alive then after reboot/redeploy new `srs_fft_windows_coefficent.tar.gz` will be downloaded again.

>NOTE-4: At any given point of time, Operator's daemonset in worker node will save only one updated file.

>NOTE-5: Apply CR without the `fftLut` block to use the default `srs_fft_windows_coefficent.bin` file at any time.

- Reference CR file for VRB1 with `fftLut` block

```yaml
apiVersion: sriovvrb.intel.com/v1
kind: SriovVrbClusterConfig
metadata:
  name: config
  namespace: vran-acceleration-operators
spec:
  priority: 1
  nodeSelector:
    kubernetes.io/hostname: node01
  acceleratorSelector:
    pciAddress: 0000:f7:00.0
  drainSkip: true
  physicalFunction:
    pfDriver: vfio-pci
    vfDriver: vfio-pci
    vfAmount: 2
    bbDevConfig:
      vrb1:
        # Pf mode: false = VF Programming, true = PF Programming
        pfMode: false
        numVfBundles: 2
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
        qfft:
          numQueueGroups: 4
          numAqsPerGroups: 16
          aqDepthLog2: 4
        fftLut:
          fftUrl: "http://fftlut-cache.default.svc.cluster.local/srs_fft_windows_coefficient.tar.gz"
          fftChecksum: "387bfc0ee19860ffd4ad852b26f8469e8a93032e"
```

## Appendix 4 - Gathering logs for bug report
To gather logs for filing bug report please run `gather_sriovfec_logs.sh` script downloaded from https://github.com/intel/sriov-fec-operator/blob/main/gather_sriovfec_logs.sh

```
Usage: ./gather_sriovfec_logs.sh [K8S_BIN] [NAMESPACE]

Positional arguments:
 K8S_BIN    Orchestrator binary (default: oc)
 NAMESPACE  Namespace with SRIOV-FEC operator pods (default: vran-acceleration-operators)
```

Example
```shell
[user@ctrl1 /home]# ./gather_sriovfec_logs.sh
Getting information about nodes
Getting information about pods in vran-acceleration-operators
Getting information about ClusterConfigs in vran-acceleration-operators
Getting information about NodeConfigs in vran-acceleration-operators
Getting information about system configurations in vran-acceleration-operators
sriov-fec-ctrl1-Wed Aug 24 15:09:57 UTC 2022/
...
sriov-fec-ctrl1-Wed Aug 24 15:09:57 UTC 2022/systemLogs/lspci-worker-1.log
Please attach 'sriov-fec.logs.tar.gz' to bug report. If you had to apply some configs and deleted them to reproduce issue, attach them as well.

[user@ctrl1 /home]# ls -F
 gather_sriovfec_logs.sh*  'sriov-fec-ctrl1-Wed Aug 24 15:09:57 UTC 2022'/   sriov-fec.logs.tar.gz
```
