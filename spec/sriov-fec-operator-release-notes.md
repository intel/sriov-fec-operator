```text
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2020-2026 Intel Corporation
```

# Release Notes
This document provides high-level system features, issues, and limitations information for SRIOV-FEC Operator for Wireless FEC Accelerators.

<!-- TOC depthfrom:1 depthto:2 -->

- [Release Notes](#release-notes)
- [Release history](#release-history)
    - [SRIOV-FEC Operator for OpenShift Operating System](#sriov-fec-operator-for-openshift-operating-system)
    - [SRIOV-FEC Operator for Ubuntu Operating System](#sriov-fec-operator-for-ubuntu-operating-system)
- [Features for release](#features-for-release)
    - [v2.12.0](#v2120)
    - [v2.11.1](#v2111)
    - [v2.11.0](#v2110)
    - [v2.10.0](#v2100)
    - [v2.9.0](#v290)
    - [v2.8.0](#v280)
    - [v2.7.2](#v272)
    - [v2.7.1](#v271)
    - [v2.7.0](#v270)
    - [v2.6.1](#v261)
    - [v2.6.0](#v260)
    - [v2.5.0](#v250)
    - [v2.4.0](#v240)
    - [v2.3.1](#v231)
    - [v2.3.0](#v230)
    - [v2.2.1](#v221)
    - [v2.2.0](#v220)
    - [v2.1.0](#v210)
    - [v2.0.2](#v202)
    - [v2.0.1](#v201)
    - [v2.0.0](#v200)
    - [v1.3.0](#v130)
    - [v1.2.1](#v121)
    - [v1.2.0](#v120)
    - [v1.1.0](#v110)
    - [v1.0.0](#v100)

<!-- /TOC -->

# Release history

## SRIOV-FEC Operator for OpenShift Operating System
| Version            | Release Date   | OCP Version(s) compatibility | Verified on OCP                              |
|--------------------|----------------|------------------------------|----------------------------------------------|
| [v2.12.0](#v2120)  | Sept 2025      | 4.12 and higher versions     | 4.12 and higher (latest stable versions)     |
| [v2.11.1](#v2111)  | Apr 2025       | 4.12 and higher versions     | 4.12 and higher (latest stable versions)     |
| [v2.11.0](#v2110)  | Feb 2025       | 4.12 and higher versions     | 4.12 and higher (latest stable versions)     |
| [v2.10.0](#v2100)  | Dec 2024       | 4.12 and higher versions     | 4.12 and higher (latest stable versions)     |
| [v2.9.0](#v290)    | May 2024       | 4.11, 4.12, 4.13, 4.14, 4.15 | 4.11.54, 4.12.57, 4.13.27, 4.14.25, 4.15.13  |
| [v2.8.0](#v280)    | Dec 2023       | 4.11, 4.12, 4.13, 4.14       | 4.11.54, 4.12.45, 4.13.27, 4.14.7            |
| [v2.7.2](#v272)    | October 2023   | 4.10, 4.11, 4.12, 4.13       | 4.10.67, 4.11.50, 4.12.37, 4.13.15           |
| [v2.7.1](#v271)    | July 2023      | 4.10, 4.11, 4.12, 4.13       | 4.11.43, 4.12.18, 4.13.7                     |
| [v2.7.0](#v270)    | May 2023       | 4.10, 4.11, 4.12, 4.13       | 4.11.43, 4.12.18, 4.13.0.rc18                |
| [v2.6.1](#v261)    | January 2023   | 4.10, 4.11, 4.12             | 4.10.43, 4.11.18, 4.12.0-rc.4                |
| [v2.6.0](#v260)    | December 2022  | 4.10, 4.11, 4.12             | 4.10.43, 4.11.18, 4.12-rc2                   |
| [v2.5.0](#v250)    | September 2022 | 4.9, 4.10, 4.11              | 4.9.48, 4.10.34, 4.11.5                      |
| [v2.4.0](#v240)    | September 2022 | 4.9, 4.10, 4.11              | 4.9.41, 4.10.21, 4.11.2                      |
| [v2.3.1](#v231)    | July 2022      | 4.8, 4.9, 4.10               | 4.8.46, 4.9.41, 4.10.21                      |
| [v2.3.0](#v230)    | May 2022       | 4.8, 4.9, 4.10               | 4.8.42, 4.9.36, 4.10.17                      |
| [v2.2.1](#v221)    | April 2022     | 4.8, 4.9, 4.10               | 4.8.35, 4.9.23, 4.10.5                       |
| [v2.2.0](#v220)    | March 2022     | 4.8, 4.9, 4.10               | 4.8.35, 4.9.23, 4.10.5                       |
| [v2.1.0](#v210)    | January 2022   | 4.9                          | 4.9.7                                        |
| [v2.0.2](#v202)    | November 2021  | 4.8                          | 4.8.12                                       |
| [v2.0.1](#v201)    | October 2021   | 4.8                          | 4.8.13                                       |
| [v2.0.0](#v200)    | September 2021 | 4.8                          | 4.8.5                                        |
| [v1.3.0](#v130)    | August 2021    | 4.8                          | 4.8.2                                        |
| [v1.2.1](#v121)    | June 2021      | 4.7                          | 4.7.8                                        |
| [v1.2.0](#v120)    | June 2021      | 4.7                          | 4.7.8                                        |
| [v1.1.0](#v110)    | March 2021     | 4.6                          | 4.6.16                                       |
| [v1.0.0](#v100)    | January 2021   | 4.6                          | 4.6.4                                        |

## SRIOV-FEC Operator for Ubuntu Operating System
| Version            | Release Date   | Ubuntu OS Version    | K8s Version          |
|--------------------|----------------|----------------------|----------------------|
| [v2.12.1](#v2121)  | June 2026      | v24.04 LTS           | v1.28                |
| [v2.12.0](#v2120)  | Sept 2025      | v24.04 LTS           | v1.26, v1.28         |
| [v2.11.1](#v2111)  | Apr 2025       | v24.04 LTS           | v1.26                |
| [v2.11.0](#v2110)  | Feb 2025       | v24.04 LTS           | v1.26                |
| [v2.10.0](#v2100)  | Dec 2024       | v24.04 LTS           | v1.26                |
| [v2.9.0](#v290)    | May 2024       | v22.04 LTS           | v1.26                |
| [v2.8.0](#v280)    | Dec 2023       | v22.04 LTS           | v1.26                |
| [v2.7.2](#v272)    | October 2023   | v22.04 LTS           | v1.26                |
| [v2.7.1](#v271)    | July 2023      | v22.04 LTS           | v1.26                |
| [v2.7.0](#v270)    | May 2023       | v22.04 LTS           | v1.26                |


# Features for release
## v2.12.1
### New features
- None

### Changes to existing features
- ubi base image version upgraded to latest v9.8
- promethus/client_golang version set to 1.14.0
- logrus pkg version set to 1.9.1

### Fixed issues
- Fix for configuration restore when VFs are deleted
  by an external event.

### Known issues
- None

### Tested with Operating Systems
- Ubuntu 24.04
  - Kubernetes 1.28

## v2.12.0
### New features
- Operator Daemonset functionality enhancements
- Support for custom VRB resource names for multi node cluster deployments.
- Increase or decrease log level at runtime for debugging.

### Changes to existing features
- Log gather script enhancements for collecting additional information
- Updated ubi base image to latest version(9.6-1758184547)
- kube-rbac-proxy docker image pointing to new location: registry.k8s.io/kubebuilder/kube-rbac-proxy:v0.15.0
- golang version updated to 1.24.5
- x/net version updated to 0.44.0
- protobuf version updated to 1.5.4
- oauth2 version updated to 0.27.0

### Fixed issues
- None

### Known issues
- None

### Tested with Operating Systems
- Ubuntu 24.04
  - Kubernetes 1.26, 1.28
- OpenShift: 4.12 and higher versions

## v2.11.1
### New features
- None

### Changes to existing features
- None

### Fixed issues
- Fix for issue of configuration reapplied to accelerators in some cases when
  node having dual accelerators and both are configured.

### Known issues
- GNR-D with two VRB2 accelerator instances, in case of multiple worker nodes deployment
  user defined resource naming is not supported.

### Tested with Operating Systems
- Ubuntu 24.04 LTS
  - Kubernetes 1.26
- OpenShift: 4.12 and higher versions

## v2.11.0
### New features
- Updated pf-bb-config version to 25.01 to support VRB2 on GNR-D
- Bug fixes

### Changes to existing features
- Update UBI base image to latest version
- daemonset deployment configuration moved to configMaps to enable configuration changes
- golan/glog version update to 1.2.4

### Fixed issues
- Enhancement for dual VRB2 accelerator to map VF resource by explicit 
  mention to the underlying PF.
- Fix for the first accelerator is reconfigured when second accelerator is configured
  in case of dual accelerators present on same worker node.
- Fix for CR apply failure on ACC100 in HP DL110 platforms (issue #322)
- Set the protobuf version (1.33.0) properly in go.mod file

### Known issues
- GNR-D with two VRB2 accelerator instances, in case of multiple worker nodes deployment
  user defined resource naming is not supported.

### Tested with Operating Systems
- Ubuntu 24.04 LTS
  - Kubernetes 1.26
- OpenShift: 4.12 and higher versions

## v2.10.0
### New features
- Update pf-bb-config version to 24.11 to support VRB2 on GRN-D B0 ES2 Device.
- Reduce periodic log messages for controller-manager and daemonset
- Redirect pf-bb-config logs to daemonset pod logs

### Changes to existing features
- Telemetry functionality enhancements
  - Telemetry disabled by default
  - Config option for enable/disable
  - Config option for collect time interval
  - Addition validation in telemetry processing
- Update UBI base image to v9.5
- golang version update to 1.23.4
- Update kmod pkg version to 28.10-el9
- go x/net pkg version set to 0.23.0
- sriov-network-device-plugin version set to 3.7.0
- Update to collect VRB device configuration details for debugging

### Fixed issues
- Fix for golang linter errors
- Fix for OCP meta data annotations

### Known issues
- Limitation on two accelerator devices on same node
  - Request to a specific accelerator resource is not supported
  - Update/Delete of configuration on one accelerator will trigger reconfiguration on other accelerator device also.

### Tested with Operating Systems
- OS: Ubuntu 24.04 LTS
  - Kubernetes 1.26
- OpenShift: 4.12 and higher versions

## v2.9.0
### New features
- Updated pf-bb-conf version to 24.03
- drainSkip parameter default value set to true
- Log the PCI link status of accelerator in Daemon pod logs
- Add pf-bb-config tool version added to SFNC/SVNC output

### Changes to existing features
- refactoring daemon/reconciler for SFCC and SVCC APIs
- Bump controller tool version to fix nil pointer
- UBI base docker image version update to 9.4-947
- Daemon base docker image set to ubi-minimal v9.4-949
- UBI micro base docker image version set to 9.4-6
- xnet and protobuf pkg versions updated
- improvements to unit test coverage

### Fixed issues
- Applying SriovFecClusterConfig fails in some cases issue fixed
- Daemon loger issue fixed
- SPR-EE telemetry vfcount issue on bbdevconf end point is fixed

### Known issues
- N3000 8VF(max) SFCC configuration apply fail

### Tested with Operating Systems
- Ubuntu 22.04 LTS (Jammy Jellyfish)
	- Kubernetes 1.26.2
- OpenShift: 4.15.13
	- Kubernetes: v1.27.8+4fab27b

## v2.8.0
### New features
- Initial support for Intel vRAN Boost v2 (VRB2) on GNR-D (Early Access Pre-Alpha)
- Updated pf-bb-conf version to 23.11
- Ability to update srs_fft_windows_coefficient.bin file on worker node for VRB1 and VRB2.

### Changes to existing features
- UBI base docker image version updated to 9.3-6.
- sriov-network-device-plugin version updated to 4.14
- Telemetry request/response flow between daemon and pf-bb-conf is updated.
- xnet and go-logr package version updated.
- Restricted hostPath mount to read-only for /lib/modules.
- Restricted hostPath mount specific to device-plugin for sriov-network-device-plugin.
- Resource cleanup in proper order during the CR deletion.
- Restrict the vfio-pci driver parameter disable_idle_d3 set to ACC100 only.

### Fixed issues
- Fix for leader election resource cleanup after removal of Operator.
- Fix for logs collecting script.
- Fix for validation of maxNumQGroup parameter in CR.

### Known issues
- Applying SriovFecClusterConfig fails in some cases, a random failure. When it happens it is recommend to reboot the Node to recover from failure state. For stable version of FEC Operator deployment use v2.7.2.

### Tested with Operating Systems
- Ubuntu 22.04 LTS (Jammy Jellyfish)
  - Kernel: 5.15.0-72-generic, 5.15.0-1030-realtime
  - Kubernetes 1.26.2
- OpenShift: 4.14.7
  - Red Hat Enterprise Linux CoreOS 414.92.202312132152-0
  - Kubernetes: v1.27.8+4fab27b
  - RT Kernel: 5.14.0-284.45.1.rt14.330.el9_2.x86_64

## v2.7.2
### New features
- None

### Changes to existing features
- None

### Fixed issues
- Fix for failure in enabling VFs when kernel is overloaded

### Known issues
- None

### Tested with Operating Systems
- Ubuntu 22.04 LTS (Jammy Jellyfish)
  - Kernel: 5.15.0-72-generic, 5.15.0-1030-realtime
  - Kubernetes 1.26.2
- OpenShift: 4.13.0
  - Red Hat Enterprise Linux CoreOS 413.92.202305191644-0
  - Kubernetes: v1.26.3+b404935
  - RT Kernel: 5.14.0-284.13.1.el9_2.x86_64

## v2.7.1
### New features
- pf-bb-config updated (22.11 -> 23.03)
- Bug fixes

### Changes to existing features
- FEC resource names can be controlled through manager environment variables
- igb-uio driver is added to list of support drivers for VF interface
- Base images are updated to ubi9.2

### Fixed issues
- Fix for supporting multiple FEC devices on same node
- Fix for checking secure boot enabled mode

### Known issues
- None

### Tested with Operating Systems
- Ubuntu 22.04 LTS (Jammy Jellyfish)
  - Kubernetes 1.26.2
  - Kernel: 5.15.0-72-generic, 5.15.0-1030-realtime
- OpenShift: 4.13.0
  - Red Hat Enterprise Linux CoreOS 413.92.202305191644-0
  - Kubernetes: v1.26.3+b404935
  - RT Kernel: 5.14.0-284.13.1.el9_2.x86_64

## v2.7.0
### New features
- Support for OCP 4.13.x
- Bug fixes

### Changes to existing features
- VFIO token handling enhancements
- sriov-network-device-plugin version update to v4.14

### Fixed issues
- Enhanced error handling while processing telemetry data to fix Daemon crash addressing issue: https://github.com/intel/sriov-fec-operator/issues/48
- Leader lease renewal frequency configuration in case of Single Node Cluster addressing issue: https://github.com/intel/sriov-fec-operator/issues/36

### Known issues
- None

### Tested with Operating Systems
- Ubuntu 22.04 LTS (Jammy Jellyfish)
  - Kubernetes 1.26.2
  - Kernel: 5.15.0-72-generic, 5.15.0-1030-realtime
- OpenShift: 4.13.0
  - Red Hat Enterprise Linux CoreOS 413.92.202305191644-0
  - Kubernetes: v1.26.3+b404935
  - RT Kernel: 5.14.0-284.13.1.el9_2.x86_64

## v2.6.1
### New features
- pf-bb-config updated (22.07 -> 22.11)
- Added support for pf-bb-config telemetry
- Added support for ACC200 cards (SPR-EE)
- Operator now propagates `Tolerations` from Subscription to managed Daemonsets

### Changes to existing features
- Improved timeouts for LeaderElection functionality
- Manager deployment always starts with 1 replica and scales to 2 for multi-node clusters
- Base images are updated to ubi9.1 instead of ubi8.6
- Reduced RBAC permissions required by operator
- Daemon now has readiness and liveliness probes
- Removed mentions of Smart Edge Open from documentation. Operator is now standalone project.

### Fixed issues
- None

### Known issues
- None

### Tested with Operating Systems
- OpenShift: 4.12.0-rc.4
  - Red Hat Enterprise Linux CoreOS 412.86.202212081411-0
  - Kubernetes: v1.25.4+86bd4ff
  - RT Kernel: 4.18.0-372.36.1.rt7.193.el8_6.x86_64

## v2.6.0
### New features
- Support for OCP4.12.x

### Changes to existing features
- None

### Fixed issues
- None

### Known issues
- None

### Tested with Operating Systems
- OpenShift: 4.12.0-rc.2
  - Red Hat Enterprise Linux CoreOS 412.86.202211142021-0
  - Kubernetes: v1.25.2+cd98eda
  - RT Kernel: 4.18.0-425.3.1.rt7.213.el8.x86_64

## v2.5.0
### New features
- pf-bb-config updated (22.03 -> 22.07)
- Added support for Ubuntu 22.04
- Improved documentation for VFIO token

### Changes to existing features
- None

### Fixed issues
- None

### Known issues
- None

### Tested with Operating Systems
- Ubuntu 22.04 LTS (Jammy Jellyfish)
  - Kubernetes 1.23.5+c285e78
  - Kernel: 5.15.0-43-generic
- OpenShift: 4.11.5
  - Red Hat Enterprise Linux CoreOS 411.86.202209140028-0 (Ootpa)
  - Kubernetes: v1.24.0+3882f8f
  - RT Kernel: 4.18.0-372.26.1.rt7.183.el8_6.x86_64

## v2.4.0
### New features
- Support for OCP4.11.x

### Changes to existing features
- SriovFecClusterConfig.spec.physicalFunction.bbDevConfig field is now marked as 'required'

### Fixed issues
- None

### Known issues
- None

### Tested with Operating Systems
- OpenShift: 4.11.2
  - Red Hat Enterprise Linux CoreOS 411.86.202208191320-0 (Ootpa)
  - Kubernetes: v1.24.0+b62823b
  - RT Kernel: 4.18.0-372.19.1.rt7.176.el8_6.x86_64

## v2.3.1
### New features
- None

### Changes to existing features
- None

### Fixed issues
- fix for pf_bb_config throwing "MMIO is not accessible causing UR error over PCIe"

### Known issues
- None

### Tested with Operating Systems
- OpenShift: 4.10.21
  - Red Hat Enterprise Linux CoreOS 410.84.202206010432-0 (Ootpa)
  - Kubernetes: v1.23.5+3afdacb
  - RT Kernel: 4.18.0-305.49.1.rt7.121.el8_4.x86_64

## v2.3.0
### New features
- pf-bb-config updated (21.11 -> 22.03)
- Initial support of vfio-pci driver for ACC100 

### Changes to existing features
- Flattened sriov-fec operator structure by removing the `sriov-fec` directory
- Previous `labeler` directory acts now as internal package of sriov-fec operator
- Operator no longer adds missing kernel parameters `intel_iommu=on` and `iommu=pt`. User has to configure them [manually](https://wiki.ubuntu.com/Kernel/KernelBootParameters#Permanently_Add_a_Kernel_Boot_Parameter).

### Fixed issues
- None

### Known issues
- None

### Tested with Operating Systems
- OpenShift: 4.10.17
  - Red Hat Enterprise Linux CoreOS 410.84.202206010432-0 (Ootpa)
  - Kubernetes: v1.23.5+3afdacb
  - RT Kernel: 4.18.0-305.49.1.rt7.121.el8_4.x86_64

## v2.2.1
### New features
- Completed validation for MacLaren Summit card

### Changes to existing features
- None

### Fixed issues
- Adjusting CSV by adding relatedImages tag - addressing https://github.com/intel/sriov-fec-operator/issues/19

### Known issues
- None

### Tested with Operating Systems
- OpenShift: 4.9.36
  - Red Hat Enterprise Linux CoreOS 49.84.202205241705-0 (Ootpa)
  - Kubernetes: v1.22.8+f34b40c
  - RT Kernel: 4.18.0-305.45.1.rt7.117.el8_4.x86_64

## v2.2.0
### New features
- Support for OCP4.10.x

### Changes to existing features
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

### Fixed issues
- None

### Known issues
- None

### Tested with Operating Systems
- OpenShift: 4.9.7
  - Red Hat Enterprise Linux CoreOS 49.84.202111022104-0 (Ootpa)
  - Kubernetes: v1.22.2+5e38c72
  - RT Kernel: 4.18.0-305.25.1.rt7.97.el8_4.x86_64
- CentOS 7.9
  - Kubernetes: v1.22.2
  - RT Kernel: 3.10.0-1160.11.1.rt56.1145.el7.x86_64

## v2.1.0
### New features
- Support for OCP4.9.x
- Bugfixes

### Changes to existing features
- None

### Fixed issues
- SriovFecClusterConfig.spec.drainSkip was not rewritten into SriovFecNodeConfig.spec.drainSkip so SNO worker
  was trying to drain its workloads

### Known issues
- None

### Tested with Operating Systems
- OpenShift: 4.9.7
  - Red Hat Enterprise Linux CoreOS 49.84.202111022104-0 (Ootpa)
  - Kubernetes: v1.22.2+5e38c72
  - RT Kernel: 4.18.0-305.25.1.rt7.97.el8_4.x86_64

## v2.0.2
### New features
- None

### Changes to existing features
- Added webhook that converts existing SriovFecClusterConfigs with `nodes` field to SriovFecClusterConfig with `nodeSelector` and `acceleratorSelectors`
- Added webhook that prohibits creation of  SriovFecClusterConfig with `nodes` field.
- Daemon's reconciliation process trigger has been adjusted to cover multi-reboot scenarios

### Fixed issues
- SriovFecNodeConfig stucks in InProgress state(issue observed in case of multiple reboots)

### Known issues
- None

### Tested with Operating Systems
- OpenShift: 4.8.13
  - Red Hat Enterprise Linux CoreOS 48.84.202109210859-0 (Ootpa)
  - Kubernetes: v1.21.1+a620f50
  - RT Kernel: 4.18.0-305.19.1.rt7.91.el8_4.x86_64

## v2.0.1
### New features
- None

### Changes to existing features
- Daemon reconcile loop has been redesigned

### Fixed issues
- None

### Known issues
- None

### Tested with Operating Systems
- OpenShift: 4.8.13
  - Red Hat Enterprise Linux CoreOS 48.84.202109210859-0 (Ootpa)
  - Kubernetes: v1.21.1+a620f50
  - RT Kernel: 4.18.0-305.19.1.rt7.91.el8_4.x86_64

## v2.0.0
### New features
- Added new version (v2) of API with selectors
- Added resources cleanup on SriovFecClusterConfig deletion
- SriovFecController no longer overwrites ConfigMaps with `immutable` key
- Added support for deployment on K8S

### Changes to existing features
- Improved existing validation rules and added new rules
- Removed old API (v1)
- Updated pf-bb-config from 21.3 to 21.6 and OperatorSDK from 1.4.2 to 1.9.0

### Fixed issues
- None

### Known issues
- None

### Tested with Operating Systems
- OpenShift: 4.8.5
  - Red Hat Enterprise Linux CoreOS 48.84.202108062347-0
  - Kubernetes: v1.21.1+9807387
  - RT Kernel: 4.18.0-305.10.2.rt7.83.el8_4.x86_64

## v1.3.0
### New Features
- SEO SR-IOV Operator for Wireless FEC Accelerators OCP4.8.2 support
  - validated on ACC100 only

### Changes to Existing Features
- None

### Fixed Issues
- None

### Known issues
- None

### Tested with Operating Systems
- OpenShift: 4.8.2
  - Red Hat Enterprise Linux CoreOS 48.84.202107202156-0
  - Kubernetes: v1.21.1+051ac4f
  - RT Kernel: 4.18.0-305.10.2.rt7.83.el8_4.x86_64

## v1.2.1
### New features
- None

### Changes to existing features
- None

### Fixed issues
- [4.7.9 sriov-fec-v1.1.0 install does not succeed initially #270](https://github.com/intel/sriov-fec-operator/issues/270)

### Known issues
- None

### Tested with Operating Systems
- OpenShift: 4.7.8
  - Red Hat Enterprise Linux CoreOS 47.83.202104161442-0
  - Kubernetes: v1.20.0+7d0a2b2
  - RT Kernel: 4.18.0-240.22.1.rt7.77.el8_3.x86_64

## v1.2.0
### New features
- SEO SR-IOV Operator for Wireless FEC Accelerators OCP4.7.8 support
  - validated on ACC100 only

### Changes to existing features
- None

### Fixed issues
- None

### Known issues
- None

### Tested with Operating Systems
- OpenShift: 4.7.8
  - Red Hat Enterprise Linux CoreOS 47.83.202104161442-0
  - Kubernetes: v1.20.0+7d0a2b2
  - RT Kernel: 4.18.0-240.22.1.rt7.77.el8_3.x86_64

## v1.1.0
### New features
- SEO SR-IOV Operator for Wireless FEC Accelerators
  - Added support for Intel® vRAN Dedicated Accelerator ACC100
  - Independent accelerator discovery mechanism now enables standalone usage

### Changes to existing features
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

### Fixed issues
- SEO SR-IOV Operator for Wireless FEC Accelerators
  - Fixed status conditions to match convention introduced in N3000 operator
- Common
  - Fixed discovery for devices with LTE bitstream
  - Fixed field optionality policies in CRDs

### Known issues
- None

### Tested with Operating Systems
- OpenShift: 4.6.16
  - Red Hat Enterprise Linux CoreOS 46.82.202101301821-0
  - Kubernetes: v1.19.0+e49167a
  - RT Kernel: 4.18.0-193.41.1.rt13.91.el8_2.x86_64
  - OPAE: n3000-1.3.8-2-rte-el8
  - RTL Image: 20ww27.5-2x2x25G-5GLDPC-v1.6.1-3.0.0_unsigned.bin
  - NVM Package: v7.30

## v1.0.0
### New features
- SEO SRIOV-FEC Operator for Intel® FPGA PAC N3000
  - The SRIOV FEC operator handles the management of the FEC devices used to accelerate the FEC process in vRAN L1 applications
  - Create desired Virtual Functions for the FEC device, bind them to appropriate drivers and configure the VF's queues for desired functionality in 4G or 5G deployment
  - Deploys an instance of K8s SRIOV device plugin which manages the FEC VFs as an OpenShift cluster resource and configures this device plugin to detect the resources

### Changes to existing features
- There are no unsupported or discontinued features relevant to this release.

### Fixed issues
- None (First release)

### Known issues
- None

### Tested with Operating Systems
- OpenShift: 4.6.4
  - Red Hat Enterprise Linux CoreOS 46.82.202011061621-0
  - Kubernetes: v1.19.0+9f84db3
  - RT Kernel: 4.18.0-193.28.1.rt13.77.el8_2.x86_64
  - OPAE: n3000-1.3.8-2-rte-el8
  - RTL Image: 20ww27.5-2x2x25G-5GLDPC-v1.6.1-3.0.0_unsigned.bin
  - NVM Package: v7.30

> **_Single Node OpenShift (SNO)_**
>
>Daemon part (running on each featured worker node) of operator drains a node (moves its workloads to another node) before applying requested configuration.  
>Node draining doesn't work on SNO deployment. Because of that, operator's API exposes `SriovFecClusterConfig.spec.drainSkip` parameter which stops daemon doing workload migration.
>In theory it is all what is needed to find operator usable on SNO, however, operator's validation cycle is executed _ONLY_ on multi-worker-node clusters.
