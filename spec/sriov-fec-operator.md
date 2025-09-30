```text
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2020-2025 Intel Corporation
```
<!-- omit in toc -->
# SRIOV-FEC Operator for Wireless FEC Accelerators

- [Overview](#overview)
- [SRIOV-FEC Operator for Wireless FEC Accelerators](#sriov-fec-operator-for-wireless-fec-accelerators-1)
  - [Wireless FEC Acceleration management](#wireless-fec-acceleration-management)
    - [FEC Configuration](#fec-configuration)
  - [SRIOV-FEC Operator Pods](#sriov-fec-operator-pods)
  - [SRIOV-FEC Operator APIs](#sriov-fec-operator-apis)
  - [SRIOV Network Device Plugin](#sriov-network-device-plugin)
- [Virtual Function I/O (VFIO) Driver](#virtual-function-io-vfio-driver)
  - [Secure Boot](#secure-boot)
  - [VFIO Token](#vfio-token)
- [Deploying the Operator](#deploying-the-operator)
  - [Getting available nodes](#getting-available-nodes)
  - [Getting accelerators from node](#getting-accelerators-from-node)
  - [Creating Custom Resource](#creating-custom-resource-cr)
    - [Sample CR for ACC100](#acc100)
    - [Sample CR for VRB1](#vran-boost-accelerator-v1-vrb1)
    - [Sample CR for VRB2](#vran-boost-accelerator-v2-vrb2)
  - [Applying Custom Resources](#applying-custom-resources)
  - [Retrieving daemon pod logs](#retrieving-daemon-pod-logs)
    - [ACC100 daemon pod log snippet](#full-sample-daemon-pod-log-for-acc100)
    - [VRB1 daemon pod log snippet](#full-sample-daemon-pod-log-for-vrb1)
    - [VRB2 daemon pod log snippet](#full-sample-daemon-pod-log-for-vrb2)
  - [Retrieving Node Configuration](#retrieving-node-configuration)
  - [Deploying a sample test-bbdev pod](#deploying-a-sample-test-bbdev-pod)
  - [Telemetry](#telemetry)
- [Hardware Validation Environment](#hardware-validation-environment)
- [Summary](#summary)
- [Appendix 1 - Developer Notes](#appendix-1---developer-notes)
  - [Drain skip option](#drain-skip-option)
  - [VrbResourceName](#vrbresourcename-optional)
- [Appendix 2 - Reference CR configurations for supported accelerators in SRIOV-FEC Operator](#appendix-2---reference-cr-configurations-for-supported-accelerators-in-sriov-fec-operator)
  - [ACC100](#acc100)
  - [vRAN Boost Accelerator V1 (VRB1)](#vran-boost-accelerator-v1-vrb1)
  - [vRAN Boost Accelerator V2 (VRB2)](#vran-boost-accelerator-v2-vrb2)
- [Appendix 3 - Gathering logs for bug report](#appendix-3---gathering-logs-for-bug-report)
- [Appendix 4 - Additional instructions for applications using VF interface in case of VFIO mode](#appendix-4---additional-instructions-for-applications-using-vf-interface-in-case-of-vfio-mode)

## Overview

This document provides instructions for using the SRIOV-FEC Operator for Wireless FEC Accelerators in Red Hat's OpenShift Container Platform and Kubernetes. Developed with the aid of the Operator SDK project, this operator is designed to manage FEC devices, including Intel® vRAN Dedicated Accelerator ACC100, Intel® FPGA Programmable Acceleration Card N3000 and Intel® vRAN Boost Accelerators like VRB1 and VRB2, which are used to accelerate the FEC process in vRAN L1 applications. While the operator efficiently handles the orchestration and management of FEC resources, it does not cover the management of NIC SRIOV devices/resources within the OpenShift cluster. Users are expected to deploy a separate operator or SRIOV Network Device plugin to manage the orchestration of SRIOV NIC VFs between pods, ensuring comprehensive network resource management alongside FEC acceleration.

## SRIOV-FEC Operator for Wireless FEC Accelerators

The role of the SRIOV-FEC Operator for Intel Wireless FEC Accelerator is to orchestrate and manage the resources/devices exposed by a range of Intel's vRAN FEC acceleration devices/hardware within the OpenShift or Kubernetes cluster. The operator is a state machine which will configure the resources and then monitor them and act autonomously based on the user interaction.
The operator design of the SRIOV-FEC Operator for Intel Wireless FEC Accelerator supports the following vRAN FEC accelerators:

* [Intel® vRAN Dedicated Accelerator ACC100](vran-accelerators-supported-by-operator.md#intel-vran-dedicated-accelerator-acc100)
* [Intel® FPGA Programmable Acceleration Card N3000](vran-accelerators-supported-by-operator.md#intel-fpga-programmable-acceleration-card-n3000)
* [Intel® vRAN Boost Accelerator V1 (VRB1)](vran-accelerators-supported-by-operator.md#intel-vran-boost-accelerator-v1-vrb1)
* [Intel® vRAN Boost Accelerator V2 (VRB2)](vran-accelerators-supported-by-operator.md#intel-vran-boost-accelerator-v2-vrb2)

### Wireless FEC Acceleration management

This operator handles the management of the FEC devices used to accelerate the FEC process in vRAN L1 applications. These FEC devices are provided by designated hardware, including Intel® vRAN Dedicated Accelerator ACC100 and the more advanced Intel® vRAN Boost Accelerators such as VRB1 and VRB2.
The operator provides functionality to create desired VFs (Virtual Functions) for the FEC device, binds them to appropriate drivers, and configures the VF's queues for desired functionality in 4G or 5G deployment.
It also deploys an instance of the [SR-IOV Network Device Plugin](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin) which manages the FEC VFs as an OpenShift cluster resource and configures this device plugin to detect the resources. 
The user interacts with the operator by providing a CR (CustomResource). 
The operator constantly monitors the state of the CR to detect any changes and acts based on the changes detected. 
The CR is provided per cluster configuration. The components for individual nodes can be configured by specifying appropriate values for each component per "nodeSelector".
Once the CR is applied or updated, the operator/daemon checks if the configuration is already applied, and, if not it binds the PFs to driver, creates desired amount of VFs, binds them to driver and runs the [pf-bb-config](https://github.com/intel/pf-bb-config) utility to configure the VF queues to the desired configuration.

This operator is a common operator for FEC device/resource management across a range of accelerator cards, including the Intel® vRAN Boost Accelerators. For specific examples of CRs dedicated to a single accelerator card only, see:

* [Sample CR for ACC100](#sample-cr-for-wireless-fec-acc100)
* [Sample CR for VRB1](#vran-boost-accelerator-v1-vrb1)
* [Sample CR for VRB2](#vran-boost-accelerator-v1-vrb1)

The workflow of the SRIOV-FEC operator is shown in the following diagram:

![SRIOV-FEC Operator Design](images/sriov_fec_operator_acc100.png)

#### FEC Configuration

Intel's vRAN FEC acceleration devices/hardware expose the FEC PF device, which must be bound to drivers such as `pci-pf-stub`, `igb_uio` and `vfio-pci` to enable the creation of FEC VF devices. Among these, VFIO-PCI is strongly recommended for obtaining telemetry data, providing enhanced monitoring and diagnostics capabilities. Once the FEC PF is bound to the correct driver, users can create a number of devices to be used in Cloud Native deployment of vRAN to accelerate FEC. These devices, once created, should be bound to a user-space driver like VFIO-PCI to function properly and be consumed in vRAN application pods. Before the device can be utilized by the application, it needs to be configured, specifically the mapping of queues exposed to the VFs. This configuration is accomplished via the pf-bb-config application, using input from the CR (CustomResource) as a configuration guide.

> NOTE: For [Intel® vRAN Dedicated Accelerator ACC100](vran-accelerators-supported-by-operator.md#intel-vran-dedicated-accelerator-acc100) it is advised to create all 16 VFs. The card is configured to provide up to 8 queue groups with up to 16 queues per group. The queue groups can be divided between groups allocated to 5G/4G and Uplink/Downlink, it can be configured for 4G or 5G only, or both 4G and 5G at the same time. Each configured VF has access to all the queues. Each of the queue groups has a distinct priority level. The request for given queue group is made from application level (ie. vRAN application leveraging the FEC device).

### SRIOV-FEC Operator Pods
- `sriov-fec-bundle`: This pod is part of the deployment bundle for the SRIOV-FEC Operator, containing necessary components and configurations for the operator to function correctly. It ensures that the operator's resources are properly initialized and available for use.
- `accelerator-discovery`: This pod is responsible for discovering and identifying hardware accelerators available in the cluster. It scans the nodes to detect the presence of FEC devices and other accelerators, providing essential information for resource allocation and management.
- `sriov-device-plugin`: This pod runs the SRIOV device plugin, which is responsible for managing SRIOV virtual functions (VFs) as Kubernetes resources. It ensures that VFs are correctly allocated to pods based on their resource requests and constraints.
- `sriov-fec-controller-manager`: This pod is a critical component of the SRIOV-FEC Operator, responsible for orchestrating and managing the resources and devices exposed by Intel's vRAN FEC acceleration hardware within the OpenShift cluster. This pod includes two main containers:
  - kube-rbac-proxy: This container provides a secure proxy for the Kubernetes API server, ensuring that access to the operator's metrics and health endpoints is properly authenticated and authorized. It listens on port 8443 and uses the kube-rbac-proxy image to facilitate secure communication.
  - manager: The manager container is the core of the controller-manager pod, executing the operator's logic to manage FEC devices. It handles tasks such as leader election, metrics collection, and health checks. The manager container uses the sriov-fec-operator image and is configured with environment variables to specify images for the daemon, labeler, and network device plugin, as well as liveness and readiness probe settings.
The pod is configured with various volumes for storing certificates and service account tokens, ensuring secure operation. It also includes node selectors and tolerations to ensure compatibility across different node architectures. The controller-manager pod plays a vital role in maintaining the desired state of FEC resources, applying configurations specified in Custom Resources (CRs) such as SriovFecClusterConfig and SriovFecNodeConfig, and ensuring efficient resource management and acceleration of vRAN workloads.
- `sriov-fec-daemonset`: The sriov-fec-daemonset pod is a key component of the SRIOV-FEC Operator, deployed as a DaemonSet to ensure that it runs on every node in the cluster. This pod is responsible for configuring and managing the FEC devices on individual nodes, ensuring they are correctly set up and available for use by application pods. Here are the details of its configuration and functionality:
  - sriov-fec-daemon container: This container runs the sriov-fec-daemon image, which is responsible for the node-level management of FEC devices. It listens on port 8080 for internal communications and performs health and readiness checks via HTTP endpoints. The container is configured with environment variables to specify the namespace, node name, and various operational parameters such as drain timeout and lease duration. Additionally, the [pf-bb-config](https://github.com/intel/pf-bb-config) utility runs inside this container, executing the necessary configurations for FEC devices, such as queue mappings and driver bindings, to ensure optimal performance and resource allocation.
  - Node Selectors and Tolerations: The pod is configured with node selectors to ensure it runs on nodes with Intel accelerators present. It also includes tolerations for various node conditions, allowing it to remain scheduled even under resource pressure.
  - Functionality: The daemonset pod continuously monitors the state of FEC devices, applies necessary configurations using the pf-bb-config utility, and ensures that they are ready for use by application pods. It plays a crucial role in maintaining the operational readiness of FEC resources across the cluster.

### SRIOV-FEC Operator APIs
The SRIOV-FEC Operator provides two distinct APIs to manage different types of FEC acceleration devices and integrated solutions. These APIs are designed to cater to the specific requirements and configurations of the hardware they support, ensuring efficient resource management and acceleration of FEC processes in vRAN deployments.
- Legacy API for ACC100 and N3000: The operator includes a legacy API located at [sriovfec/v2](https://github.com/intel/sriov-fec-operator/tree/main/api/sriovfec/v2). This API is primarily used for managing the Intel® vRAN Dedicated Accelerator ACC100 card and the Intel® FPGA Programmable Acceleration Card N3000. It features two main resources: `sriovfecnodeconfig` (sfnc) and `sriovfecclusterconfig` (sfcc). These resources allow users to configure and manage node-specific and cluster-wide settings, respectively, providing tailored functionalities for the operation of these devices. In this documentation, we will be using the sriovfec/v2 API for examples with ACC100.
- API for VRB1, VRB2, and Future Deployments: For the more advanced Intel® vRAN Boost integrated solutions, such as VRB1 and VRB2, the operator offers a dedicated API located at [sriovvrb/v1](https://github.com/intel/sriov-fec-operator/tree/main/api/sriovvrb/v1). This API is designed to support the unique features and capabilities of the VRB1 and VRB2 solutions, facilitating their integration and management within cloud-native environments. It includes two main resources: `sriovvrbnodeconfig` (svnc) and `sriovvrbclusterconfig` (svcc), which enable users to configure node-specific and cluster-wide settings for these solutions. This API allows users to leverage the enhanced processing capabilities of VRB1 and VRB2, including advanced queue management and signal processing features.
By providing these two APIs, the SRIOV-FEC Operator ensures comprehensive support for a wide range of FEC acceleration devices and solutions, allowing users to optimize their vRAN deployments according to the specific hardware in use.

### SRIOV Network Device Plugin

As part of the SRIOV-FEC operator the K8s SRIOV Network Device plugin is being deployed. The plugin is configured to detect the FEC devices only and is being configured according to the CR. This deployment of the SRIOV Network Device plugin does not manage non-FEC devices. For more information, refer to the documentation for [SRIOV Network Device plugin](https://github.com/openshift/sriov-network-device-plugin). After the deployment of the Operator and update/application of the CR, the user will be able to detect the FEC VFs as allocatable resources in the OpenShift cluster. The output should be similar to this (`intel.com/intel_fec_acc100` or alternative for a different FEC accelerator):

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

## Virtual Function I/O (VFIO) Driver

### Secure Boot

Until SRIOV-FEC operator 2.3.0, `pf-bb-config` application which comes as part of SRIOV-FEC operator distribution, 
relied on MMIO access to the PF of the ACC100 (access through mmap of the PF PCIe BAR config space using igb_uio and/or pf_pci_stub drivers).

In case of enabled secure boot, this access is blocked by kernel through a feature called [lockdown](https://man7.org/linux/man-pages/man7/kernel_lockdown.7.html).
Lockdown mode automatically prevents relying on igb_uio and/or pf_pci_stub drivers due to the direct mmap.
In other words: when secure boot is enabled, this legacy usage is not supported.

To be able to support this special mode, `pf_bb_config` application has been enhanced (v22.03) and now it could use more secure approach relying on vfio-pci.  

| ![SRIOV-FEC Operator Design](images/vfio/vfio-pci.svg) |
|------------------------------------------------------|

SRIOV-FEC operator 2.3.0 leverages enhancements provided by `pf_bb_config` and it provides support for vfio-pci driver. 
It means operator would work correctly on a platforms where secure boot is enabled.

'vfio-pci' driver support implemented in SRIOV-FEC operator 2.3.0 is visualized by diagram below:


| ![SRIOV-FEC Operator Design](images/vfio/vfio-pci-support-in-operator.svg) |
|-------------------------------------------------------------------------------|

Previously supported drivers `pci-pf-stub` and `igb_uio` are still supported by an operator, but they cannot be used together with secure boot feature.

### VFIO Token

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

The SRIOV-FEC Operator for Wireless FEC Accelerators is easily deployable from the OpenShift or Kubernetes cluster via provisioning and application of YAML spec files.
If operator is being installed on OpenShift, then follow [deployment steps for OpenShift](openshift-deployment.md).
Otherwise follow [steps for Kubernetes](kubernetes-deployment.md).
> **_NOTE:_** **The following examples use the `sriovfec/v2` API (`sriovfecnodeconfig`, `sriovfecclusterconfig`).**  
> **If deploying VRB1 or VRB2, the `sriovvrb/v1` API (`sriovvrbnodeconfig`, `sriovvrbclusterconfig`) must be used instead.**

### Getting available nodes
To get all the nodes containing one of the supported vRAN FEC accelerator devices run the following command (all the commands are run in the `vran-acceleration-operators` namespace, if operator is used on Kubernetes then use `kubectl` instead of `oc`):
```shell
[user@ctrl1 /home]# oc get sriovfecnodeconfig
NAME             CONFIGURED
node1            NotRequested
```

### Getting accelerators from node
To find the PF of the SRIOV-FEC accelerator device to be configured, run the following command:

```shell
[user@ctrl1 /home]# oc get sriovfecnodeconfig node1 -o yaml

***
status:
  conditions:
  - lastTransitionTime: "2025-05-07T22:32:15Z"
    message: ""
    observedGeneration: 1
    reason: NotRequested
    status: "False"
    type: Configured
  inventory:
    sriovAccelerators:
    - deviceID: 0d5c
      driver: ""
      maxVirtualFunctions: 16
      pciAddress: 0000:af:00.0
      vendorID: "8086"
      virtualFunctions: []
  pfBbConfVersion: v25.01-0-g812e032
```

### Creating Custom Resource (CR)
To configure the FEC device with desired settings, create YAML file for the CR.
For the list of sample CRs applicable to all supported devices see:

* [Sample CR for ACC100](#acc100)
* [Sample CR for VRB1](#vran-boost-accelerator-v1-vrb1)
* [Sample CR for VRB2](#vran-boost-accelerator-v2-vrb2)

### Applying Custom Resources
To apply a CR run:

```shell
[user@ctrl1 /home]# oc apply -f <cr-yaml-file>
```

After creation of the CR, the SRIOV-FEC daemon starts configuring the FEC device. Once the SRIOV-FEC configuration is complete, the following status is reported:

```shell
[user@ctrl1 /home]# oc get sriovfecnodeconfig
NAME             CONFIGURED
node1            Succeeded
```

To view the status of current CR run (sample output):

```shell
[user@ctrl1 /home]# oc get sriovfecclusterconfig config -o yaml
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

### Retrieving daemon pod logs
To view logs from the SRIOV-FEC daemon pod, which indicate successful programming of the VF queues, you can use the following command.

```shell
[user@ctrl1 /home]# oc get pod | grep sriov-fec-daemonset
sriov-fec-daemonset-h4jf8                      1/1     Running   0          19h

[user@ctrl1 /home]# oc logs sriov-fec-daemonset-h4jf8
```
By executing these commands, you can access the logs that provide insights into the configuration and status of the FEC devices managed by the operator.
To access the pf-bb-config log data within the daemon pod logs, users can search for the string monitorLogFile. This string indicates that the pf-bb-config log file has been integrated into the daemon pod logs, allowing users to easily locate and review the configuration details and status updates related to the FEC device.
For detailed examples of sample log outputs for applicable devices, please refer to the following links:

#### [Full sample daemon pod log for ACC100](samples/acc100-daemon-pod.log)
```
***
{"file":"/workspace-go/pkg/daemon/common.go:48","func":"github.com/intel/sriov-fec-operator/pkg/daemon.execAndSuppress","level":"info","msg":"commands output","output":"== pf_bb_config Version v25.01-0-g812e032 ==\nACC100 PF [0000:8a:00.0] configuration complete!\nLog file = /var/log/pf_bb_cfg_0000:8a:00.0.log\n","time":"2025-05-07T22:43:31Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Wed May  7 22:43:29 2025:INFO:Queue Groups: 2 5GUL, 2 5GDL, 2 4GUL, 2 4GDL","pciAddr":"0000:8a:00.0","time":"2025-05-07T22:43:31Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Wed May  7 22:43:29 2025:INFO:Configuration in VF mode","pciAddr":"0000:8a:00.0","time":"2025-05-07T22:43:31Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Wed May  7 22:43:30 2025:INFO: ROM version MM 99AD92","pciAddr":"0000:8a:00.0","time":"2025-05-07T22:43:31Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Wed May  7 22:43:31 2025:INFO:DDR Training completed in 1250 ms","pciAddr":"0000:8a:00.0","time":"2025-05-07T22:43:31Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Wed May  7 22:43:31 2025:INFO:PF ACC100 configuration complete","pciAddr":"0000:8a:00.0","time":"2025-05-07T22:43:31Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Wed May  7 22:43:31 2025:INFO:ACC100 PF [0000:8a:00.0] configuration complete!","pciAddr":"0000:8a:00.0","time":"2025-05-07T22:43:31Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Wed May  7 22:43:31 2025:INFO:Running in daemon mode for VFIO VF token","pciAddr":"0000:8a:00.0","time":"2025-05-07T22:43:31Z"}
{"file":"/workspace-go/pkg/daemon/node_management.go:65","func":"github.com/intel/sriov-fec-operator/pkg/daemon.(*NodeConfigurator).isDeviceBoundToDriver","level":"info","msg":"device is bound to driver","path":"/sys/bus/pci/devices/0000:8b:00.0/driver","time":"2025-05-07T22:43:31Z"}
```
#### [Full sample daemon pod log for VRB1](samples/vrb1-daemon-pod.log)
```
***
{"file":"/workspace-go/pkg/daemon/bbdevconfig_ini_generator.go:107","func":"github.com/intel/sriov-fec-operator/pkg/daemon.logIniFile","generated BBDevConfig":"[MODE]\npf_mode_en = 0\n\n[VFBUNDLES]\nnum_vf_bundles = 1\n\n[MAXQSIZE]\nmax_queue_size = 1024\n\n[QUL4G]\nnum_qgroups        = 2\nnum_aqs_per_groups = 16\naq_depth_log2      = 4\n\n[QDL4G]\nnum_qgroups        = 2\nnum_aqs_per_groups = 16\naq_depth_log2      = 4\n\n[QUL5G]\nnum_qgroups        = 2\nnum_aqs_per_groups = 16\naq_depth_log2      = 4\n\n[QDL5G]\nnum_qgroups        = 2\nnum_aqs_per_groups = 16\naq_depth_log2      = 4\n\n[QFFT]\nnum_qgroups        = 2\nnum_aqs_per_groups = 16\naq_depth_log2      = 4\n","level":"info","msg":"logIniFile","time":"2025-05-07T22:56:00Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:312","func":"github.com/intel/sriov-fec-operator/pkg/daemon.(*fftUpdater).VrbgetFftFilePath","level":"info","msg":"Using default SRS FFT file for configuration","time":"2025-05-07T22:56:00Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:197","func":"github.com/intel/sriov-fec-operator/pkg/daemon.(*pfBBConfigController).updateFftWindowsCoefficientFilepath","level":"info","msg":"SRS FFT file path is : /sriov_workdir/vrb1/srs_fft_windows_coefficient.bin","time":"2025-05-07T22:56:00Z"}
{"args":["/sriov_workdir/pf_bb_config","VRB1","-c","/tmp/0000:f7:00.0.ini","-p","0000:f7:00.0","-v","02bddbbf-bbb0-4d79-886b-91bad3fbb510","-f","/sriov_workdir/vrb1/srs_fft_windows_coefficient.bin"],"cmd":"/sriov_workdir/pf_bb_config","file":"/workspace-go/pkg/daemon/common.go:35","func":"github.com/intel/sriov-fec-operator/pkg/daemon.execAndSuppress","level":"info","msg":"executing command","time":"2025-05-07T22:56:00Z"}
{"file":"/workspace-go/pkg/daemon/common.go:48","func":"github.com/intel/sriov-fec-operator/pkg/daemon.execAndSuppress","level":"info","msg":"commands output","output":"== pf_bb_config Version v25.01-0-g812e032 ==\nVRB1 PF [0000:f7:00.0] configuration complete!\nLog file = /var/log/pf_bb_cfg_0000:f7:00.0.log\n","time":"2025-05-07T22:56:00Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Wed May  7 22:56:00 2025:INFO:Queue Groups UL4G 2 DL4G 2 UL5G 2 DL5G 2 FFT 2","pciAddr":"0000:f7:00.0","time":"2025-05-07T22:56:00Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Wed May  7 22:56:00 2025:INFO:Configuration in VF mode","pciAddr":"0000:f7:00.0","time":"2025-05-07T22:56:00Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Wed May  7 22:56:00 2025:INFO:  FFT Window coeffs preloading from /sriov_workdir/vrb1/srs_fft_windows_coefficient.bin","pciAddr":"0000:f7:00.0","time":"2025-05-07T22:56:00Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Wed May  7 22:56:00 2025:INFO:  FFT Size 2048 Window 0 Size 296 Start -40","pciAddr":"0000:f7:00.0","time":"2025-05-07T22:56:00Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Wed May  7 22:56:00 2025:INFO:  FFT Version Number D588","pciAddr":"0000:f7:00.0","time":"2025-05-07T22:56:00Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Wed May  7 22:56:00 2025:INFO:VRB1 configuration complete","pciAddr":"0000:f7:00.0","time":"2025-05-07T22:56:00Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Wed May  7 22:56:00 2025:INFO:VRB1 PF [0000:f7:00.0] configuration complete!","pciAddr":"0000:f7:00.0","time":"2025-05-07T22:56:00Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Wed May  7 22:56:00 2025:INFO:Running in daemon mode for VFIO VF token","pciAddr":"0000:f7:00.0","time":"2025-05-07T22:56:00Z"}
{"file":"/workspace-go/pkg/daemon/node_management.go:65","func":"github.com/intel/sriov-fec-operator/pkg/daemon.(*NodeConfigurator).isDeviceBoundToDriver","level":"info","msg":"device is bound to driver","path":"/sys/bus/pci/devices/0000:f7:00.1/driver","time":"2025-05-07T22:56:00Z"}
```
#### [Full sample daemon pod log for VRB2](samples/vrb2-daemon-pod.log)
```
***
{"file":"/workspace-go/pkg/daemon/bbdevconfig_ini_generator.go:107","func":"github.com/intel/sriov-fec-operator/pkg/daemon.logIniFile","generated BBDevConfig":"[MODE]\npf_mode_en = 0\n\n[VFBUNDLES]\nnum_vf_bundles = 1\n\n[MAXQSIZE]\nmax_queue_size = 1024\n\n[QUL4G]\nnum_qgroups        = 2\nnum_aqs_per_groups = 16\naq_depth_log2      = 4\n\n[QDL4G]\nnum_qgroups        = 2\nnum_aqs_per_groups = 16\naq_depth_log2      = 4\n\n[QUL5G]\nnum_qgroups        = 2\nnum_aqs_per_groups = 16\naq_depth_log2      = 4\n\n[QDL5G]\nnum_qgroups        = 2\nnum_aqs_per_groups = 16\naq_depth_log2      = 4\n\n[QFFT]\nnum_qgroups        = 4\nnum_aqs_per_groups = 64\naq_depth_log2      = 4\n\n[QMLD]\nnum_qgroups        = 4\nnum_aqs_per_groups = 64\naq_depth_log2      = 4\n","level":"info","msg":"logIniFile","time":"2025-05-08T14:44:21Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:312","func":"github.com/intel/sriov-fec-operator/pkg/daemon.(*fftUpdater).VrbgetFftFilePath","level":"info","msg":"Using default SRS FFT file for configuration","time":"2025-05-08T14:44:21Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:197","func":"github.com/intel/sriov-fec-operator/pkg/daemon.(*pfBBConfigController).updateFftWindowsCoefficientFilepath","level":"info","msg":"SRS FFT file path is : /sriov_workdir/vrb2/srs_fft_windows_coefficient.bin","time":"2025-05-08T14:44:21Z"}
{"args":["/sriov_workdir/pf_bb_config","VRB2","-c","/tmp/0000:07:00.0.ini","-p","0000:07:00.0","-v","02bddbbf-bbb0-4d79-886b-91bad3fbb510","-f","/sriov_workdir/vrb2/srs_fft_windows_coefficient.bin"],"cmd":"/sriov_workdir/pf_bb_config","file":"/workspace-go/pkg/daemon/common.go:35","func":"github.com/intel/sriov-fec-operator/pkg/daemon.execAndSuppress","level":"info","msg":"executing command","time":"2025-05-08T14:44:21Z"}
{"file":"/workspace-go/pkg/daemon/common.go:48","func":"github.com/intel/sriov-fec-operator/pkg/daemon.execAndSuppress","level":"info","msg":"commands output","output":"== pf_bb_config Version v25.01-0-g812e032 ==\nVRB2 PF [0000:07:00.0] configuration complete!\nLog file = /var/log/pf_bb_cfg_0000:07:00.0.log\n","time":"2025-05-08T14:44:21Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Thu May  8 14:44:21 2025:INFO:Adjust PG on the device from 4 to 0","pciAddr":"0000:07:00.0","time":"2025-05-08T14:44:21Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Thu May  8 14:44:21 2025:INFO:Queue Groups UL4G 2 DL4G 2 UL5G 2 DL5G 2 FFT 4 MLD 4","pciAddr":"0000:07:00.0","time":"2025-05-08T14:44:21Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Thu May  8 14:44:21 2025:INFO:Configuration in VF mode","pciAddr":"0000:07:00.0","time":"2025-05-08T14:44:21Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Thu May  8 14:44:21 2025:INFO:  FFT Window coeffs preloading from /sriov_workdir/vrb2/srs_fft_windows_coefficient.bin on engine 0 ","pciAddr":"0000:07:00.0","time":"2025-05-08T14:44:21Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Thu May  8 14:44:21 2025:INFO:  FFT Size 2048 Window 0 Size 512 Start -256","pciAddr":"0000:07:00.0","time":"2025-05-08T14:44:21Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Thu May  8 14:44:21 2025:INFO:  FFT Version Number E74A","pciAddr":"0000:07:00.0","time":"2025-05-08T14:44:21Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Thu May  8 14:44:21 2025:INFO:PF VRB2 configuration complete","pciAddr":"0000:07:00.0","time":"2025-05-08T14:44:21Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Thu May  8 14:44:21 2025:INFO:VRB2 PF [0000:07:00.0] configuration complete!","pciAddr":"0000:07:00.0","time":"2025-05-08T14:44:21Z"}
{"file":"/workspace-go/pkg/daemon/bbdevconfig.go:409","func":"github.com/intel/sriov-fec-operator/pkg/daemon.monitorLogFile.func1","level":"info","msg":"Thu May  8 14:44:21 2025:INFO:Running in daemon mode for VFIO VF token","pciAddr":"0000:07:00.0","time":"2025-05-08T14:44:21Z"}
{"file":"/workspace-go/pkg/daemon/node_management.go:65","func":"github.com/intel/sriov-fec-operator/pkg/daemon.(*NodeConfigurator).isDeviceBoundToDriver","level":"info","msg":"device is bound to driver","path":"/sys/bus/pci/devices/0000:07:00.1/driver","time":"2025-05-08T14:44:21Z"}
```

### Retrieving Node Configuration
The user can observe changes in the FEC configuration. The created devices should appear similar to the following output. In this example, 0d5c is the Physical Function (PF) for the ACC100 card, while 57c0 is the PF for the VRB1 integrated solution, and 57c2 is the PF for the VRB2 integrated solution. Correspondingly, 0d5d is a Virtual Function (VF) for the ACC100, 57c1 is a VF for VRB1, and 57c3 is a VF for VRB2.

```shell
[user@ctrl1 /home]# oc get sriovfecnodeconfig node1 -o yaml
```

For a list of sample status outputs for applicable devices, see:
#### Sample Node Config Output for ACC100
```yaml
[user@ctrl1 /home]# oc get sriovfecnodeconfig node1 -o yaml
apiVersion: sriovfec.intel.com/v2
kind: SriovFecNodeConfig
metadata:
  creationTimestamp: "2025-05-07T22:31:43Z"
  generation: 1
  name: node1
  namespace: vran-acceleration-operators
  resourceVersion: "1435385"
  uid: 18cb6746-6a94-4802-836a-fada54a867e1
spec:
  physicalFunctions: []
status:
  conditions:
  - lastTransitionTime: "2025-05-07T22:32:33Z"
    message: ""
    observedGeneration: 1
    reason: NotRequested
    status: "False"
    type: Configured
  inventory:
    sriovAccelerators:
    - deviceID: 0d5c
      driver: vfio-pci
      maxVirtualFunctions: 16
      pciAddress: "0000:31:00.0"
      vendorID: "8086"
      virtualFunctions:
      - deviceID: 0d5d
        driver: vfio-pci
        pciAddress: "0000:32:00.0"
      - deviceID: 0d5d
        driver: vfio-pci
        pciAddress: "0000:32:00.1"
  pfBbConfVersion: v25.01-0-g812e032
```
#### Sample Node Config Output for VRB1
```yaml
[user@ctrl1 /home]# oc get sriovvrbnodeconfig node1 -o yaml
apiVersion: sriovvrb.intel.com/v1
kind: SriovVrbNodeConfig
metadata:
  creationTimestamp: "2025-05-07T22:53:44Z"
  generation: 2
  name: node1
  namespace: vran-acceleration-operators
  resourceVersion: "58673984"
  uid: 202cdb07-9ae3-4f86-9f99-f64477a7fc5c
spec:
  drainSkip: true
  physicalFunctions:
  - bbDevConfig:
      vrb1:
        downlink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 2
        downlink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 2
        fftLut:
          fftChecksum: ""
          fftUrl: ""
        maxQueueSize: 1024
        numVfBundles: 1
        qfft:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 2
        uplink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 2
        uplink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 2
    pciAddress: 0000:f7:00.0
    pfDriver: vfio-pci
    vfAmount: 1
    vfDriver: vfio-pci
    vrbResourceName: ""
status:
  conditions:
  - lastTransitionTime: "2025-05-07T22:56:02Z"
    message: Configured successfully
    observedGeneration: 2
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
  pfBbConfVersion: v25.01-0-g812e032
```
#### Sample Node Config Output for VRB2
```yaml
[user@ctrl1 /home]# oc get sriovvrbnodeconfig node1 -o yaml
apiVersion: sriovvrb.intel.com/v1
kind: SriovVrbNodeConfig
metadata:
  creationTimestamp: "2025-05-08T14:42:04Z"
  generation: 2
  name: node1
  namespace: vran-acceleration-operators
  resourceVersion: "48861353"
  uid: 4650cf89-dc37-4918-b87e-fd7d706f9f06
spec:
  drainSkip: true
  physicalFunctions:
  - bbDevConfig:
      vrb2:
        downlink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 2
        downlink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 2
        fftLut:
          fftChecksum: ""
          fftUrl: ""
        maxQueueSize: 1024
        numVfBundles: 1
        qfft:
          aqDepthLog2: 4
          numAqsPerGroups: 64
          numQueueGroups: 4
        qmld:
          aqDepthLog2: 4
          numAqsPerGroups: 64
          numQueueGroups: 4
        uplink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 2
        uplink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 2
    pciAddress: "0000:07:00.0"
    pfDriver: vfio-pci
    vfAmount: 1
    vfDriver: vfio-pci
    vrbResourceName: ""
status:
  conditions:
  - lastTransitionTime: "2025-05-08T14:44:28Z"
    message: Configured successfully
    observedGeneration: 2
    reason: Succeeded
    status: "True"
    type: Configured
  inventory:
    sriovAccelerators:
    - deviceID: 57c2
      driver: vfio-pci
      maxVirtualFunctions: 64
      pciAddress: "0000:07:00.0"
      vendorID: "8086"
      virtualFunctions:
      - deviceID: 57c3
        driver: vfio-pci
        pciAddress: "0000:07:00.1"
    - deviceID: 57c2
      driver: vfio-pci
      maxVirtualFunctions: 64
      pciAddress: 0000:0a:00.0
      vendorID: "8086"
      virtualFunctions: []
  pfBbConfVersion: v25.01-0-g812e032
```

### Deploying a sample test-bbdev pod
Once the SRIOV-FEC operator has set up and configured the device, users can test the device using a sample 'test-bbdev' application from the [DPDK project](https://github.com/DPDK/dpdk/tree/main/app/test-bbdev). To facilitate this, users are encouraged to create a sample Docker image of the DPDK application. This image will serve as a basis for deploying and testing the application within a Kubernetes environment. Please note that instructions on how to create a sample test-bbdev application are beyond the scope of this documentation.

With a sample image of the DPDK application, the following pod can be created similar to the example below (`intel.com/intel_fec_acc100` needs to be replaced as needed when a different accelerator is used):

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

### Telemetry
Operator exposes telemetry from pf-bb-config application for any supported card which uses `vfio-pci` PF driver in Prometheus format.
      It is available in `daemonset` container under `:8080/bbdevconfig` endpoint.
      By default endpoint updates metrics every 15 second, however this interval could be modified by
      changing value of `SRIOV_FEC_METRIC_GATHER_INTERVAL` env var in operators subscription.

There are 5 available metrics:
- bytes_processed_per_vfs - represents number of bytes that are processed by VF
  - `pci_address` - represents unique BDF for VF
  - `queue_type` - represents queue type for VF. Available values:
    - VRB1: `5GDL`, `5GUL`, `FFT`
    - VRB2: `5GDL`, `5GUL`, `FFT`, `4GDL`, `4GUL`, `MLD`
- code_blocks_per_vfs - number of code blocks processed by VF
  - `pci_address` - represents unique BDF for VF
  - `queue_type` - represents queue type for VF. Available values:
    - VRB1: `5GDL`, `5GUL`, `FFT`
    - VRB2: `5GDL`, `5GUL`, `FFT`, `4GDL`, `4GUL`, `MLD`
- counters_per_engine - number of code blocks processed by Engine
  - `engine_id` - represents integer ID of engine on card
  - `pci_address` - represents unique BDF for card on which engine is located
  - `queue_type` - represents queue type for VF. Available values:
    - VRB1: `5GDL`, `5GUL`, `FFT`
    - VRB2: `5GDL`, `5GUL`, `FFT`, `4GDL`, `4GUL`, `MLD`
- vf_count - describes number of configured VFs on card
  - `pci_address` - represents unique BDF for PF
  - `status` - represents current status of SriovFecNodeConfig. Available values: `InProgress`, `Succeeded`, `Failed`, `Ignored`
- vf_status - equals to 1 if `status` is `RTE_BBDEV_DEV_CONFIGURED` or `RTE_BBDEV_DEV_ACTIVE` and 0 otherwise
  - `pci_address` - represents unique BDF for VF
  - `status` - represents status as exposed by pf-bb-config. Available values: `RTE_BBDEV_DEV_NOSTATUS`, `RTE_BBDEV_DEV_NOT_SUPPORTED`, `RTE_BBDEV_DEV_RESET`,
    `RTE_BBDEV_DEV_CONFIGURED`, `RTE_BBDEV_DEV_ACTIVE`, `RTE_BBDEV_DEV_FATAL_ERR`, `RTE_BBDEV_DEV_RESTART_REQ`, `RTE_BBDEV_DEV_RECONFIG_REQ`, `RTE_BBDEV_DEV_CORRECT_ERR`

Note: VRB1 can process 4G DL/UL operations but it does not have telemetry counters for such operations.

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

- Intel® vRAN Dedicated Accelerator ACC100
- 2nd Generation Intel® Xeon® processor platform

## Summary

The SRIOV-FEC Operator for Wireless FEC Accelerators is a fully functional tool to manage the vRAN FEC resources autonomously in a Cloud Native OpenShift environment based on the user input.
The operator handles all the necessary actions from creation of FEC resources to configuration and management of the resources within the OpenShift cluster.

## Appendix 1 - Developer Notes

### Drain skip option

Using the option `spec.drainSkip: false` in CR will perform the [node drain](https://kubernetes.io/docs/reference/kubectl/generated/kubectl_drain/) while applying the configuration. If you do not want to drain the node during the CR apply, set this option to `true` which is the default behavior.

### VrbResourceName (Optional)

Using the `sriovvrbclusterconfig.spec.vrbResourceName` allows you to specify a custom resource name for the sriov-device-plugin specific to VRB2 with multiple accelerators. If not provided, the default resource name `intel_vrb_vrb2` will be used. Using this option will link the custom `vrbResourceName` to a specific VRB2 physical function.
If the `vrbResourceName` option is used, the `sriovvrbclusterconfig` must specify the `sriovvrbclusterconfig.spec.acceleratorSelector.pciAddress` to ensure proper linkage and configuration of the VRB2 physical function.

- **Description**: Indicates a custom resource name for the sriov-device-plugin specific to VRB2.
- **Type**: `string`
- **Optional**: Yes
- **Pattern**: `^[a-zA-Z0-9-_]+$`

**Limitations:**
- In the case of dual VRB2 devices in one node, if `vrbResourceName` is used in one device CR, then it is mandatory to use it in the CR for the second device.
- Once `vrbResourceName` is set, it cannot be removed from the CR; it can only be renamed.
- It is mandatory to have different `vrbResourceName` values across different sriovvrbclusterconfig. If the same `vrbResourceName` is used in multiple CRs, the sriov-device-plugin will crash. This can be fixed by updating one of the CRs to use a different `vrbResourceName` and re-applying the configuration.


## Appendix 2 - Reference CR configurations for supported accelerators in SRIOV-FEC Operator

### ACC100
- Reference CR for ACC100
```yaml
apiVersion: sriovfec.intel.com/v2
kind: SriovFecClusterConfig
metadata:
  name: config
  namespace: vran-acceleration-operators
spec:
  priority: 1
  nodeSelector:
    kubernetes.io/hostname: worker-node
  acceleratorSelector:
    pciAddress: 0000:af:00.0
  physicalFunction:  
    pfDriver: "vfio-pci"
    vfDriver: "vfio-pci"
    vfAmount: 2
    bbDevConfig:
      acc100:
        pfMode: false
        numVfBundles: 2
        maxQueueSize: 1024
        uplink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 0
        downlink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 0
        uplink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        downlink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
```

### vRAN Boost Accelerator V1 (VRB1)

- Reference CR for VRB1

```yaml
apiVersion: sriovvrb.intel.com/v1
kind: SriovVrbClusterConfig
metadata:
  name: config
  namespace: vran-acceleration-operators
spec:
  priority: 1
  nodeSelector:
    kubernetes.io/hostname: worker-node
  acceleratorSelector:
    pciAddress: 0000:f7:00.0
  drainSkip: true
  physicalFunction:
    pfDriver: vfio-pci
    vfDriver: vfio-pci
    vfAmount: 2
    bbDevConfig:
      vrb1:
        pfMode: false
        numVfBundles: 2
        maxQueueSize: 1024
        uplink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 0
        downlink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 0
        uplink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        downlink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        qfft:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
```

- Reference CR for ACC200 (Deprecated)

> **_NOTE:_** The use of the `sriovfec/v2` API for ACC200 is deprecated. Intel recommends transitioning to the `sriovvrb/v1` API for future configurations. While the `sriovfec/v2` API will continue to support existing applications using ACC200, new users are strongly encouraged to utilize the `sriovvrb/v1` API for configuring VRB1 devices. It is important to note that there is **NO** functional difference between the `sriovvrb/v1` and `sriovfec/v2` APIs concerning the VRB1 accelerator device within the Operator.

```yaml
apiVersion: sriovfec.intel.com/v2
kind: SriovFecClusterConfig
metadata:
  name: config
  namespace: vran-acceleration-operators
spec:
  priority: 1
  nodeSelector:
    kubernetes.io/hostname: worker-node
  acceleratorSelector:
    pciAddress: 0000:f7:00.0
  drainSkip: true
  physicalFunction:
    pfDriver: vfio-pci
    vfDriver: vfio-pci
    vfAmount: 2
    bbDevConfig:
        pfMode: false
        numVfBundles: 2
        maxQueueSize: 1024
      acc200:
        uplink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 0
        downlink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 0
        uplink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        downlink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        qfft:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
```

### vRAN Boost Accelerator V2 (VRB2)

- Reference CR for VRB2

```yaml
apiVersion: sriovvrb.intel.com/v1
kind: SriovVrbClusterConfig
metadata:
  name: config
  namespace: vran-acceleration-operators
spec:
  priority: 1
  nodeSelector:
    kubernetes.io/hostname: worker-node
  acceleratorSelector:
    pciAddress: 0000:07:00.0
  drainSkip: true
  physicalFunction:
    pfDriver: vfio-pci
    vfDriver: vfio-pci
    vfAmount: 2
    bbDevConfig:
      vrb2:
        pfMode: false
        numVfBundles: 2
        maxQueueSize: 1024
        downlink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 0
        uplink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 0
        downlink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        uplink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        qfft:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        qmld:
          aqDepthLog2: 4
          numAqsPerGroups: 64
          numQueueGroups: 4
```

- Reference CR for VRB2 using optional VrbResourceName

```yaml
apiVersion: sriovvrb.intel.com/v1
kind: SriovVrbClusterConfig
metadata:
  name: config
  namespace: vran-acceleration-operators
spec:
  priority: 1
  nodeSelector:
    kubernetes.io/hostname: worker-node
  acceleratorSelector:
    pciAddress: 0000:07:00.0
  drainSkip: true
  physicalFunction:
    pfDriver: vfio-pci
    vfDriver: vfio-pci
    vfAmount: 2
    bbDevConfig:
      vrb2:
        pfMode: false
        numVfBundles: 2
        maxQueueSize: 1024
        downlink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 0
        uplink4G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 0
        downlink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        uplink5G:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        qfft:
          aqDepthLog2: 4
          numAqsPerGroups: 16
          numQueueGroups: 4
        qmld:
          aqDepthLog2: 4
          numAqsPerGroups: 64
          numQueueGroups: 4
  vrbResourceName: "intel_vrb_vrb2_1"
```

## Appendix 3 - Gathering logs for bug report
To gather logs for filing bug report please run `gather_sriovfec_logs.sh` script downloaded from https://github.com/smart-edge-open/sriov-fec-operator/blob/main/gather_sriovfec_logs.sh

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

## Appendix 4 - Additional instructions for applications using VF interface in case of VFIO mode

As described in [pf-bb-config application documentation](https://github.com/intel/pf-bb-config?tab=readme-ov-file#using-vfio-pci-driver),  In case of VFIO mode after applying the configuration through CR, pf-bb-config application runs in a daemon mode inside the Operator daemon POD all the time, and this is a must requirement for the DU application to be able to consciously use the VF interface.   Below are additional information and instructions that developer should be aware of while using VF interfaces in VFIO mode:

- If the user wants to update/modify existing accelerator configuration, DU application has to stop and release the VF resource before changing or deleting the configuration.
- It is not expected to happen that Operator daemon pod to be killed and restarted by itself, but for any external event reason (one such example is probe failures) if the daemon pod happens to be restarted, then pf-bb-config application run in the daemon mode will also be terminated and will be restarted and reconfigure the accelerator in the new instance of daemon pod that deployed automatically.
- In the event of daemon pod restarts, DU application may not be able to use the VF interface. To recover from this state, DU application should release the VF interface and reconfigure VF interface (or DU application terminate and restart) as referenced in pf-bb-config documentation.
- Reset of other Operator pods ie., manager and labeler will not cause any interruption for application to use VF interface.
