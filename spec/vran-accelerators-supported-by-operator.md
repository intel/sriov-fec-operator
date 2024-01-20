```text
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2020-2024 Intel Corporation
```
<!-- omit in toc -->
# Intel's vRAN Boost accelerators supported by SRIOV-FEC Operator

- [Overview](#overview)
- [Intel® vRAN Dedicated Accelerator ACC100](#intel-vran-dedicated-accelerator-acc100)
  - [Intel® vRAN Dedicated Accelerator ACC100 FlexRAN Host Interface Overview](#intel-vran-dedicated-accelerator-acc100-flexran-host-interface-overview)
  - [SRIOV-FEC Operator for Intel® vRAN Dedicated Accelerator ACC100](#sriov-fec-operator-for-intel-vran-dedicated-accelerator-acc100)
- [Intel® vRAN Boost v1.0 (VRB1) Accelerator](#intel-vran-boost-v10-vrb1-accelerator)
- [Intel® vRAN Boost v2.0 (VRB2) Accelerator](#intel-vran-boost-v20-vrb2-accelerator)

## Overview

This document details the Intel's vRAN accelerator devices/hardware supported by the [SRIOV-FEC Operator for Wireless FEC Accelerators](https://github.com/smart-edge-open/sriov-fec-operator/blob/master/spec/openshift-sriov-fec-operator.md) in Red Hat's OpenShift Container Platform.

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

### SRIOV-FEC Operator for Intel® vRAN Dedicated Accelerator ACC100

The role of the operator for the Intel® vRAN Dedicated Accelerator ACC100 card is to orchestrate and manage the resources/devices exposed by the card within the OpenShift cluster. The operator is a state machine which will configure the resources and then monitor them and act autonomously based on the user interaction.
The operator design for Intel® vRAN Dedicated Accelerator ACC100 consist of:

* [SRIOV-FEC Operator for Wireless FEC Accelerators](https://github.com/smart-edge-open/sriov-fec-operator/blob/master/spec/sriov-fec-operator.md)

## Intel® vRAN Boost v1.0 (VRB1) Accelerator

The Intel® vRAN Boost integrated accelerator enables cost-effective 4G and 5G next-generation virtualized Radio Access Network (vRAN) solutions. The Intel vRAN Boost v1.0 (VRB1 in the code) is specifically integrated on the 4th Gen Intel® Xeon® Scalable processor with Intel® vRAN Boost, also known as Sapphire Rapids Edge Enhanced (SPR-EE).

Intel vRAN Boost v1.0 (VRB1) includes a 5G Low Density Parity Check (LDPC) encoder/decoder, rate match/dematch, Hybrid Automatic Repeat Request (HARQ) with access to DDR memory for buffer management, a 4G Turbo encoder/decoder, a Fast Fourier Transform (FFT) block providing DFT/iDFT processing offload for the 5G Sounding Reference Signal (SRS), a Queue Manager (QMGR), and a DMA subsystem. There is no dedicated on-card memory for HARQ, the coherent memory on the CPU side is being used.

## Intel® vRAN Boost v2.0 (VRB2) Accelerator

The Intel® vRAN Boost integrated accelerator enables cost-effective 4G and 5G next-generation virtualized Radio Access Network (vRAN) solutions. The Intel vRAN Boost v2.0 (VRB2) is specifically integrated on the Intel® Xeon® Granite Rapids-D Process (GNR-D).

Intel vRAN Boost v2.0 includes a 5G Low Density Parity Check (LDPC) encoder/decoder, rate match/dematch, Hybrid Automatic Repeat Request (HARQ) with access to DDR memory for buffer management, a 4G Turbo encoder/decoder, a Fast Fourier Transform (FFT) block providing DFT/iDFT processing offload for the 5G Sounding Reference Signal (SRS), a MLD-TS accelerator, a Queue Manager (QMGR), and a DMA subsystem. There is no dedicated on-card memory for HARQ, the coherent memory on the CPU side is being used.
