```text
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2020-2021 Intel Corporation
```
<!-- omit in toc -->
# OpenNESS Operator for Intel® FPGA PAC N3000 documentation

- [Overview](#overview)
- [OpenNESS Operator for Intel® FPGA PAC N3000 (Programming)](#openness-operator-for-intel-fpga-pac-n3000-programming)
  - [Intel® FPGA PAC N3000 (Programming)](#intel-fpga-pac-n3000-programming)
    - [Telemetry](#telemetry)
    - [Driver Container](#driver-container)
    - [N3000 Daemon](#n3000-daemon)
      - [OPAE RTL Update](#opae-rtl-update)
      - [NVM Update](#nvm-update)
- [Technical Requirements and Dependencies](#technical-requirements-and-dependencies)
- [Deploying the Operator](#deploying-the-operator)
  - [Install the Bundle](#install-the-bundle)
  - [Applying Custom Resources](#applying-custom-resources)
- [Hardware Validation Environment](#hardware-validation-environment)
- [Summary](#summary)
- [Appendix 1 - Developer Notes](#appendix-1---developer-notes)
  - [Uninstalling Previously Installed Operator](#uninstalling-previously-installed-operator)
  - [Setting Up Operator Registry Locally](#setting-up-operator-registry-locally)
- [Appendix 2 - OpenNESS Operator for Intel® FPGA PAC N3000 (Programming)](#appendix-2---openness-operator-for-intel-fpga-pac-n3000-programming)
  - [N3000 Programming](#n3000-programming)
    - [Sample CR for N3000 programming (N3000)](#sample-cr-for-n3000-programming-n3000)
    - [Sample Status for N3000 programming (N3000)](#sample-status-for-n3000-programming-n3000)
    - [Sample Daemon log for N3000 programming (N3000)](#sample-daemon-log-for-n3000-programming-n3000)

## Overview

This document provides the instructions for using the OpenNESS Operator for Intel® FPGA PAC N3000 (Programming) in Red Hat's OpenShift Container Platform. This operator was developed with aid of the Special Resource Operator framework based on the Operator SDK project.

## OpenNESS Operator for Intel® FPGA PAC N3000 (Programming)

The role of the OpenNESS Operator for Intel® FPGA PAC N3000 (Programming) is to orchestrate and manage the resources/devices exposed by the Intel® FPGA PAC N3000 card within the OpenShift cluster. The operator is a state machine which will configure the resources and then monitor them and act autonomously based on the user interaction. For vRAN use-cases it is expected that the operator is used alongside the [OpenNESS Operator for Intel Wireless FEC Accelerator.](https://github.com/otcshare/openshift-operator/blob/master/spec/openshift-sriov-fec-operator.md)
The operator design of the OpenNESS Operator for Intel® FPGA PAC N3000 (Programming) supports the following device:

* [Intel® PAC N3000 for vRAN Acceleration](https://github.com/otcshare/openshift-operator/blob/master/spec/vran-accelerators-supported-by-operator.md#intel-pac-n3000-for-vran-acceleration)

### Intel® FPGA PAC N3000 (Programming)

This operator handles the management of the FPGA configuration. It provides functionality to load the necessary drivers, allows the user to program the Intel® FPGA PAC N3000 user image and to update the firmware of the Intel® XL710 NICs (Network Interface Cards). It also deploys an instance of Prometheus Exporter which collects metrics from the Intel® FPGA PAC N3000 card. The user interacts with the operator by providing a CR (Custom Resource). The operator constantly monitors the state of the CR to detect any changes and acts based on the changes detected. The CR is provided per cluster configuration, and the components for individual nodes can be configured by specifying appropriate values for each component per "nodeName". The operator attempts to download the FPGA user image and the XL710 firmware from a location specified in the CR. The user is responsible for providing a HTTP server from which the files can be downloaded. The user is also responsible for provisioning of the PCI address of the RSU interface from the Intel® FPGA PAC N3000 used to program the FPGA's user image, as well as MAC addresses of the XL710 NICs to be updated with the new firmware. The user does not have to program both the FPGA and the XL710 component at the same time.

> Note: The Update of the PAC N3000's Intel® MAX® 10 BMC image is currently not supported by the operator. While it is possible to load the BMC image through the same mechanism as loading the user image, the BMC version will not be tracked by the operator. Additionally the outcome of such action cannot be guaranteed.

For more details, refer to:
- Intel® FPGA PAC N3000 5G User image - [AN 907: Enabling 5G Wireless Acceleration in FlexRAN: for the Intel® FPGA Programmable Acceleration Card N3000](https://www.intel.com/content/www/us/en/programmable/documentation/ocl1575542673666.html)
- Intel PAC N3000 Data Sheet - [Intel FPGA Programmable Acceleration Card N3000 Data Sheet](https://www.intel.com/content/dam/www/programmable/us/en/pdfs/literature/ds/ds-pac-n3000.pdf)

![5G user image](images/Intel-N3000-5G-user-image.png)

An example CR for the OpenNESS Operator for Intel® FPGA PAC N3000 (Programming) can be found at:

* [Sample CR for N3000 programming (N3000)](#sample-cr-for-n3000-programming-n3000)

The workflow of the N3000 operator is shown in the following diagram:
![Intel® FPGA PAC N3000 Operator Design](images/n3000_operator.png)

#### Telemetry

During the deployment of the N3000 operator a Prometheus exporter is deployed on each node. This exporter is responsible for gathering Intel® FPGA PAC N3000 telemetry such as temperature, voltage and power consumption. The statistics are collected using the (Open Programmable Acceleration Engine) OPAE's 'fpgainfo' tool and can be scraped by a Prometheus instance.

#### Driver Container

The driver container contains pre-built OPAE drivers built for a specific version of the node's kernel. This container is deployed as a DaemonSet on each applicable node; on deployment it mounts the required drivers onto the nodes filesystem and once it has finished executing its purpose, it sleeps indefinitely.

#### N3000 Daemon

The N3000 Daemon is part of the operator. It is a DaemonSet deployed on each applicable node. It is a reconcile loop which monitors the changes in each node's CR and acts on the changes. The logic implemented into this Daemon takes care of updating the cards' FPGA user image and NIC firmware. It is also responsible for draining the nodes and taking them out of commission when required by the update.

##### OPAE RTL Update

Once the operator/daemon detects a change to a CR related to the update of the FPGA user image, it tries to perform an update. It checks whether the card is already programmed with the current image, and accordingly either continues with an update and takes the node out of commission, if required, or reports back to the user that the image version loaded is up to date. The user image file with the program for the FPGA is expected to be provided by the user. The user is also responsible to sign the user image using PACSign to produce a signed user image with SSL Keys or an unsigned image without the keys, [the OPAE Documentation provides more details](https://www.intel.com/content/www/us/en/programmable/documentation/dlq1585950463484.html). The user is required to place the user image file on an accessible HTTP server and provide an URL for it in the CR. If the file is provided correctly and the image is to be updated, the N3000 Daemon will update the FPGA user image using the OPAE tools provided in its Docker image and reset the PCI device. The update of the FPGA user image may take up to 40 minutes per card. For programming cards on multiple nodes, the programming will happen only one node at a time.

As an example for the vRAN use-case, the card is to be programmed with an FEC image for either Turbo (4G) or LDPC (5G) - [see the product table](https://www.intel.com/content/www/us/en/programmable/products/boards_and_kits/dev-kits/altera/intel-fpga-pac-n3000/overview.html).

To get all the nodes containing the Intel® FPGA PAC N3000 card run the following command (all the commands are run in the `vran-acceleration-operators` namespace):
```shell
[user@ctrl1 /home]# oc get n3000node

NAME                       FLASH
node1                      NotRequested
```

To get the information about the card on each node run:

```shell
[user@ctrl1 /home]# oc get n3000node node1 -o yaml

***
status:
  conditions:
  - lastTransitionTime: "2020-12-15T17:09:26Z"
    message: Inventory up to date
    observedGeneration: 1
    reason: NotRequested
    status: "False"
    type: Flashed
  fortville:
  - N3000PCI: 0000:1b:00.0
    NICs:
    - MAC: 64:4c:36:11:1b:a8
      NVMVersion: 7.00 0x800052b0 0.0.0
      PCIAddr: 0000:1a:00.0
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
    - MAC: 64:4c:36:11:1b:a9
      NVMVersion: 7.00 0x800052b0 0.0.0
      PCIAddr: 0000:1a:00.1
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
    - MAC: 64:4c:36:11:1b:ac
      NVMVersion: 7.00 0x800052b0 0.0.0
      PCIAddr: 0000:1c:00.0
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
    - MAC: 64:4c:36:11:1b:ad
      NVMVersion: 7.00 0x800052b0 0.0.0
      PCIAddr: 0000:1c:00.1
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
  fpga:
  - PCIAddr: 0000:1b:00.0
    bitstreamId: "0x23000410010310"
    bitstreamVersion: 0.2.3
    deviceId: "0x0b30"
```

To update the user image of the Intel® FPGA PAC N3000 card user must create a CR containing the information about which node and which card should be programmed.

```yaml
apiVersion: fpga.intel.com/v1
kind: N3000Cluster
metadata:
  name: n3000
  namespace: vran-acceleration-operators
spec:
  nodes:
    - nodeName: "node1"
      fpga:
        - userImageURL: "http://10.10.10.122:8000/pkg/20ww27.5-2x2x25G-5GLDPC-v1.6.1-3.0.0_unsigned.bin"
          PCIAddr: "0000:1b:00.0"
          checksum: "0b0a87b974d35ea16023ceb57f7d5d9c"
```

To apply the CR run:

```shell
[user@ctrl1 /home]# oc apply -f <fpga_cr_name>.yaml
```

After provisioning of appropriate user image (ie. 5G FEC image - '2x2x25G-5GLDPC-v1.6.1-3.0.0-unsigned.bin' used in this example), and a creation of the CR, the N3000 daemon starts programming the image. To see the status run following command:

```shell
[user@ctrl1 /home]# oc get n3000node

NAME                       FLASH
node1                      InProgress
```

The logs similar to the output below will be created in the N3000 daemon's pod:
```shell
[user@ctrl1 /home]# oc get pod | grep n3000-daemonset
n3000-daemonset-5k55l                          1/1     Running   0          18h

[user@ctrl1 /home]# oc logs n3000-daemonset-5k55l

***
{"level":"info","ts":1608054338.8866854,"logger":"daemon.drainhelper.cordonAndDrain()","msg":"node drained"}
{"level":"info","ts":1608054338.8867319,"logger":"daemon.drainhelper.Run()","msg":"worker function - start"}
{"level":"info","ts":1608054338.9003832,"logger":"daemon.fpgaManager.ProgramFPGAs","msg":"Start program","PCIAddr":"0000:1b:00.0"}
{"level":"info","ts":1608054338.9004142,"logger":"daemon.fpgaManager.ProgramFPGA","msg":"Starting","pci":"0000:1b:00.0"}
{"level":"info","ts":1608056309.9367146,"logger":"daemon.fpgaManager.ProgramFPGA","msg":"Program FPGA completed, start new power cycle N3000 ...","pci":"0000:1b:00.0"}
{"level":"info","ts":1608056333.3528838,"logger":"daemon.drainhelper.Run()","msg":"worker function - end","performUncordon":true}
***
```

Once the FPGA user image update is complete, the following status is reported:

```shell
[user@ctrl1 /home]# oc get n3000node
NAME                       FLASH
node1                      Succeeded
```

The user can observe the changed BitStream ID of the card:

```yaml
[user@ctrl1 /home]# oc get n3000node node1 -o yaml

***

status:                                                                                                 
  conditions:                                                                                           
  - lastTransitionTime: "2020-12-15T18:18:53Z"                                                          
    message: Flashed successfully                                                                       
    observedGeneration: 2                                                                               
    reason: Succeeded                                                                                   
    status: "True"                                                                                      
    type: Flashed                                                                                       
  fortville:                                                                                            
  - N3000PCI: 0000:1b:00.0                                                                              
    NICs:                                                                                               
    - MAC: 64:4c:36:11:1b:a8                                                                            
      NVMVersion: 7.00 0x800052b0 0.0.0                                                                 
      PCIAddr: 0000:1a:00.0                                                                             
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
    - MAC: 64:4c:36:11:1b:a9           
      NVMVersion: 7.00 0x800052b0 0.0.0                                                                 
      PCIAddr: 0000:1a:00.1                                                                             
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
    - MAC: 64:4c:36:11:1b:ac           
      NVMVersion: 7.00 0x800052b0 0.0.0                                                                 
      PCIAddr: 0000:1c:00.0                                                                             
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
    - MAC: 64:4c:36:11:1b:ad           
      NVMVersion: 7.00 0x800052b0 0.0.0                                                                 
      PCIAddr: 0000:1c:00.1                                                                             
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
  fpga:                            
  - PCIAddr: 0000:1b:00.0          
    bitstreamId: "0x2315842A010601"
    bitstreamVersion: 0.2.3
    deviceId: "0x0b30
```

For extra verification user can check the FEC PCI devices from the node and expect the following output (Devices belonging to the FPGA are reported in the output, where Device ID '0b30' is the RSU interface used to program the card, and the '0d8f' is a Physical Function of the newly programmed FEC device):

```shell
[user@node1 /home]# lspci | grep accelerators
1b:00.0 Processing accelerators: Intel Corporation Device 0b30
1d:00.0 Processing accelerators: Intel Corporation Device 0d8f (rev 01)
```

##### NVM Update

Once the operator/daemon detects a change to a CR related to the update of the Intel® XL710 firmware, it tries to perform an update. It checks whether the card is already programmed with the current firmware, and accordingly either continues with an update and takes the node out of commission, if required, or reports back to the user that the firmware version loaded is up to date. The firmware for the Intel® XL710 NICs is expected to be provided by the user. The user is also responsible to verify that the firmware version is compatible with the device, see [the NVM utility link](https://downloadcenter.intel.com/download/24769/Non-Volatile-Memory-NVM-Update-Utility-for-Intel-Ethernet-Network-Adapter-700-Series). The user is required to place the firmware on an accessible HTTP server and provide an URL for it in the CR. If the file is provided correctly and the firmware is to be updated, the N3000 Daemon will update the Intel® XL710 NICs with the NVM utility provided.

To get all the nodes containing the Intel® FPGA PAC N3000 card run the following command (all the commands are run in the `vran-acceleration-operators` namespace):
```shell
[user@ctrl1 /home]# oc get n3000node

NAME                       FLASH
node1                      NotRequested
```

To find the NIC devices belonging to the Intel® FPGA PAC N3000 run following command, the user can detect the device information of the NICs from the output:

```shell
[user@ctrl1 /home]# oc get n3000node node1 -o yaml

***
status:
  conditions:
  - lastTransitionTime: "2020-12-15T17:09:26Z"
    message: Inventory up to date
    observedGeneration: 1
    reason: NotRequested
    status: "False"
    type: Flashed
  fortville:
  - N3000PCI: 0000:1b:00.0
    NICs:
    - MAC: 64:4c:36:11:1b:a8
      NVMVersion: 7.00 0x800052b0 0.0.0
      PCIAddr: 0000:1a:00.0
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
    - MAC: 64:4c:36:11:1b:a9
      NVMVersion: 7.00 0x800052b0 0.0.0
      PCIAddr: 0000:1a:00.1
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
    - MAC: 64:4c:36:11:1b:ac
      NVMVersion: 7.00 0x800052b0 0.0.0
      PCIAddr: 0000:1c:00.0
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
    - MAC: 64:4c:36:11:1b:ad
      NVMVersion: 7.00 0x800052b0 0.0.0
      PCIAddr: 0000:1c:00.1
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
  fpga:
  - PCIAddr: 0000:1b:00.0
    bitstreamId: "0x23000410010310"
    bitstreamVersion: 0.2.3
    deviceId: "0x0b30"
```

To update the NVM firmware of the Intel® FPGA PAC N3000 cards' NICs user must create a CR containing the information about which node and which card should be programmed. The Physical Functions of the NICs will be updated in logical pairs.

```yaml
apiVersion: fpga.intel.com/v1
kind: N3000Cluster
metadata:
  name: n3000
  namespace: vran-acceleration-operators
spec:
  nodes:
    - nodeName: "node1"
      fortville:
        firmwareURL: "http://10.103.102.122:8000/7.30/700Series_NVMUpdatePackage_v7_30_Linux.tar.gz"
        checksum: "0b0a87b974d35ea16023ceb57f7d5d9c"
        MACs:
          - MAC: "64:4c:36:11:1b:a8"
```

To apply the CR run:

```shell
[user@ctrl1 /home]# oc apply -f <nic_cr_name>.yaml
```

After provisioning of appropriate NVM NIC firmware package, and a creation of the CR, the N3000 daemon starts programming the NICs firmware. To see the status run following command:

```shell
[user@ctrl1 /home]# oc get n3000node

NAME                       FLASH
node1                      InProgress
```

Once the NVM firmware update is complete, the following status is reported:

```shell
[user@ctrl1 /home]# oc get n3000node
NAME                       FLASH
node1                      Succeeded
```

The user can observe the change of the cards' NICs firmware:

```yaml
[user@ctrl1 /home]# oc get n3000node node1 -o yaml

***

status:                                                                                                 
  conditions:                                                                                           
  - lastTransitionTime: "2020-12-15T19:14:43Z"                                                          
    message: Flashed successfully                                                                       
    observedGeneration: 2                                                                               
    reason: Succeeded                                                                                   
    status: "True"                                                                                      
    type: Flashed                                                                                       
  fortville:                                                                                            
  - N3000PCI: 0000:1b:00.0                                                                              
    NICs:                                                                                               
    - MAC: 64:4c:36:11:1b:a8                                                                            
      NVMVersion: 7.30 0x80008360 0.0.0                                                                 
      PCIAddr: 0000:1a:00.0                                                                             
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
    - MAC: 64:4c:36:11:1b:a9           
      NVMVersion: 7.30 0x80008360 0.0.0                                                                 
      PCIAddr: 0000:1a:00.1                                                                             
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
    - MAC: 64:4c:36:11:1b:ac           
      NVMVersion: 7.00 0x800052b0 0.0.0                                                                 
      PCIAddr: 0000:1c:00.0                                                                             
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
    - MAC: 64:4c:36:11:1b:ad           
      NVMVersion: 7.00 0x800052b0 0.0.0                                                                 
      PCIAddr: 0000:1c:00.1                                                                             
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
  fpga:                            
  - PCIAddr: 0000:1b:00.0          
    bitstreamId: "0x2315842A010601"
    bitstreamVersion: 0.2.3
    deviceId: "0x0b30
```

## Technical Requirements and Dependencies

The PACN3000 Operator bundle has the following requirements:

- [Intel® FPGA PAC N3000 card](https://www.intel.com/content/www/us/en/programmable/products/boards_and_kits/dev-kits/altera/intel-fpga-pac-n3000/overview.html)
- vRAN RTL image for the Intel® FPGA PAC N3000 card
- [NVM utility](https://downloadcenter.intel.com/download/24769/Non-Volatile-Memory-NVM-Update-Utility-for-Intel-Ethernet-Network-Adapter-700-Series)
- [OpenShift 4.6.16](https://www.redhat.com/en/openshift-4/features?adobe_mc_sdid=SDID%3D3DA5D7008646C094-1B97A001FC92CC4A%7CMCORGID%3D945D02BE532957400A490D4C%40AdobeOrg%7CTS%3D1608134794&adobe_mc_ref=https%3A%2F%2Fwww.google.com%2F&sc_cid=7013a00000260opAAAutm_medium%3DSearch&utm_source=RedHat&utm_content=000040NZ&utm_term=10014427&utm_id=SEM_RHMP_NonBrand_BMM_Openshift%7CGen_NA&utm_campaign=Red-Hat_Openshift&cm_mmc=Search_RedHat-_-Open%20Marketplace_Open%20Marketplace-_-WW_WW-_-SEM_RHMP_NonBrand_BMM_Openshift%7CGen_NA&cm_mmca1=000040NZ&cm_mmca2=10014427&cm_mmca3=Red-Hat_Openshift&gclid=CjwKCAiA_eb-BRB2EiwAGBnXXg0avYbi2BsHdp9wL4DzavziWPcnGpqZbU0fpZ3-xOV2REuyKgQPThoCZ3QQAvD_BwE&gclsrc=aw.ds)
- RT Kernel configured with [Performance Addon Operator](https://access.redhat.com/documentation/en-us/openshift_container_platform/4.6/html/scalability_and_performance/cnf-performance-addon-operator-for-low-latency-nodes) (the OPAE Docker images are built for specific kernel version).

## Deploying the Operator

The OpenNESS Operator for Intel® FPGA PAC N3000 (Programming) is easily deployable from the OpenShift cluster via provisioning and application of the following YAML spec files:

### Install the Bundle

To install the PACN3000 operator bundle perform the following steps:

Create the project:

```shell
[user@ctrl1 /home]# oc new-project vran-acceleration-operators
```

Create an operator group and the subscriptions (all the commands are run in the `vran-acceleration-operators` namespace):

```shell
[user@ctrl1 /home]#  cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: n3000-operators
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
  name: n3000-subscription
  namespace: vran-acceleration-operators 
spec:
  channel: stable
  name: n3000
  source: certified-operators
  sourceNamespace: openshift-marketplace
EOF
```

Verify that the operators are installed and pods are running:

```shell
[user@ctrl1 /home]# oc get csv

NAME               DISPLAY                                        VERSION   REPLACES   PHASE
n3000.v1.1.0       Intel® FPGA PAC N3000 Operator                 1.1.0                Succeeded
```

```shell
[user@ctrl1 /home]# oc get pod

NAME                                            READY   STATUS    RESTARTS   AGE                                                                              
fpga-driver-daemonset-pkc6m                     1/1     Running   0          43s                                                                              
fpgainfo-exporter-gmsnk                         1/1     Running   0          44s                                                                              
n3000-controller-manager-6f6cfdbf6d-5sv5x       2/2     Running   0          52s                                                                              
n3000-daemonset-4lf7q                           1/1     Running   0          44s                                                                              
n3000-discovery-4zx49                           1/1     Running   0          44s                                                                              
n3000-discovery-sq25x                           1/1     Running   0          44s                                                                              
n3000-discovery-zfg6g                           1/1     Running   0          44s                                        
```

### Applying Custom Resources

Once the operator is successfully deployed, the user interacts with it by creating CRs which will be interpreted by the operators, for examples of CRs see the following sections:

- [N3000 Daemon](#n3000-daemon)
- [NVM Update](#nvm-update)

To apply a CR run:

```shell
[user@ctrl1 /home]# oc apply -f <cr-name>
```

To view the status of current CR run (sample output):

```shell
[user@ctrl1 /home]# oc get n3000cluster n3000 -o yaml 
***
spec:
  nodes:
  - fortville:
      MACs:
      - MAC: 64:4c:36:11:1b:a8
      checksum: 0b0a87b974d35ea16023ceb57f7d5d9c
      firmwareURL: http://10.10.10.122:8000/7.30/700Series_NVMUpdatePackage_v7_30_Linux.tar.gz
    nodeName: node1
```

## Hardware Validation Environment 
- Intel® FPGA PAC N3000-2
- Intel® FPGA PAC N3000-N
- 2nd Generation Intel® Xeon® processor platform

## Summary

The OpenNESS Operator for Intel® FPGA PAC N3000 (Programming) is a fully functional tool to manage the Intel® FPGA PAC N3000 card and its resources autonomously in a Cloud Native OpenShift environment based on the user input.
The operator handles all the necessary actions from programming/updating the FPGA to configuration and management of the resources within the OpenShift cluster.

## Appendix 1 - Developer Notes

### Uninstalling Previously Installed Operator

If the operator has been previously installed, the user needs to perform the following steps to delete the operator deployment.

Use the following command to identify items to delete:

```shell
[user@ctrl1 /home]# oc get csv -n vran-acceleration-operators

NAME               DISPLAY                                        VERSION   REPLACES   PHASE
n3000.v1.1.0       Intel® FPGA PAC N3000 Operator                 1.1.0                Succeeded
```

Then delete the items and the namespace:

```shell
[user@ctrl1 /home]# oc delete csv n3000.v1.1.0
[user@ctrl1 /home]# oc delete ns vran-acceleration-operators
```

### Setting Up Operator Registry Locally

If needed the user can set up a local registry for the operators' images.

Prerequisite: Make sure that the images used by the operator are pushed to LOCAL_REGISTRY

The operator-sdk CLI is required - see [Getting started with the Operator SDK](https://docs.openshift.com/container-platform/4.6/operators/operator_sdk/osdk-getting-started.html#osdk-installing-cli_osdk-getting-started).

Install OPM (if not already installed):

```shell
# RELEASE_VERSION=v1.15.3
# curl -LO https://github.com/operator-framework/operator-registry/releases/download/${RELEASE_VERSION}/linux-amd64-opm
# chmod +x linux-amd64-opm
# sudo mkdir -p /usr/local/bin/
# sudo cp linux-amd64-opm /usr/local/bin/opm
# rm -f linux-amd64-opm
```

Determine local registry address:

```shell
# export LOCAL_IMAGE_REGISTRY=<IP_ADDRESS>:<PORT>
```

Determine OS kernel version running on the node containing the card:

```shell
# export TEST_KERNEL_VERSION=<KERNEL_VERSION>
```

Determine path to operator repository:

```shell
# export TEST_OPERATOR_REPO_PATH=`readlink -f openshift-operator/`
```

Navigate to operator repository path:

```shell
# cd ${TEST_OPERATOR_REPO_PATH}
```

Copy OPAE installation file into ${TEST_OPERATOR_REPO_PATH}/files/opae and/or custom kernel RPM into ${TEST_OPERATOR_REPO_PATH}/files/kernel if desired (Set and pass the `KERNEL_SOURCE=file` environmental variable when using custom kernel RPM with `make build all`).

Build and push images to local registry:

```shell
# IMAGE_REGISTRY=${LOCAL_IMAGE_REGISTRY} TLS_VERIFY=false KERNEL_VERSION=${TEST_KERNEL_VERSION} make build_all
```

Create and push the index image:

```shell
# IMAGE_REGISTRY=${LOCAL_IMAGE_REGISTRY} TLS_VERIFY=false make build_index
```

Create the catalog source:

```shell
# cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
    name: intel-operators
    namespace: openshift-marketplace
spec:
    sourceType: grpc
    image: ${LOCAL_REGISTRY}/n3000-operators-index:1.1.0
    publisher: Intel
    displayName: N3000 operators(Local)
EOF
```

Wait for `packagemanifest` to be available:

```shell
[user@ctrl1 /home]# oc get packagemanifests n3000 sriov-fec

 NAME        CATALOG                  AGE
 n3000       N3000 operators(Local)   24s
 sriov-fec   N3000 operators(Local)   24s
```

## Appendix 2 - OpenNESS Operator for Intel® FPGA PAC N3000 (Programming)

### N3000 Programming

#### Sample CR for N3000 programming (N3000)

```yaml
apiVersion: fpga.intel.com/v1
kind: N3000Cluster
metadata:
  name: n3000
  namespace: vran-acceleration-operators
spec:
  nodes:
    - nodeName: node1.png.intel.com
      fpga:
        - userImageURL: http://10.10.10.122:8000/pkg/sr_vista_rot_2x2x25-v1.3.16.bin
          PCIAddr: 0000:1b:00.0
          checksum: "0b0a87b974d35ea16023ceb57f7d5d9c"
      fortville:
        firmwareURL: http://10.10.10.122:8000/7.00/700Series_NVMDowngradePackage_v7_00_Linux.tar.gz
        checksum: "0b0a87b974d35ea16023ceb57f7d5d9d"
        MACs:
          - MAC: 64:4c:36:12:61:d9
    - nodeName: node2.png.intel.com
      fpga:
        - userImageURL: http://10.10.10.122:8000/pkg/sr_vista_rot_2x2x25-v1.3.16.bin
          PCIAddr: 0000:1b:00.0
          checksum: "0b0a87b974d35ea16023ceb57f7d5d9c"
    - nodeName: node3.png.intel.com
      fortville:
        firmwareURL: http://10.10.10.122:8000/7.00/700Series_NVMDowngradePackage_v7_00_Linux.tar.gz
        checksum: "0b0a87b974d35ea16023ceb57f7d5d9d"
        MACs:
          - MAC: 64:4c:36:12:61:d3
```

#### Sample Status for N3000 programming (N3000)

```yaml
status:                                                                                                 
  conditions:                                                                                           
  - lastTransitionTime: "2020-12-15T18:18:53Z"                                                          
    message: Flashed successfully                                                                       
    observedGeneration: 2                                                                               
    reason: Succeeded                                                                                   
    status: "True"                                                                                      
    type: Flashed                                                                                       
  fortville:                                                                                            
  - N3000PCI: 0000:1b:00.0                                                                              
    NICs:                                                                                               
    - MAC: 64:4c:36:11:1b:a8                                                                            
      NVMVersion: 7.00 0x800052b0 0.0.0                                                                 
      PCIAddr: 0000:1a:00.0                                                                             
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
    - MAC: 64:4c:36:11:1b:a9           
      NVMVersion: 7.00 0x800052b0 0.0.0                                                                 
      PCIAddr: 0000:1a:00.1                                                                             
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
    - MAC: 64:4c:36:11:1b:ac           
      NVMVersion: 7.00 0x800052b0 0.0.0                                                                 
      PCIAddr: 0000:1c:00.0                                                                             
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
    - MAC: 64:4c:36:11:1b:ad           
      NVMVersion: 7.00 0x800052b0 0.0.0                                                                 
      PCIAddr: 0000:1c:00.1                                                                             
      name: Ethernet Controller XXV710 Intel(R) FPGA Programmable Acceleration Card N3000 for Networking
  fpga:                            
  - PCIAddr: 0000:1b:00.0          
    bitstreamId: "0x2315842A010601"
    bitstreamVersion: 0.2.3
    deviceId: "0x0b30
```

#### Sample Daemon log for N3000 programming (N3000)

```shell
{"level":"info","ts":1608054338.8866854,"logger":"daemon.drainhelper.cordonAndDrain()","msg":"node drained"}
{"level":"info","ts":1608054338.8867319,"logger":"daemon.drainhelper.Run()","msg":"worker function - start"}
{"level":"info","ts":1608054338.9003832,"logger":"daemon.fpgaManager.ProgramFPGAs","msg":"Start program","PCIAddr":"0000:1b:00.0"}
{"level":"info","ts":1608054338.9004142,"logger":"daemon.fpgaManager.ProgramFPGA","msg":"Starting","pci":"0000:1b:00.0"}
{"level":"info","ts":1608056309.9367146,"logger":"daemon.fpgaManager.ProgramFPGA","msg":"Program FPGA completed, start new power cycle N3000 ...","pci":"0000:1b:00.0"}
{"level":"info","ts":1608056333.3528838,"logger":"daemon.drainhelper.Run()","msg":"worker function - end","performUncordon":true}
```
