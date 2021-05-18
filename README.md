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
