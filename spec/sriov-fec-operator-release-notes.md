```text
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2020-2022 Intel Corporation
```
<!-- omit in toc -->
# Release Notes
This document provides high-level system features, issues, and limitations information for SEO SR-IOV Operator for Wireless FEC Accelerators.
- [Release history](#release-history)
- [SRIOV-FEC Operator](#sriov-fec-operator)
- [Features for Release](#features-for-release)
- [Changes to Existing Features](#changes-to-existing-features)
- [Fixed Issues](#fixed-issues)
- [Release Content](#release-content)
- [Supported Operating Systems](#supported-operating-systems)
- [Package Versions](#package-versions)


> **_Single Node OpenShift (SNO)_**
>
>Daemon part (running on each featured worker node) of operator drains a node (moves its workloads to another node) before applying requested configuration.  
>Node draining doesn't work on SNO deployment. Because of that, operator's API exposes `SriovFecClusterConfig.spec.drainSkip` parameter which stops daemon doing workload migration.
>In theory it is all what is needed to find operator usable on SNO, however, operator's validation cycle is executed _ONLY_ on multi-worker-node clusters.

# Release history

### SRIOV-FEC Operator

| Version | Release Date   | OCP Version(s) compatibility | Verified on OCP         |
|---------|----------------|------------------------------|-------------------------|
| 1.0.0   | January 2021   | 4.6                          | 4.6.4                   |
| 1.1.0   | March 2021     | 4.6                          | 4.6.16                  |
| 1.2.0   | June 2021      | 4.7                          | 4.7.8                   |
| 1.2.1   | June 2021      | 4.7                          | 4.7.8                   |
| 1.3.0   | August 2021    | 4.8                          | 4.8.2                   |
| 2.0.0   | September 2021 | 4.8                          | 4.8.5                   |
| 2.0.1   | October 2021   | 4.8                          | 4.8.13                  |
| 2.0.2   | November 2021  | 4.8                          | 4.8.12                  |
| 2.1.0   | November 2021  | 4.9                          | 4.9.7                   |
| 2.1.1   | January 2022   | 4.9                          | 4.9.7                   |
| 2.2.0   | March 2022     | 4.8, 4.9, 4.10               | 4.8.35, 4.9.23, 4.10.5  | 
| 2.2.1   | April 2022     | 4.8, 4.9, 4.10               | 4.8.35, 4.9.23, 4.10.5  |
| 2.3.0   | May 2022       | 4.8, 4.9, 4.10               | 4.8.42, 4.9.36, 4.10.17 |
| 2.3.1   | July 2022      | 4.8, 4.9, 4.10               | 4.8.46, 4.9.41, 4.10.21 |

# Features for Release

***v2.3.1***
- Bugfixes

***v2.3.0***
- pf-bb-config updated (21.11 -> 22.03)
- Initial support of vfio-pci driver for ACC100 

***v2.2.1***
- Completed validation for MacLaren Summit card

***v2.2.0***
- Support for OCP4.10.x

***v2.1.1***
- Added support for igb_uio module as a PF driver
- Added support for Docker tool as an image building tool

***v2.1.0***
- Support for OCP4.9.x
- Bugfixes

***v2.0.2***
- Bugfixes

***v2.0.1***
- Bugfixes

***v2.0.0***
- Added new version (v2) of API with selectors
- Added resources cleanup on SriovFecClusterConfig deletion
- SriovFecController no longer overwrites ConfigMaps with `immutable` key
- Added support for deployment on K8S

***v1.3.0***
- SEO SR-IOV Operator for Wireless FEC Accelerators OCP4.8.2 support
  - validated on ACC100 only

***v1.2.0***
- SEO SR-IOV Operator for Wireless FEC Accelerators OCP4.7.8 support
  - validated on ACC100 only

***v1.1.0***
- SEO SR-IOV Operator for Wireless FEC Accelerators
  - Added support for Intel® vRAN Dedicated Accelerator ACC100
  - Independent accelerator discovery mechanism now enables standalone usage

***v1.0.0***
- SEO SRIOV-FEC Operator for Intel® FPGA PAC N3000
  - The SRIOV FEC operator handles the management of the FEC devices used to accelerate the FEC process in vRAN L1 applications
  - Create desired Virtual Functions for the FEC device, bind them to appropriate drivers and configure the VF's queues for desired functionality in 4G or 5G deployment
  - Deploys an instance of K8s SRIOV device plugin which manages the FEC VFs as an OpenShift cluster resource and configures this device plugin to detect the resources

# Changes to Existing Features

***v2.3.0***
- Flattened sriov-fec operator structure by removing the `sriov-fec` directory
- Previous `labeler` directory acts now as internal package of sriov-fec operator
- Operator no longer adds missing kernel parameters `intel_iommu=on` and `iommu=pt`. User has to configure them [manually](https://wiki.ubuntu.com/Kernel/KernelBootParameters#Permanently_Add_a_Kernel_Boot_Parameter).

***v2.2.0***
- this release targets multiple OCP versions (4.8, 4.9, 4.10). Validation cycle has covered following upgrades:
  - 4.8.x (sriov-fec 2.0.2) -> 4.10.x (sriov-fec 2.2.0)
  - 4.9.x (sriov-fec 2.1.1) -> 4.10.x (sriov-fec 2.2.0)
- Updated pf-bb-config from 21.6 to 21.11
- Updated SriovDevicePlugin from 4.9 to 4.10
- SriovFecNodeConfig changes its state to "Succeeded" only after successful restart of sriov-device-plugin
- Renamed OpenNESS in documentation to Smart Edge Open (SEO)
- `physicalFunction` in `SriovFecClusterConfig` CR is now required
- Operator automatically detects type of cluster(Openshift/Kubernetes) and uses corresponding dependencies
- `SriovFecClusterConfig.nodes` field is not supported anymore, SFCC should rely on `nodeSelector` and `acceleratorSelectors` fields
- Renamed repository from openshift-operator to sriov-fec-operator
- Development of N3000 Operator has been suspended and its source code is not part of main branch
- previous `common` directory acts now as internal package of sriov-fec operator

***v2.0.2***
- Added webhook that converts existing SriovFecClusterConfigs with `nodes` field to SriovFecClusterConfig with `nodeSelector` and `acceleratorSelectors`
- Added webhook that prohibits creation of  SriovFecClusterConfig with `nodes` field.
- Daemon's reconciliation process trigger has been adjusted to cover multi-reboot scenarios

***v2.0.1***
- Daemon reconcile loop has been redesigned

***v2.0.0***
- Improved existing validation rules and added new rules
- Removed old API (v1)
- Updated pf-bb-config from 21.3 to 21.6 and OperatorSDK from 1.4.2 to 1.9.0

***v1.1.0***
- SEO SR-IOV Operator for Wireless FEC Accelerators
  - Added supported vendor: 1172 - Altera Corporation
  - pf-bb-config updated to 21.3
- Common
  - Operator SDK updated to 1.4.2
  - `stable` channel is now used for subscriptions
  - Image build refactored and moved to Makefile
  - Generated bundle files were removed from repository
  - Common packages and labeler extracted from N3000/
  - Index image build target added to Makefile
  - sriov-fec daemonsets now use `readOnlyRootFilesystem: true`
  - Supported accelerators list moved to `supported-accelerators` configmap
  - `n3000-discovery` was renamed to `accelerator-discovery`
  - Any namespace can be now used for operators deployment

***v1.0.0***
- There are no unsupported or discontinued features relevant to this release.

# Fixed Issues

***2.3.1***
- fix for pf_bb_config throwing "MMIO is not accessible causing UR error over PCIe"

***2.2.1***
- Adjusting CSV by adding relatedImages tag - addressing https://github.com/smart-edge-open/sriov-fec-operator/issues/19

***2.1.0***
- SriovFecClusterConfig.spec.drainSkip was not rewritten into SriovFecNodeConfig.spec.drainSkip so SNO worker
  was trying to drain its workloads

***2.0.2***
- SriovFecNodeConfig stucks in InProgress state(issue observed in case of multiple reboots)

***v1.2.1***
- [4.7.9 sriov-fec-v1.1.0 install does not succeed initially #270](https://github.com/smart-edge-open/sriov-fec-operator/issues/270)

***v1.1.0***
- SEO SR-IOV Operator for Wireless FEC Accelerators
  - Fixed status conditions to match convention introduced in N3000 operator
- Common
  - Fixed discovery for devices with LTE bitstream
  - Fixed field optionality policies in CRDs

***v1.0.0***
- n/a - this is the first release.

# Release Content
- SEO SR-IOV Operator for Wireless FEC Accelerators
- Documentation

# Supported Operating Systems

***v2.3.1***
- OpenShift: 4.10.21
- OS: Red Hat Enterprise Linux CoreOS 410.84.202206010432-0 (Ootpa)
- Kubernetes: v1.23.5+3afdacb
- RT Kernel: 4.18.0-305.49.1.rt7.121.el8_4.x86_64

***v2.3.0*** was tested using the following:
- OpenShift: 4.10.17
- OS: Red Hat Enterprise Linux CoreOS 410.84.202206010432-0 (Ootpa)
- Kubernetes: v1.23.5+3afdacb
- RT Kernel: 4.18.0-305.49.1.rt7.121.el8_4.x86_64

***v2.3.0*** was tested using the following:
- OpenShift: 4.9.36
- OS: Red Hat Enterprise Linux CoreOS 49.84.202205241705-0 (Ootpa)
- Kubernetes: v1.22.8+f34b40c
- RT Kernel: 4.18.0-305.45.1.rt7.117.el8_4.x86_64

***v2.1.1*** was tested using the following:
- OpenShift: 4.9.7
- OS: Red Hat Enterprise Linux CoreOS 49.84.202111022104-0 (Ootpa)
- Kubernetes: v1.22.2+5e38c72
- RT Kernel: 4.18.0-305.25.1.rt7.97.el8_4.x86_64

***v2.1.1*** was tested using the following:
- CentOS 7.9
- Kubernetes: v1.22.2
- RT Kernel: 3.10.0-1160.11.1.rt56.1145.el7.x86_64

***v2.1.0*** was tested using the following:
- OpenShift: 4.9.7
- OS: Red Hat Enterprise Linux CoreOS 49.84.202111022104-0 (Ootpa)
- Kubernetes: v1.22.2+5e38c72
- RT Kernel: 4.18.0-305.25.1.rt7.97.el8_4.x86_64

***v2.0.2*** was tested using the following:
- OpenShift: 4.8.13
- OS: Red Hat Enterprise Linux CoreOS 48.84.202109210859-0 (Ootpa)
- Kubernetes: v1.21.1+a620f50
- RT Kernel: 4.18.0-305.19.1.rt7.91.el8_4.x86_64

***v2.0.1*** was tested using the following:
- OpenShift: 4.8.13
- OS: Red Hat Enterprise Linux CoreOS 48.84.202109210859-0 (Ootpa)
- Kubernetes: v1.21.1+a620f50
- RT Kernel: 4.18.0-305.19.1.rt7.91.el8_4.x86_64

***v2.0.0*** was tested using the following:
- OpenShift: 4.8.5
- OS: Red Hat Enterprise Linux CoreOS 48.84.202108062347-0
- Kubernetes: v1.21.1+9807387
- RT Kernel: 4.18.0-305.10.2.rt7.83.el8_4.x86_64

***v1.3.0*** was tested using the following:
- OpenShift: 4.8.2
- OS: Red Hat Enterprise Linux CoreOS 48.84.202107202156-0
- Kubernetes: v1.21.1+051ac4f
- RT Kernel: 4.18.0-305.10.2.rt7.83.el8_4.x86_64

***v1.2.1*** was tested using the following:
- OpenShift: 4.7.8
- OS: Red Hat Enterprise Linux CoreOS 47.83.202104161442-0
- Kubernetes: v1.20.0+7d0a2b2
- RT Kernel: 4.18.0-240.22.1.rt7.77.el8_3.x86_64

***v1.2.0*** was tested using the following:
- OpenShift: 4.7.8
- OS: Red Hat Enterprise Linux CoreOS 47.83.202104161442-0
- Kubernetes: v1.20.0+7d0a2b2
- RT Kernel: 4.18.0-240.22.1.rt7.77.el8_3.x86_64

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
- Golang: 1.18
- DPDK: v20.11
- pf-bb-config-app: v22.03
