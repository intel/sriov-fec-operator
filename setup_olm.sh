#!/bin/bash

PATH=$PATH:../bin

set +x

oc delete ns intel-fpga-operators

# oc describe csv n3000.v2.0.0 -n intel-fpga-operators
# oc delete csv n3000.v1.1.0 -n default
# oc delete Catalogsources n3000-catalog -n default
# oc delete Subscription n3000-v2-0-0-sub -n default
# oc delete ns intel-fpga-operators

oc create ns intel-fpga-operators
#oc new-project intel-fpga-operators
operator-sdk run bundle ryan_raasch/intel-fpga-bundle:v2.0.0 --verbose -n intel-fpga-operators

# oc describe csv n3000.v2.0.0 -n intel-fpga-operator
# oc describe Clusterserviceversion -n intel-fpga-operators
# oc describe Operatorgroups operator-sdk-og -n intel-fpga-operators
