```text
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2020-2021 Intel Corporation
```
<!-- omit in toc -->
# Release Notes 
This document provides high-level system features, issues, and limitations information for OpenNESS Operator for Intel® FPGA PAC N3000 and OpenNESS SR-IOV Operator for Wireless FEC Accelerators.
- [Release history](#release-history)
- [Features for Release](#features-for-release)
- [Changes to Existing Features](#changes-to-existing-features)
- [Fixed Issues](#fixed-issues)
- [Known Issues and Limitations](#known-issues-and-limitations)
- [Release Content](#release-content)
- [Hardware and Software Compatibility](#hardware-and-software-compatibility)
- [Supported Operating Systems](#supported-operating-systems)
- [Package Versions](#package-versions)

# Release history 
***v1.1.0*** 
- OpenNESS Operator for Intel® FPGA PAC N3000 
- OpenNESS SR-IOV Operator for Wireless FEC Accelerators
  
***v1.0.0***
- OpenNESS Operator for Intel® FPGA PAC N3000
- OpenNESS SRIOV-FEC Operator for Intel® FPGA PAC N3000
   
# Features for Release 
***v1.1.0***
- OpenNESS SR-IOV Operator for Wireless FEC Accelerators
  - Added support for Intel® vRAN Dedicated Accelerator ACC100
  - Independent accelerator discovery mechanism now enables standalone usage

***v1.0.0***
- OpenNESS Operator for Intel® FPGA PAC N3000  
  - N3000 operator handles the management of the FPGA configuration
  - Load the necessary drivers, allows the user to program the Intel® FPGA PAC N3000 user image and to update the firmware of the Intel® XL710 NICs
  - Download the FPGA user image and the XL710 firmware from a location specified in the CR
- OpenNESS SRIOV-FEC Operator for Intel® FPGA PAC N3000
  - The SRIOV FEC operator handles the management of the FEC devices used to accelerate the FEC process in vRAN L1 applications
  - Create desired Virtual Functions for the FEC device, bind them to appropriate drivers and configure the VF's queues for desired functionality in 4G or 5G deployment
  - Deploys an instance of K8s SRIOV device plugin which manages the FEC VFs as an OpenShift cluster resource and configures this device plugin to detect the resources
  - Prometheus fpgainfo exporter 
    - Deploys an instance of Prometheus exporter which collects metrics from the Intel® FPGA PAC N3000 card

# Changes to Existing Features
***v1.1.0***
- OpenNESS Operator for Intel® FPGA PAC N3000
  - n3000node- prefix was removed from N3000 resources
  - Flashing process logging improvements
- OpenNESS SR-IOV Operator for Wireless FEC Accelerators
  - Added supported vendor: 1172 - Altera Corporation
  - pf-bb-config updated to 21.3
- Common
  - Operator SDK updated to 1.4.2
  - `stable` channel is now used for subscriptions
  - Image build refactored and moved to Makefile
  - Generated bundle files were removed from repository
  - Common packages and labeler extracted from N3000/
  - Index image build target added to Makefile
  - Both, n3000 and sriov-fec daemonsets now use `readOnlyRootFilesystem: true`
  - Supported accelerators list moved to `supported-accelerators` configmap
  - `n3000-discovery` was renamed to `accelerator-discovery`
  - Any namespace can be now used for operators deployment
  
***v1.0.0***
- There are no unsupported or discontinued features relevant to this release.

# Fixed Issues
***v1.1.0***
- OpenNESS Operator for Intel® FPGA PAC N3000
  - Daemon in started only after confirmed driver initialization
  - Removed `hostPort:` from `fpgainfo-exporter` pod definition
- OpenNESS SR-IOV Operator for Wireless FEC Accelerators
  - Fixed status conditions to match convention introduced in N3000 operator
- Common
  - Fixed discovery for devices with LTE bitstream
  - Fixed field optionality policies in CRDs
  - Fixed DNS policy for n3000 daemonset

***v1.0.0***
- n/a - this is the first release.

# Known Issues and Limitations
- After a successful user image of Fortville update, when power cycling the N3000 with the RSU command, a failure to reboot properly has been observed occasionally. This results in failed SPI transactions and a loss of communication with the BMC. To resolve, reboot the server.

# Release Content
- OpenNESS Operator for Intel® FPGA PAC N3000 
- OpenNESS SR-IOV Operator for Wireless FEC Accelerators
- Prometheus fpgainfo exporter
- Documentation

# Hardware and Software Compatibility
The OpenNESS Operator for Intel® FPGA PAC N3000 has the following requirements:
- [Intel® FPGA PAC N3000 card](https://www.intel.com/content/www/us/en/programmable/products/boards_and_kits/dev-kits/altera/intel-fpga-pac-n3000/overview.html)
- vRAN RTL image for the Intel® FPGA PAC N3000 card
- NVM utility
- OpenShift
- RT Kernel (the OPAE Docker images are built for specific kernel version)

# Supported Operating Systems
***v1.1.0*** was tested using the following:
- OpenShift: 4.6.16
- OS: Red Hat Enterprise Linux CoreOS 46.82.202101301821-0
- Kubernetes: v1.19.0+e49167a
- RT Kernel: 4.18.0-193.41.1.rt13.91.el8_2.x86_64
- OPAE: n3000-1.3.8-2-rte-el8
- RTL Image: 20ww27.5-2x2x25G-5GLDPC-v1.6.1-3.0.0_unsigned.bin
- NVM Package: v7.30

***v1.0.0*** was tested using the following:
- OpenShift: 4.6.4
- OS: Red Hat Enterprise Linux CoreOS 46.82.202011061621-0
- Kubernetes: v1.19.0+9f84db3
- RT Kernel: 4.18.0-193.28.1.rt13.77.el8_2.x86_64
- OPAE: n3000-1.3.8-2-rte-el8
- RTL Image: 20ww27.5-2x2x25G-5GLDPC-v1.6.1-3.0.0_unsigned.bin
- NVM Package: v7.30

# Package Versions 
Package:
- Prometheus: 1.7.1
- Golang: 1.15
- Kubernetes: 1.19.0
- DPDK: v20.11
- pf-bb-config-app: v21.3
