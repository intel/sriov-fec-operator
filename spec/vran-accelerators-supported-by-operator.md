```text
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2020-2022 Intel Corporation
```
<!-- omit in toc -->
# Intel's vRAN accelerators supported by SRIOV-FEC Operator on OpenShift

- [Overview](#overview)
- [Intel® vRAN Dedicated Accelerator ACC100](#intel-vran-dedicated-accelerator-acc100)
  - [Intel® vRAN Dedicated Accelerator ACC100 FlexRAN Host Interface Overview](#intel-vran-dedicated-accelerator-acc100-flexran-host-interface-overview)
  - [SRIOV-FEC Operator for Intel® vRAN Dedicated Accelerator ACC100](#sriov-fec-operator-for-intel-vran-dedicated-accelerator-acc100)
- [Intel® ACC200 vRAN Dedicated Accelerator](#intel-acc200-vran-dedicated-accelerator)

## Overview

This document details the Intel's vRAN accelerator devices/hardware supported by the [SRIOV-FEC Operator for Wireless FEC Accelerators](https://github.com/intel-collab/applications.orchestration.operators.sriov-fec-operator/blob/master/spec/openshift-sriov-fec-operator.md) in Red Hat's OpenShift Container Platform.

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

* [SRIOV-FEC Operator for Wireless FEC Accelerators](https://github.com/intel-collab/applications.orchestration.operators.sriov-fec-operator/blob/master/spec/sriov-fec-operator.md)

# Intel® ACC200 vRAN Dedicated Accelerator

The Intel® vRAN Dedicated Accelerator ACC200 peripheral enables cost-effective 4G and 5G next-generation virtualized Radio Access Network (vRAN) solutions integrated on Sapphire Rapids Edge Enhanced Processor (SPR-EE) Intel® 7 based Xeon® multi-core server processor.

The ACC200 includes a 5G Low Density Parity Check (LDPC) encoder/decoder, rate match/dematch, Hybrid Automatic Repeat Request (HARQ) with access to DDR memory for buffer management, a 4G Turbo encoder/decoder, a Fast Fourier Transform (FFT) block providing DFT/iDFT processing offload for the 5G Sounding Reference Signal (SRS), a Queue Manager (QMGR), and a DMA subsystem. There is no dedicated on-card memory for HARQ, this is using coherent memory on the CPU side.