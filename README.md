# N5010 openshift-operator

oc apply -k N3000/config/default

# Image locations.

* quay.io/ryan_raasch/intel-fpga-operator:v2.0.0
* quay.io/ryan_raasch/intel-fpga-daemon:v2.0.0
* quay.io/ryan_raasch/intel-fpga-monitoring:v2.0.0
* quay.io/ryan_raasch/intel-fpga-labeler:v2.0.0
* quay.io/ryan_raasch/dfl-kmod:eea9cbc-4.18.0-193.el8.x86_64

## namespace: intel-fpga-operators

# Node labels
oc get node  -l fpga.intel.com/network-accelerator-n5010=

# Deployment using operator-sdk and bundle
oc create ns intel-fpga-operators

operator-sdk run bundle quay.io/ryan_raasch/intel-fpga-bundle:v2.0.0 --verbose -n intel-fpga-operators

# Create a green or blue stream update on node
```
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2020 Intel Corporation

apiVersion: fpga.intel.com/v1
kind: N3000Node
metadata:
  name: n3000
  namespace: intel-fpga-operators
spec:
  nodes:
    - nodeName: worker1
      fpga:
        - userImageURL: "http://10.100.1.78:8080/N5010/hw/green_bits/N5010_ofs-fim_PR_0_0_1__afu_example_axi_pim_unsigned.bin"
          PCIAddr: "0000:00:04.0"
          checksum: "890285f7304e01de546ac5a65574a8ab"
```
