```text
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2020-2025 Intel Corporation
```
<!-- omit in toc -->
# Intel's vRAN accelerators supported by SRIOV-FEC Operator on OpenShift

- [Overview](#overview)
- [Intel® vRAN Dedicated Accelerator ACC100](#intel-vran-dedicated-accelerator-acc100)
- [Intel® FPGA Programmable Acceleration Card N3000](#intel-fpga-programmable-acceleration-card-n3000)
- [Intel® vRAN Boost Accelerators](#intel-vran-boost-accelerators)
  - [Intel® vRAN Boost Accelerator FlexRAN Host Interface Overview](#intel-vran-boost-accelerator-flexran-host-interface-overview)
  - [SRIOV-FEC Operator for Intel® vRAN Boost Accelerators](#sriov-fec-operator-for-intel-vran-boost-accelerators)
- [Intel® vRAN Boost Accelerator V1 (VRB1)](#intel-vran-boost-accelerator-v1-vrb1)
- [Intel® vRAN Boost Accelerator V2 (VRB2)](#intel-vran-boost-accelerator-v2-vrb2)

## Overview

This document details the Intel's vRAN accelerator devices/hardware supported by the [SRIOV-FEC Operator for Wireless FEC Accelerators](https://github.com/intel/sriov-fec-operator/blob/master/spec/openshift-sriov-fec-operator.md) in Red Hat's OpenShift Container Platform.

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

## Intel® FPGA Programmable Acceleration Card N3000

The Intel® FPGA Programmable Acceleration Card N3000 is designed to provide flexible and high-performance acceleration for 4G and 5G next-generation virtualized Radio Access Network (vRAN) solutions. This card is particularly suited for applications requiring adaptable and scalable processing capabilities in cloud-native environments.

These hardware blocks provide the following features exposed by the PMD:

- LDPC Encode in the Downlink (5GNR)
- LDPC Decode in the Uplink (5GNR)
- Turbo Encode in the DL with total throughput of 4.5 Gbits/s
- Turbo Decode in the UL with total throughput of 1.5 Gbits/s assuming 8 decoder iterations
- 8 VFs per PF (physical device)
- Maximum of 32 UL queues per VF
- Maximum of 32 DL queues per VF
- PCIe Gen-3 x8 Interface
- MSI-X
- SR-IOV

For more information, see guide in Intel® FPGA Programmable Acceleration Card N3000 [(LTE)](https://doc.dpdk.org/guides/bbdevs/fpga_lte_fec.html) [(5GNR)](https://doc.dpdk.org/guides/bbdevs/fpga_5gnr_fec.html).

## Intel® vRAN Boost Accelerators
Intel® vRAN Boost Accelerators, including the VRB1 and VRB2 models, represent a significant advancement in the field of virtualized Radio Access Network (vRAN) solutions. These accelerators are designed to enhance the performance and efficiency of 4G and 5G networks by offloading complex processing tasks from the CPU, thereby freeing up resources and reducing latency. Integrated with Intel® Xeon® processors, they provide a comprehensive suite of features such as Low Density Parity Check (LDPC) encoding/decoding, Turbo encoding/decoding, Fast Fourier Transform (FFT) processing, and Hybrid Automatic Repeat Request (HARQ) buffer management. The VRB1 and VRB2 models support Single Root I/O Virtualization (SR-IOV), allowing for multiple virtual functions per physical function, which is crucial for scalable and flexible network deployments. Additionally, the VRB2 introduces advanced capabilities like MLD-TS processing and a higher queue count per virtual function, further enhancing throughput and scalability. These accelerators are pivotal in enabling cost-effective, high-performance vRAN solutions that can adapt to various deployment scenarios, from small cells to massive MIMO networks, making them indispensable for modern telecommunications infrastructure.

### Intel® vRAN Boost Accelerator FlexRAN Host Interface Overview

FlexRAN is a reference layer 1 pipeline of 4G eNb and 5G gNb on Intel® architecture. The FlexRAN reference pipeline consists of an L1 pipeline, optimized L1 processing modules, BBU pooling framework, cloud and cloud-native deployment support, and accelerator support for hardware offload. Intel® vRAN Boost Accelerators, such as VRB1 and VRB2, are used by FlexRAN to offload FEC (Forward Error Correction) and other processing tasks for 4G and 5G.
Intel® vRAN Boost Accelerators used in the FlexRAN solution expose the following physical functions to the CPU host:
- Multiple FEC interfaces that can be used for 4G or 5G FEC acceleration
  - The LTE FEC IP components include turbo encoder/turbo decoder and rate matching/de-matching
  - The 5GNR FEC IP components include low-density parity-check (LDPC) Encoder / LDPC Decoder, rate matching/de-matching, and UL HARQ combining
  - Additional processing capabilities such as FFT and MLD-TS for enhanced signal processing
These accelerators provide a comprehensive suite of features that enhance the performance and efficiency of vRAN deployments, supporting a wide range of network configurations from small cells to massive MIMO networks.

### SRIOV-FEC Operator for Intel® vRAN Boost Accelerators

The role of the operator for Intel® vRAN Boost Accelerators, such as VRB1 and VRB2, is to orchestrate and manage the resources/devices exposed by these accelerators within the OpenShift cluster. The operator acts as a state machine, configuring the resources and monitoring them to ensure optimal performance and responsiveness to user interactions.
The operator design for Intel® vRAN Boost Accelerators consists of:

* [SRIOV-FEC Operator for Wireless FEC Accelerators](https://github.com/intel/sriov-fec-operator/blob/master/spec/sriov-fec-operator.md)

This operator facilitates the integration of Intel® vRAN Boost Accelerators into cloud-native environments, enabling efficient resource allocation and management, and supporting advanced features such as SR-IOV for virtualization and high queue counts for enhanced throughput and scalability.

## Intel® vRAN Boost Accelerator V1 (VRB1)

The Intel® vRAN Boost Accelerator V1 (VRB1) peripheral enables cost-effective 4G and 5G next-generation virtualized Radio Access Network (vRAN) solutions integrated on 4th Gen Intel® Xeon® Scalable processor with Intel® vRAN Boost, also known as Sapphire Rapids Edge Enhanced Processor (SPR-EE).

The VRB1 includes a 5G Low Density Parity Check (LDPC) encoder/decoder, rate match/dematch, Hybrid Automatic Repeat Request (HARQ) with access to DDR memory for buffer management, a 4G Turbo encoder/decoder, a Fast Fourier Transform (FFT) block providing DFT/iDFT processing offload for the 5G Sounding Reference Signal (SRS), a Queue Manager (QMGR), and a DMA subsystem. There is no dedicated on-card memory for HARQ, this is using coherent memory on the CPU side.

These hardware blocks provide the following features exposed by the PMD:

- LDPC Encode in the Downlink (5GNR)
- LDPC Decode in the Uplink (5GNR)
- Turbo Encode in the Downlink (4G)
- Turbo Decode in the Uplink (4G)
- FFT processing
- Single Root I/O Virtualization (SR-IOV) with 16 Virtual Functions (VFs) per Physical Function (PF)
- Maximum of 256 queues per VF

For more information, see guide in [Intel® vRAN Boost Accelerator V1](https://www.intel.com/content/dam/www/central-libraries/us/en/documents/2023-02/4th-gen-xeon-scalable-vran-product-brief-final.pdf).

## Intel® vRAN Boost Accelerator V2 (VRB2)

The Intel® vRAN Boost Accelerator V2 (VRB2) peripheral enables cost-effective 4G and 5G next-generation virtualized Radio Access Network (vRAN) solutions integrated on Intel® Xeon® 6 SoC, also known as Granite Rapids-D Process (GNR-D).

Intel vRAN Boost v2.0 includes a 5G Low Density Parity Check (LDPC) encoder/decoder, rate match/dematch, Hybrid Automatic Repeat Request (HARQ) with access to DDR memory for buffer management, a 4G Turbo encoder/decoder, a Fast Fourier Transform (FFT) block providing DFT/iDFT processing offload for the 5G Sounding Reference Signal (SRS), a MLD-TS accelerator, a Queue Manager (QMGR), and a DMA subsystem. There is no dedicated on-card memory for HARQ, the coherent memory on the CPU side is being used.

These hardware blocks provide the following features exposed by the PMD:

- LDPC Encode in the Downlink (5GNR)
- LDPC Decode in the Uplink (5GNR)
- Turbo Encode in the Downlink (4G)
- Turbo Decode in the Uplink (4G)
- FFT processing, notably suited for SRS and PRACH acceleration
- MLD-TS processing
- Single Root I/O Virtualization (SR-IOV) with 16 Virtual Functions (VFs) per Physical Function (PF)
- Maximum of 2048 queues per VF
- Message Signaled Interrupts (MSIs)

For more information, see guide in [Intel® vRAN Boost Accelerator V2](https://www.intel.com/content/dam/www/central-libraries/us/en/documents/2025-02/vran-xeon6-soc-p-cores-solution-brief-final.pdf).