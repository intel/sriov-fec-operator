## Technical Requirements and Dependencies

The SEO Operator for Wireless FEC Accelerators has the following requirements:

- [Intel® vRAN Dedicated Accelerator ACC100](https://builders.intel.com/docs/networkbuilders/intel-vran-dedicated-accelerator-acc100-product-brief.pdf) (Optional)
- [Intel® FPGA PAC N3000 card](https://www.intel.com/content/www/us/en/programmable/products/boards_and_kits/dev-kits/altera/intel-fpga-pac-n3000/overview.html) (Optional)
- [Kubernetes 1.22](https://kubernetes.io/blog/2021/08/04/kubernetes-1-22-release-announcement/)
- RT Kernel configured for OS [Centos 7](https://linuxsoft.cern.ch/cern/centos/7/rt/x86_64/repoview/kernel-rt.html) or [Ubuntu](https://askubuntu.com/questions/1349568/installing-real-time-patch-for-ubuntu-20-04)
- [Configured kernel parameters](https://wiki.ubuntu.com/Kernel/KernelBootParameters#Permanently_Add_a_Kernel_Boot_Parameter): 
  - Always required: `"intel_iommu=on", "iommu=pt"`
  - sriov-fec:2.3.0 comes with experimental support of `vfio-pci` driver. Configurations leveraging `vfio-pci` require following kernel parameters:
    - vfio_pci.enable_sriov=1
    - vfio_pci.disable_idle_d3=1
    

### Setting Up CatalogSource
Prerequisite: Make sure that the images used by the operator are pushed to IMAGE_REGISTRY and all nodes in cluster have access to IMAGE_REGISTRY

If operator is built from source, then user has to create CatalogSource for OLM.

The operator-sdk CLI is required - see [Getting started with the Operator SDK](https://docs.openshift.com/container-platform/4.6/operators/operator_sdk/osdk-getting-started.html#osdk-installing-cli_osdk-getting-started).

Install OPM (if not already installed):

```shell
# export RELEASE_VERSION=v1.21.0
# curl -LO https://github.com/operator-framework/operator-registry/releases/download/${RELEASE_VERSION}/linux-amd64-opm
# chmod +x linux-amd64-opm
# sudo mkdir -p /usr/local/bin/
# sudo cp linux-amd64-opm /usr/local/bin/opm
# rm -f linux-amd64-opm
```

Determine local registry address:

```shell
# export IMAGE_REGISTRY=<IP_ADDRESS>:<PORT>
```

Navigate to operator directory:

```shell
# cd sriov-fec-operator
```

Export `VERSION` variable

```shell
# export VERSION=2.2.0
```

Build and push images to local registry:

```shell
# IMAGE_REGISTRY=${IMAGE_REGISTRY} TLS_VERIFY=false VERSION=${VERSION} make build_all
```

Create and push the index image:

```shell
# IMAGE_REGISTRY=${IMAGE_REGISTRY} TLS_VERIFY=false VERSION=${VERSION} make build_index
```

Create the catalog source:

```shell
# cat <<EOF | kubectl apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
    name: intel-operators
    namespace: openshift-marketplace
spec:
    sourceType: grpc
    image: ${IMAGE_REGISTRY}/sriov-fec-index:${VERSION}
    publisher: Intel
    displayName: SRIOV-FEC operator(Local)
EOF
```

Wait for `packagemanifest` to be available:

```shell
[user@ctrl1 /home]# kubectl get packagemanifests sriov-fec

 NAME        CATALOG                     AGE
 sriov-fec   SRIOV-FEC operator(Local)   24s
```

### Install dependencies

If Kubernetes doesn't have installed OLM (Operator lifecycle manager) start from installing Operator-sdk (https://olm.operatorframework.io/)

After Operator-sdk installation run following command
```shell
[user@ctrl1 /home]# operator-sdk olm install
```
Install PCIutils on worker nodes
```shell
[user@ctrl1 /home]# yum install pciutils
```
### Install the Bundle

To install the SEO Operator for Wireless FEC Accelerators operator bundle perform the following steps:

Create the project:
```shell
[user@ctrl1 /home]# kubectl create namespace vran-acceleration-operators
[user@ctrl1 /home]# kubectl config set-context --current --namespace=vran-acceleration-operators
```
Execute following commands on cluster:

Create an operator group and the subscriptions (all the commands are run in the `vran-acceleration-operators` namespace):

```shell
[user@ctrl1 /home]#  cat <<EOF | kubectl apply -f -
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: vran-operators
  namespace: vran-acceleration-operators
spec:
  targetNamespaces:
    - vran-acceleration-operators
EOF
```

```shell
[user@ctrl1 /home]#  cat <<EOF | kubectl apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: sriov-fec-subscription
  namespace: vran-acceleration-operators
spec:
  channel: stable
  name: sriov-fec
  source: intel-operators
  sourceNamespace: olm
EOF
```

Verify that the operator is installed and pods are running:

```shell
[user@ctrl1 /home]# kubectl get csv
NAME               DISPLAY                                                        VERSION   REPLACES   PHASE
sriov-fec.v2.2.0   SEO SR-IOV Operator for Wireless FEC Accelerators              2.2.0                Succeeded
```

```shell
[user@ctrl1 /home]# kubectl get pod
NAME                                            READY   STATUS    RESTARTS   AGE                                                                              
                                           
sriov-device-plugin-hkq6f                       1/1     Running   0          35s                                                                              
sriov-fec-controller-manager-78488c4c65-cpknc   2/2     Running   0          44s                                                                              
sriov-fec-daemonset-7h8kb                       1/1     Running   0          35s                                                                              
```

In following chapters (for [example](sriov-fec-sriov-fec-operator.md#uninstalling-previously-installed-operator) , use `kubectl` instead of `oc` in commands.
