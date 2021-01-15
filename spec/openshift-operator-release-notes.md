```text
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2020 Intel Corporation
```
<!-- omit in toc -->
# Release Notes 
This document provides high-level system features, issues, and limitations information for OpenNESS Operator for Intel® FPGA PAC N3000. 
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
1. OpenNESS Operator for Intel® FPGA PAC N3000 v1.0.0  

   
# Features for Release 
1. **OpenNESS Operator for Intel® FPGA PAC N3000 v1.0.0**
   - N3000 Operator  
      - N3000 operator handles the management of the FPGA configuration
      - Load the necessary drivers, allows the user to program the Intel® FPGA PAC N3000 user image and to update the firmware of the Intel® XL710 NICs
	  - Download the FPGA user image and the XL710 firmware from a location specified in the CR
   - SRIOV FEC Operator 
      - The SRIOV FEC operator handles the management of the FEC devices used to accelerate the FEC process in vRAN L1 applications
      - Create desired Virtual Functions for the FEC device, bind them to appropriate drivers and configure the VF's queues for desired functionality in 4G or 5G deployment
	  - Deploys an instance of K8s SRIOV device plugin which manages the FEC VFs as an OpenShift cluster resource and configures this device plugin to detect the resources
   - Prometheus fpgainfo exporter 
   	  - Deploys an instance of Prometheus exporter which collects metrics from the Intel® FPGA PAC N3000 card

# Changes to Existing Features
- **OpenNESS Operator for Intel® FPGA PAC N3000 v1.0.0**
  - There are no unsupported or discontinued features relevant to this release.

# Fixed Issues
- **OpenNESS Operator for Intel® FPGA PAC N3000 v1.0.0**
  - n/a - this is the first release.

# Known Issues and Limitations
- **OpenNESS Operator for Intel® FPGA PAC N3000 v1.0.0**
  - There are no known issues relevant to this release.

# Release Content
- **OpenNESS Operator for Intel® FPGA PAC N3000 v1.0.0**:
  - N3000 Operator
  - SRIOV FEC Operator
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
**OpenNESS Operator for Intel® FPGA PAC N3000 v1.0.0** was tested using the following:
- OpenShift: 4.6.4
- Kubernetes: v1.19.0+9f84db3
- RT Kernel: 4.18.0-193.14.3.el8_2.x86_64
- RTL Image: 20ww27.5-2x2x25G-5GLDPC-v1.6.1-3.0.0_unsigned.bin
- NVM Package: v7.30

# Package Versions 
Package:
- Prometheus: 1.7.1
- Golang: 1.14.0
- Kubernetes: 1.19.0
- DPDK: v20.11
