```text
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2020-2021 Intel Corporation
```
<!-- omit in toc -->
# Intel's vRAN accelerators supported by SEO Operators on OpenShift

- [Overview](#overview)
- [Intel® PAC N3000 for vRAN Acceleration](#intel-pac-n3000-for-vran-acceleration)
  - [Enabling 5G Wireless Acceleration in FlexRAN](#enabling-5g-wireless-acceleration-in-flexran)
  - [SEO Operator for Intel® FPGA PAC N3000](#seo-operator-for-intel-fpga-pac-n3000)
- [Intel® vRAN Dedicated Accelerator ACC100](#intel-vran-dedicated-accelerator-acc100)
  - [Intel® vRAN Dedicated Accelerator ACC100 FlexRAN Host Interface Overview](#intel-vran-dedicated-accelerator-acc100-flexran-host-interface-overview)
  - [SEO Operator for Intel® vRAN Dedicated Accelerator ACC100](#seo-operator-for-intel-vran-dedicated-accelerator-acc100)

## Overview

This document details the Intel's vRAN accelerator devices/hardware supported by the [SEO Operator for Wireless FEC Accelerators](https://github.com/smart-edge-open/sriov-fec-operator/blob/main/spec/openshift-sriov-fec-operator.md) in Red Hat's OpenShift Container Platform.

## Intel® PAC N3000 for vRAN Acceleration

The Intel® FPGA PAC N3000 plays a key role in accelerating certain types of workloads, which in turn increases the overall compute capacity of a commercial, off-the-shelf platform. FPGA benefits include:

* Flexibility - FPGA functionality can change upon every power up of the device.
* Acceleration - Get products to market faster and increase your system performance.
* Integration - Modern FPGAs include on-die processors, transceiver I/Os at 28 Gbps (or faster), RAM blocks, DSP engines, and more.
* Total Cost of Ownership (TCO) - While ASICs may cost less per unit than an equivalent FPGA, building them requires a non-recurring expense (NRE), expensive software tools, specialized design teams, and long manufacturing cycles.

The deployment of artificial intelligence (AI) and machine learning (ML) applications at the edge is increasing the adoption of FPGA acceleration. This trend of devices performing machine learning at the edge locally versus relying solely on the cloud is driven by the need to lower latency, persistent availability, lower costs, and address privacy concerns.

The Intel® FPGA PAC N3000 is used as a reference FPGA and uses LTE/5G Forward Error Correction (FEC) as an example workload that accelerates the 5G or 4G L1 base station network function. The same concept and mechanism is applicable for application acceleration workloads like AI and ML on FPGA for applications.

The Intel® FPGA PAC N3000 is a full-duplex, 100 Gbps in-system, re-programmable acceleration card for multi-workload networking application acceleration. It has an optimal memory mixture designed for network functions, with an integrated network interface card (NIC) in a small form factor that enables high throughput, low latency, and low power per bit for a custom networking pipeline.

This card can be used in conjunction with an [Intel FlexRAN](https://software.intel.com/content/www/us/en/develop/videos/how-radio-access-network-is-being-virtualized-and-the-role-of-flexran.html?wapkw=FlexRAN) project. FlexRAN is a reference layer 1 pipeline of 4G eNb and 5G gNb on Intel® architecture. The FlexRAN reference pipeline consists of an L1 pipeline, optimized L1 processing modules, BBU pooling framework, cloud and cloud-native deployment support, and accelerator support for hardware offload. The Intel® FPGA PAC N3000 card is used by FlexRAN to offload FEC for 4G and 5G as well as IO for fronthaul and midhaul.

> Note: User needs to have Intel® FlexRAN licence before downloading the required package from [Intel RDC](https://www.intel.com/content/www/us/en/design/resource-design-center.html)

The Intel® FPGA PAC N3000 card used in the FlexRAN solution exposes the following physical functions to the CPU host:

* 2x2x25G Ethernet* interface that can be used for Fronthaul or Midhaul
* One FEC interface that can be used for 4G or 5G FEC look-aside acceleration via PCIe offload
* The LTE FEC IP components have turbo encoder/turbo decoder and rate matching/de-matching
* The 5GNR FEC IP components have low-density parity-check (LDPC) Encoder / LDPC Decoder, rate matching/de-matching, and UL HARQ combining
* Interface for managing and updating the FPGA Image through Remote System Update (RSU)

![Intel® PAC N3000 Host interface overview](images/seo-fpga1.png)

### Enabling 5G Wireless Acceleration in FlexRAN

The 5G Wireless Acceleration reference design provides IP (Intel® FPGA IP and software drivers) to support fronthaul IO and 5G channel coding, FEC. The Intel® FPGA PAC N3000 provides an on-board PCIe* switch that connects fronthaul and 5G channel coding functions to a PCIe* Gen3x16 edge connector. The Intel® FPGA PAC N3000 is a general-purpose acceleration card for networking.

![Data flow for the user image, FEC, and Fronthaul IO](images/Intel-N3000-5G-pipeline.png)

### SEO Operator for Intel® FPGA PAC N3000

The role of the operator for the Intel® FPGA PAC N3000 card is to orchestrate and manage the resources/devices exposed by the card within the OpenShift cluster. The operator is a state machine which will configure the resources and then monitor them and act autonomously based on the user interaction.

## Intel® vRAN Dedicated Accelerator ACC100

Intel® vRAN Dedicated Accelerator ACC100 plays a key role in accelerating 4G and 5G Virtualized Radio Access Networks (vRAN) workloads, which in turn increases the overall compute capacity of a commercial, off-the-shelf platform.

Intel® vRAN Dedicated Accelerator ACC100 provides the following features:

- LDPC FEC processing for 3GPP 5G:
  - LDPC encoder/decoder
  - Code block CRC generation/checking
  - Rate matching/de-matching
  - HARQ buffer management
- Turbo FEC processing for 3GPP 4G:
  - Turbo encoder/decoder
  - Code block CRC generation/checking
  - Rate matching/de-matching
- Scalable to required system configuration
- Hardware DMA support
- Performance monitoring
- Load balancing supported by the hardware queue manager (QMGR)
- Interface through the DPDK BBDev library and APIs

Intel® vRAN Dedicated Accelerator ACC100 benefits include:
- Reduced platform power, E2E latency and Intel® CPU core count requirements as well as increase in cell capacity than existing programmable accelerator
- Accelerates both 4G and 5G data concurrently
- Lowers development cost using commercial off the shelf (COTS) servers
- Accommodates space-constrained implementations via a low-profile PCIe* card form factor
- Enables a variety of flexible FlexRAN deployments from small cell to macro to Massive
MIMO networks
- Supports extended temperature for the most challenging of RAN deployment scenarios

For more information, see product brief in [Intel® vRAN Dedicated Accelerator ACC100](https://builders.intel.com/docs/networkbuilders/intel-vran-dedicated-accelerator-acc100-product-brief.pdf).

### Intel® vRAN Dedicated Accelerator ACC100 FlexRAN Host Interface Overview

FlexRAN is a reference layer 1 pipeline of 4G eNb and 5G gNb on Intel® architecture. The FlexRAN reference pipeline consists of an L1 pipeline, optimized L1 processing modules, BBU pooling framework, cloud and cloud-native deployment support, and accelerator support for hardware offload. Intel® vRAN Dedicated Accelerator ACC100 card is used by FlexRAN to offload FEC (Forward Error Correction) for 4G and 5G.

Intel® vRAN Dedicated Accelerator ACC100 card used in the FlexRAN solution exposes the following physical functions to the CPU host:
- One FEC interface that can be used of 4G or 5G FEC acceleration
  - The LTE FEC IP components have turbo encoder/turbo decoder and rate matching/de-matching
  - The 5GNR FEC IP components have low-density parity-check (LDPC) Encoder / LDPC Decoder, rate matching/de-matching, and UL HARQ combining

![Intel® vRAN Dedicated Accelerator ACC100 support](images/acc100-diagram.png)

### SEO Operator for Intel® vRAN Dedicated Accelerator ACC100

The role of the operator for the Intel® vRAN Dedicated Accelerator ACC100 card is to orchestrate and manage the resources/devices exposed by the card within the OpenShift cluster. The operator is a state machine which will configure the resources and then monitor them and act autonomously based on the user interaction.
The operator design for Intel® vRAN Dedicated Accelerator ACC100 consist of:

* [SEO Operator for Wireless FEC Accelerators](https://github.com/smart-edge-open/sriov-fec-operator/blob/main/spec/openshift-sriov-fec-operator.md)
