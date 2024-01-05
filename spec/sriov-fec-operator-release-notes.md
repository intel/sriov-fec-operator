```text
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2020-2023 Intel Corporation
```
<!-- omit in toc -->
# Release Notes
This document provides high-level system features, issues, and limitations information for SRIOV-FEC Operator for Wireless FEC Accelerators.
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

| Version | Release Date   | OCP Version(s) compatibility | Verified on OCP               |
|---------|----------------|------------------------------|-------------------------------|
| 1.0.0   | January 2021   | 4.6                          | 4.6.4                         |
| 1.1.0   | March 2021     | 4.6                          | 4.6.16                        |
| 1.2.0   | June 2021      | 4.7                          | 4.7.8                         |
| 1.2.1   | June 2021      | 4.7                          | 4.7.8                         |
| 1.3.0   | August 2021    | 4.8                          | 4.8.2                         |
| 2.0.0   | September 2021 | 4.8                          | 4.8.5                         |
| 2.0.1   | October 2021   | 4.8                          | 4.8.13                        |
| 2.0.2   | November 2021  | 4.8                          | 4.8.12                        |
| 2.1.0   | November 2021  | 4.9                          | 4.9.7                         |
| 2.1.1   | January 2022   | 4.9                          | 4.9.7                         |
| 2.2.0   | March 2022     | 4.8, 4.9, 4.10               | 4.8.35, 4.9.23, 4.10.5        | 
| 2.2.1   | April 2022     | 4.8, 4.9, 4.10               | 4.8.35, 4.9.23, 4.10.5        |
| 2.3.0   | May 2022       | 4.8, 4.9, 4.10               | 4.8.42, 4.9.36, 4.10.17       |
| 2.3.1   | July 2022      | 4.8, 4.9, 4.10               | 4.8.46, 4.9.41, 4.10.21       |
| 2.4.0   | September 2022 | 4.9, 4.10, 4.11              | 4.9.41, 4.10.21, 4.11.2       |
| 2.5.0   | September 2022 | 4.9, 4.10, 4.11              | 4.9.48, 4.10.34, 4.11.5       |
| 2.6.0   | December 2022  | 4.10, 4.11, 4.12             | 4.10.43, 4.11.18, 4.12-rc2    |
| 2.6.1   | January 2023   | 4.10, 4.11, 4.12             | 4.10.43, 4.11.18, 4.12.0-rc.4 |
| 2.7.0   | May 2023       | 4.10, 4.11, 4.12, 4.13       | 4.11.43, 4.12.18, 4.13.0.rc18 |
| 2.7.1   | July 2023      | 4.10, 4.11, 4.12, 4.13       | 4.11.43, 4.12.18, 4.13.7      |
| 2.7.2   | October 2023   | 4.10, 4.11, 4.12, 4.13       | 4.10.67, 4.11.50, 4.12.37, 4.13.15 |
| 2.8.0   | Dec 2023       | 4.11, 4.12, 4.13, 4.14       | 4.11.54, 4.12.45, 4.13.27, 4.14.7  |

# Features for Release
***v2.8.0***
- Added support to configure and manage VRB2 Accelerator device.
- Updated pf-bb-conf version to 23.11
- Ability to update srs_fft_windows_coefficient.bin file on worker node for VRB1 and VRB2.

***v2.7.2***
- Bug fixes

***v2.7.1***
- pf-bb-config updated (22.11 -> 23.03)
- Bug fixes

***v2.7.0***
- Support for OCP 4.13.x
- Bug fixes

***v2.6.1***
- pf-bb-config updated (22.07 -> 22.11)
- Added support for pf-bb-config telemetry
- Added support for ACC200 cards (SPR-EE)
- Operator now propagates `Tolerations` from Subscription to managed Daemonsets

***v2.6.0***
- Support for OCP4.12.x

***v2.5.0***
- pf-bb-config updated (22.03 -> 22.07)
- Added support for Ubuntu 22.04
- Improved documentation for VFIO token

***v2.4.0***
- Support for OCP4.11.x

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
***v2.8.0***
- UBI base docker image version updated to 9.3-6.
- sriov-network-device-plugin version updated to 4.14
- Telemetry request/response flow between daemon and pf-bb-conf is updated.
- xnet and go-logr package version updated.
- Restricted hostPath mount to read-only for /lib/modules.
- Restricted hostPath mount specific to device-plugin for sriov-network-device-plugin.
- Resource cleanup in proper order during the CR deletion.
- Restrict the vfio-pci driver parameter disable_idle_d3 set to ACC100 only.

***v2.7.2***
- None

***v2.7.1***
- FEC resource names can be controlled through manager environment variables
- igb-uio driver is added to list of support drivers for VF interface
- Base images are updated to ubi9.2

***v2.7.0***
- VFIO token handling enhancements
- sriov-network-device-plugin version update to v4.14

***v2.6.1***
- Improved timeouts for LeaderElection functionality
- Manager deployment always starts with 1 replica and scales to 2 for multi-node clusters
- Base images are updated to ubi9.1 instead of ubi8.6
- Reduced RBAC permissions required by operator
- Daemon now has readiness and liveliness probes
- Removed mentions of Smart Edge Open from documentation. Operator is now standalone project.

***v2.4.0***
- SriovFecClusterConfig.spec.physicalFunction.bbDevConfig field is now marked as 'required'

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
***2.8.0***
- Fix for leader election resource cleanup after removal of Operator.
- Fix for logs collecting script.
- Fix for validation of maxNumQGroup parameter in CR.

***2.7.2***
- Fix for failure in enabling VFs when kernel is overloaded

***2.7.1***
- Fix for supporting multiple FEC devices on same node
- Fix for checking secure boot enabled mode

***2.7.0***
- Enhanced error handling while processing telemetry data to fix Daemon crash addressing issue: https://github.com/smart-edge-open/sriov-fec-operator/issues/48
- Leader lease renewal frequency configuration in case of Single Node Cluster addressing issue: https://github.com/smart-edge-open/sriov-fec-operator/issues/36

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
- SRIOV-FEC Operator for Wireless FEC Accelerators
- Documentation

# Supported Operating Systems
***v2.8.0***
- OpenShift: 4.14.7
- OS: Red Hat Enterprise Linux CoreOS 414.92.202312132152-0
- Kubernetes: v1.27.8+4fab27b
- RT Kernel: 5.14.0-284.45.1.rt14.330.el9_2.x86_64

***v2.7.2***
- OpenShift: 4.13.0
- OS: Red Hat Enterprise Linux CoreOS 413.92.202305191644-0
- Kubernetes: v1.26.3+b404935
- RT Kernel: 5.14.0-284.13.1.el9_2.x86_64

***v2.7.1***
- OpenShift: 4.13.0
- OS: Red Hat Enterprise Linux CoreOS 413.92.202305191644-0
- Kubernetes: v1.26.3+b404935
- RT Kernel: 5.14.0-284.13.1.el9_2.x86_64

***v2.7.1***
- Kubernetes 1.26.2
- OS: Ubuntu 22.04 LTS (Jammy Jellyfish)
- Kernel: 5.15.0-72-generic, 5.15.0-1030-realtime

***v2.7.0***
- OpenShift: 4.13.0
- OS: Red Hat Enterprise Linux CoreOS 413.92.202305191644-0
- Kubernetes: v1.26.3+b404935
- RT Kernel: 5.14.0-284.13.1.el9_2.x86_64

***v2.7.0***
- Kubernetes 1.26.2
- OS: Ubuntu 22.04 LTS (Jammy Jellyfish)
- Kernel: 5.15.0-72-generic, 5.15.0-1030-realtime

***v2.6.1***
- OpenShift: 4.12.0-rc.4
- OS: Red Hat Enterprise Linux CoreOS 412.86.202212081411-0
- Kubernetes: v1.25.4+86bd4ff
- RT Kernel: 4.18.0-372.36.1.rt7.193.el8_6.x86_64

***v2.6.0***
- OpenShift: 4.12.0-rc.2
- OS: Red Hat Enterprise Linux CoreOS 412.86.202211142021-0
- Kubernetes: v1.25.2+cd98eda
- RT Kernel: 4.18.0-425.3.1.rt7.213.el8.x86_64

***v2.5.0***
- OpenShift: 4.11.5
- OS: Red Hat Enterprise Linux CoreOS 411.86.202209140028-0 (Ootpa)
- Kubernetes: v1.24.0+3882f8f
- RT Kernel: 4.18.0-372.26.1.rt7.183.el8_6.x86_64

***v2.5.0***
- Kubernetes 1.23.5+c285e78
- OS: Ubuntu 22.04 LTS (Jammy Jellyfish)
- Kernel: 5.15.0-43-generic

***v2.4.0***
- OpenShift: 4.11.2
- OS: Red Hat Enterprise Linux CoreOS 411.86.202208191320-0 (Ootpa)
- Kubernetes: v1.24.0+b62823b
- RT Kernel: 4.18.0-372.19.1.rt7.176.el8_6.x86_64

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
- Golang: 1.21.5
- pf-bb-config-app: v23.11
