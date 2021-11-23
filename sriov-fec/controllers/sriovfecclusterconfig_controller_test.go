// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"github.com/onsi/gomega/gstruct"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/api/meta"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	sriovv2 "github.com/otcshare/openshift-operator/sriov-fec/api/v2"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	nodeConfigPrototype = &sriovv2.SriovFecNodeConfig{
		ObjectMeta: v1.ObjectMeta{
			Namespace: NAMESPACE,
		},
		Spec: sriovv2.SriovFecNodeConfigSpec{
			PhysicalFunctions: []sriovv2.PhysicalFunctionConfigExt{},
		},
		Status: sriovv2.SriovFecNodeConfigStatus{
			Inventory: sriovv2.NodeInventory{
				SriovAccelerators: []sriovv2.SriovAccelerator{},
			},
		},
	}

	clusterConfigPrototype = &sriovv2.SriovFecClusterConfig{
		ObjectMeta: v1.ObjectMeta{
			Name:      "config",
			Namespace: NAMESPACE,
		},
		Spec: sriovv2.SriovFecClusterConfigSpec{
			NodeSelector: map[string]string{},
			PhysicalFunction: sriovv2.PhysicalFunctionConfig{
				//PCIAddress: "0000:14:00.1",
				PFDriver: "d",
				VFDriver: "v",
				VFAmount: 5,
				BBDevConfig: sriovv2.BBDevConfig{
					N3000: &sriovv2.N3000BBDevConfig{
						NetworkType: "FPGA_LTE",
						PFMode:      false,
						FLRTimeOut:  10,
						Downlink: sriovv2.UplinkDownlink{
							Bandwidth:   3,
							LoadBalance: 3,
							Queues: sriovv2.UplinkDownlinkQueues{
								VF0: 0,
								VF1: 1,
								VF2: 2,
								VF3: 3,
								VF4: 4,
								VF5: 5,
								VF6: 6,
								VF7: 7,
							},
						},
						Uplink: sriovv2.UplinkDownlink{
							Bandwidth:   2,
							LoadBalance: 2,
							Queues: sriovv2.UplinkDownlinkQueues{
								VF0: 0,
								VF1: 1,
								VF2: 2,
								VF3: 3,
								VF4: 4,
								VF5: 5,
								VF6: 6,
								VF7: 7,
							},
						},
					},
				},
			},
		},
	}

	nodePrototype = &corev1.Node{
		ObjectMeta: v1.ObjectMeta{
			Name: "node-dummy",
			Labels: map[string]string{
				"fpga.intel.com/intel-accelerator-present": "",
			},
		},
	}
)

var _ = Describe("SriovControllerTest", func() {
	var _ = Describe("Reconciler", func() {
		var log = logrus.New()

		createNodeInventory := func(nodeName string, inventory []sriovv2.SriovAccelerator) {
			nodeConfig := nodeConfigPrototype.DeepCopy()
			nodeConfig.Name = nodeName
			Expect(k8sClient.Create(context.TODO(), nodeConfig)).ToNot(HaveOccurred())

			nodeConfig.Status.Inventory.SriovAccelerators = inventory
			Expect(k8sClient.Status().Update(context.TODO(), nodeConfig)).ToNot(HaveOccurred())
			Expect(nodeConfig.Status.Inventory.SriovAccelerators).To(HaveLen(len(inventory)))
		}

		createNode := func(name string, configurers ...func(n *corev1.Node)) *corev1.Node {
			node := nodePrototype.DeepCopy()
			node.Name = name
			for _, configure := range configurers {
				configure(node)
			}
			Expect(k8sClient.Create(context.TODO(), node)).ToNot(HaveOccurred())
			return node
		}

		createAcceleratorConfig := func(configName string, configurers ...func(cc *sriovv2.SriovFecClusterConfig)) *sriovv2.SriovFecClusterConfig {
			cc := clusterConfigPrototype.DeepCopy()
			cc.Name = configName
			for _, configure := range configurers {
				configure(cc)
			}
			Expect(k8sClient.Create(context.TODO(), cc)).ToNot(HaveOccurred())
			return cc
		}

		createDummyReconcileRequest := func(ccName string) ctrl.Request {
			return ctrl.Request{NamespacedName: types.NamespacedName{Namespace: NAMESPACE, Name: ccName}}
		}

		reconcile := func(ccName string) *SriovFecClusterConfigReconciler {
			reconciler := SriovFecClusterConfigReconciler{k8sClient, log, scheme.Scheme}
			_, err := reconciler.Reconcile(context.TODO(), createDummyReconcileRequest(ccName))
			Expect(err).ToNot(HaveOccurred())
			return &reconciler
		}

		AfterEach(func() {
			ccl := new(sriovv2.SriovFecClusterConfigList)
			Expect(k8sClient.List(context.TODO(), ccl)).ToNot(HaveOccurred())
			for _, item := range ccl.Items {
				Expect(k8sClient.Delete(context.TODO(), &item)).ToNot(HaveOccurred())
			}

			ncl := new(sriovv2.SriovFecNodeConfigList)
			Expect(k8sClient.List(context.TODO(), ncl)).ToNot(HaveOccurred())
			for _, item := range ncl.Items {
				Expect(k8sClient.Delete(context.TODO(), &item)).ToNot(HaveOccurred())
			}

			Expect(k8sClient.DeleteAllOf(context.TODO(), &corev1.Node{})).ToNot(HaveOccurred())
		})

		When("Error occurs during SriovFecClusterConfig->SriovFecNodeConfig propagation", func() {
			It("ConfigurationPropagationCondition should appear on SriovFecNodeConfig", func() {
				n1 := createNode("n1")

				//Inventory is broken since it doesn't expose PcieAddress field which is obligatory,
				//It comes with a reason, when controller will try to rewrite cluster config spec into node config spec,
				//request should be rejected(again PciAddress field is obligatory)
				createNodeInventory(n1.Name, []sriovv2.SriovAccelerator{
					{
						VendorID: "vendor",
						VFs:      []sriovv2.VF{},
					},
				})

				createAcceleratorConfig("cc", func(cc *sriovv2.SriovFecClusterConfig) {
					cc.Spec.AcceleratorSelector = sriovv2.AcceleratorSelector{
						VendorID: "vendor",
					}
					cc.Spec.PhysicalFunction = sriovv2.PhysicalFunctionConfig{
						PFDriver: "pfDriver",
					}
				})

				reconcile("cc")

				nc := new(sriovv2.SriovFecNodeConfig)
				Expect(k8sClient.Get(context.TODO(), client.ObjectKey{Name: n1.Name, Namespace: NAMESPACE}, nc)).ToNot(HaveOccurred())
				Expect(nc.Spec.PhysicalFunctions).To(BeEmpty())

				conditionToCheck := meta.FindStatusCondition(nc.Status.Conditions, "ConfigurationPropagationCondition")
				Expect(conditionToCheck).ToNot(BeNil())
				Expect(*conditionToCheck).
					To(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{"Reason": Equal("Failed")}))

			})
		})

		When("single cc does not match to any node", func() {
			It("node config should not be propagated", func() {
				n1 := createNode("n1")
				n2 := createNode("n2")

				createNodeInventory(n1.Name, []sriovv2.SriovAccelerator{
					{
						VendorID: "vendor",
						VFs:      []sriovv2.VF{},
					},
				})

				createNodeInventory(n2.Name, []sriovv2.SriovAccelerator{
					{
						PCIAddress: "vendor",
						VFs:        []sriovv2.VF{},
					},
				})

				createAcceleratorConfig("cc", func(cc *sriovv2.SriovFecClusterConfig) {
					cc.Spec.AcceleratorSelector = sriovv2.AcceleratorSelector{
						VendorID: "notExistingVendor",
					}
					cc.Spec.PhysicalFunction = sriovv2.PhysicalFunctionConfig{
						PFDriver: "pfDriver",
					}
				})

				reconcile("cc")

				nc := new(sriovv2.SriovFecNodeConfig)
				Expect(k8sClient.Get(context.TODO(), client.ObjectKey{Name: n1.Name, Namespace: NAMESPACE}, nc)).ToNot(HaveOccurred())
				Expect(nc.Spec.PhysicalFunctions).To(BeEmpty())

				nc = new(sriovv2.SriovFecNodeConfig)
				Expect(k8sClient.Get(context.TODO(), client.ObjectKey{Name: n2.Name, Namespace: NAMESPACE}, nc)).ToNot(HaveOccurred())
				Expect(nc.Spec.PhysicalFunctions).To(BeEmpty())

			})
		})

		When("single cc does match to single node", func() {
			It("cc.Spec should be propagated to matching nc", func() {
				n1 := createNode("n1")
				n2 := createNode("n2")

				createNodeInventory(n1.Name, []sriovv2.SriovAccelerator{
					{
						PCIAddress: "0000:18:00.1",
						DeviceID:   "known",
						VendorID:   "8086",
						VFs:        []sriovv2.VF{},
					},
				})

				createNodeInventory(n2.Name, []sriovv2.SriovAccelerator{
					{
						PCIAddress: "0000:20:00.1",
						DeviceID:   "unknown",
						VendorID:   "8086",
						VFs:        []sriovv2.VF{},
					},
				})

				pfc := sriovv2.PhysicalFunctionConfig{
					PFDriver: "pci-pf-stub",
					VFDriver: "vfio-pci",
					VFAmount: 3,
				}

				createAcceleratorConfig("cc", func(cc *sriovv2.SriovFecClusterConfig) {
					cc.Spec.AcceleratorSelector = sriovv2.AcceleratorSelector{
						DeviceID: "known",
					}
					cc.Spec.PhysicalFunction = pfc
				})

				reconcile("cc")

				nc := new(sriovv2.SriovFecNodeConfig)
				Expect(k8sClient.Get(context.TODO(), client.ObjectKey{Name: n1.Name, Namespace: NAMESPACE}, nc)).ToNot(HaveOccurred())
				Expect(nc.Spec.PhysicalFunctions).ToNot(BeEmpty())
				Expect(nc.Spec.PhysicalFunctions[0]).
					To(Equal(sriovv2.PhysicalFunctionConfigExt{
						PCIAddress:  "0000:18:00.1",
						PFDriver:    pfc.PFDriver,
						VFDriver:    pfc.VFDriver,
						VFAmount:    pfc.VFAmount,
						BBDevConfig: pfc.BBDevConfig,
					}))
				Expect(nc.Spec.DrainSkip).To(BeFalse())

				nc2 := new(sriovv2.SriovFecNodeConfig)
				Expect(k8sClient.Get(context.TODO(), client.ObjectKey{Name: n2.Name, Namespace: NAMESPACE}, nc2)).ToNot(HaveOccurred())
				Expect(nc2.Spec.PhysicalFunctions).To(BeEmpty())
				Expect(nc2.Spec.DrainSkip).To(BeFalse())

			})
		})

		When("single cc does match to multiple nodes", func() {
			It("cc.spec should be propagated to all matching nc", func() {
				n1 := createNode("n1")
				n2 := createNode("n2")

				createNodeInventory(n1.Name, []sriovv2.SriovAccelerator{
					{
						PCIAddress: "0000:18:00.1",
						DeviceID:   "known",
						VendorID:   "8086",
						VFs:        []sriovv2.VF{},
					},
				})

				createNodeInventory(n2.Name, []sriovv2.SriovAccelerator{
					{
						PCIAddress: "0000:20:00.1",
						DeviceID:   "unknown",
						VendorID:   "8086",
						VFs:        []sriovv2.VF{},
					},
				})

				pfc := sriovv2.PhysicalFunctionConfig{
					PFDriver: "pci-pf-stub",
					VFDriver: "vfio-pci",
					VFAmount: 3,
				}

				createAcceleratorConfig("cc", func(cc *sriovv2.SriovFecClusterConfig) {
					cc.Spec.AcceleratorSelector = sriovv2.AcceleratorSelector{
						VendorID: "8086",
					}
					cc.Spec.PhysicalFunction = pfc
				})

				reconcile("cc")

				nc1 := new(sriovv2.SriovFecNodeConfig)
				Expect(k8sClient.Get(context.TODO(), client.ObjectKey{Name: n1.Name, Namespace: NAMESPACE}, nc1)).ToNot(HaveOccurred())
				Expect(nc1.Spec.PhysicalFunctions).ToNot(BeEmpty())
				Expect(nc1.Spec.PhysicalFunctions[0]).
					To(Equal(sriovv2.PhysicalFunctionConfigExt{
						PCIAddress:  "0000:18:00.1",
						PFDriver:    pfc.PFDriver,
						VFDriver:    pfc.VFDriver,
						VFAmount:    pfc.VFAmount,
						BBDevConfig: pfc.BBDevConfig,
					}))

				nc2 := new(sriovv2.SriovFecNodeConfig)
				Expect(k8sClient.Get(context.TODO(), client.ObjectKey{Name: n2.Name, Namespace: NAMESPACE}, nc2)).ToNot(HaveOccurred())
				Expect(nc2.Spec.PhysicalFunctions).ToNot(BeEmpty())
				Expect(nc2.Spec.PhysicalFunctions[0]).
					To(Equal(sriovv2.PhysicalFunctionConfigExt{
						PCIAddress:  "0000:20:00.1",
						PFDriver:    pfc.PFDriver,
						VFDriver:    pfc.VFDriver,
						VFAmount:    pfc.VFAmount,
						BBDevConfig: pfc.BBDevConfig,
					}))

			})
		})

		When("two ccs does match to two accelerators on single node", func() {
			It("both ss.specs should be propagated to matching nc", func() {
				node := createNode("foobar")

				createNodeInventory(node.Name, []sriovv2.SriovAccelerator{
					{
						PCIAddress: "0000:14:00.1",
						DeviceID:   "id1",
						VFs:        []sriovv2.VF{},
						MaxVFs:     0,
					},
					{
						PCIAddress: "0000:15:00.1",
						DeviceID:   "id2",
						VFs:        []sriovv2.VF{},
						MaxVFs:     0,
					}},
				)

				_ = createAcceleratorConfig("cc1", func(cc *sriovv2.SriovFecClusterConfig) {
					cc.Spec.AcceleratorSelector = sriovv2.AcceleratorSelector{DeviceID: "id1"}
					cc.Spec.PhysicalFunction = sriovv2.PhysicalFunctionConfig{
						PFDriver: "foo-driver",
					}
				})
				_ = createAcceleratorConfig("cc2", func(cc *sriovv2.SriovFecClusterConfig) {
					cc.Spec.AcceleratorSelector = sriovv2.AcceleratorSelector{DeviceID: "id2"}
					cc.Spec.PhysicalFunction = sriovv2.PhysicalFunctionConfig{
						PFDriver: "bar-driver",
					}
				})

				reconciler := SriovFecClusterConfigReconciler{k8sClient, log, scheme.Scheme}

				_, err := reconciler.Reconcile(context.TODO(), createDummyReconcileRequest("cc1"))
				Expect(err).ToNot(HaveOccurred())

				//Check if node config was created out of cluster config
				nodeConfigs := new(sriovv2.SriovFecNodeConfigList)
				Expect(k8sClient.List(context.TODO(), nodeConfigs)).ToNot(HaveOccurred())
				Expect(len(nodeConfigs.Items)).To(Equal(1))
				Expect(nodeConfigs.Items[0].Name).To(Equal(node.Name))
				Expect(nodeConfigs.Items[0].Spec.PhysicalFunctions).To(HaveLen(2))
			})
		})

		When("two ccs does match to single accelerator on single node", func() {
			It("cc.spec with higher priority should be propagated to matching nc", func() {

				const (
					lowPriority  = 1
					highPriority = 100
				)

				n1 := createNode("n1")

				createNodeInventory(n1.Name, []sriovv2.SriovAccelerator{
					{
						PCIAddress: "0000:15:00.1",
						VendorID:   "testvendor",
						VFs:        []sriovv2.VF{},
					},
				})

				hpcc := createAcceleratorConfig("high-priority-cluster-config", func(cc *sriovv2.SriovFecClusterConfig) {
					cc.Spec.AcceleratorSelector = sriovv2.AcceleratorSelector{
						VendorID: "testvendor",
					}
					cc.Spec.PhysicalFunction = sriovv2.PhysicalFunctionConfig{
						PFDriver: "pfDriver",
						VFDriver: "vfDriver",
						VFAmount: 1,
					}
					cc.Spec.Priority = highPriority
				})

				_ = createAcceleratorConfig("low-priority-cluster-config", func(cc *sriovv2.SriovFecClusterConfig) {
					cc.Spec.AcceleratorSelector = sriovv2.AcceleratorSelector{
						PCIAddress: "0000:15:00.1",
					}
					cc.Spec.PhysicalFunction = sriovv2.PhysicalFunctionConfig{
						PFDriver: "secondPfDriver",
						VFDriver: "secondVfDriver",
						VFAmount: 2,
					}
					cc.Spec.Priority = lowPriority
				})

				_ = reconcile("high-priority-cluster-config")

				cl := new(sriovv2.SriovFecNodeConfigList)
				Expect(k8sClient.List(context.TODO(), cl)).ToNot(HaveOccurred())
				Expect(cl.Items).To(HaveLen(1))
				nc := cl.Items[0]
				Expect(nc.Spec.PhysicalFunctions).To(HaveLen(1))
				Expect(nc.Spec.PhysicalFunctions[0].VFAmount).Should(Equal(hpcc.Spec.PhysicalFunction.VFAmount))
				Expect(nc.Spec.PhysicalFunctions[0].VFDriver).Should(Equal(hpcc.Spec.PhysicalFunction.VFDriver))
				Expect(nc.Spec.PhysicalFunctions[0].PFDriver).Should(Equal(hpcc.Spec.PhysicalFunction.PFDriver))

			})

			Context("both of them have same priority", func() {
				It("only newer cc.spec should be propagated to matching nc", func() {

					n1 := createNode("n1")

					createNodeInventory(n1.Name, []sriovv2.SriovAccelerator{
						{
							PCIAddress: "0000:15:00.1",
							VendorID:   "testvendor",
							VFs:        []sriovv2.VF{},
						},
					})

					_ = createAcceleratorConfig("config1", func(cc *sriovv2.SriovFecClusterConfig) {
						cc.Spec.AcceleratorSelector = sriovv2.AcceleratorSelector{
							VendorID: "testvendor",
						}
						cc.Spec.PhysicalFunction = sriovv2.PhysicalFunctionConfig{
							PFDriver: "pfDriver",
							VFDriver: "vfDriver",
							VFAmount: 1,
						}
						cc.Spec.Priority = 1
					})

					//put some delay between one and another config creation
					time.Sleep(time.Second)

					newerCC := createAcceleratorConfig("config2", func(cc *sriovv2.SriovFecClusterConfig) {
						cc.Spec.AcceleratorSelector = sriovv2.AcceleratorSelector{
							PCIAddress: "0000:15:00.1",
						}
						cc.Spec.PhysicalFunction = sriovv2.PhysicalFunctionConfig{
							PFDriver: "secondPfDriver",
							VFDriver: "secondVfDriver",
							VFAmount: 2,
						}
						cc.Spec.Priority = 1
					})

					_ = reconcile("config2")

					cl := new(sriovv2.SriovFecNodeConfigList)
					Expect(k8sClient.List(context.TODO(), cl)).ToNot(HaveOccurred())
					Expect(cl.Items).To(HaveLen(1))
					nc := cl.Items[0]
					Expect(nc.Spec.PhysicalFunctions).To(HaveLen(1))
					Expect(nc.Spec.PhysicalFunctions[0].VFAmount).Should(Equal(newerCC.Spec.PhysicalFunction.VFAmount))
					Expect(nc.Spec.PhysicalFunctions[0].VFDriver).Should(Equal(newerCC.Spec.PhysicalFunction.VFDriver))
					Expect(nc.Spec.PhysicalFunctions[0].PFDriver).Should(Equal(newerCC.Spec.PhysicalFunction.PFDriver))

				})
			})

			Context("ccs have different priorities", func() {
				It("higher proprity spec should be propagated to matching nc", func() {

					n1 := createNode("n1")

					createNodeInventory(n1.Name, []sriovv2.SriovAccelerator{
						{
							PCIAddress: "0000:15:00.1",
							VendorID:   "testvendor",
							VFs:        []sriovv2.VF{},
						},
					})

					higherPriorityClusterConfig := createAcceleratorConfig("config2", func(cc *sriovv2.SriovFecClusterConfig) {
						cc.Spec.AcceleratorSelector = sriovv2.AcceleratorSelector{
							PCIAddress: "0000:15:00.1",
						}
						cc.Spec.PhysicalFunction = sriovv2.PhysicalFunctionConfig{
							PFDriver: "secondPfDriver",
							VFDriver: "secondVfDriver",
							VFAmount: 2,
						}
						cc.Spec.Priority = 2
					})

					_ = createAcceleratorConfig("config1", func(cc *sriovv2.SriovFecClusterConfig) {
						cc.Spec.AcceleratorSelector = sriovv2.AcceleratorSelector{
							VendorID: "testvendor",
						}
						cc.Spec.PhysicalFunction = sriovv2.PhysicalFunctionConfig{
							PFDriver: "pfDriver",
							VFDriver: "vfDriver",
							VFAmount: 1,
						}
						cc.Spec.Priority = 1
					})

					_ = reconcile("config1")

					cl := new(sriovv2.SriovFecNodeConfigList)
					Expect(k8sClient.List(context.TODO(), cl)).ToNot(HaveOccurred())
					Expect(cl.Items).To(HaveLen(1))
					nc := cl.Items[0]
					Expect(nc.Spec.PhysicalFunctions).To(HaveLen(1))
					Expect(nc.Spec.PhysicalFunctions[0].VFAmount).Should(Equal(higherPriorityClusterConfig.Spec.PhysicalFunction.VFAmount))
					Expect(nc.Spec.PhysicalFunctions[0].VFDriver).Should(Equal(higherPriorityClusterConfig.Spec.PhysicalFunction.VFDriver))
					Expect(nc.Spec.PhysicalFunctions[0].PFDriver).Should(Equal(higherPriorityClusterConfig.Spec.PhysicalFunction.PFDriver))

				})
			})

		})

		When("cc has no node selector", func() {
			It("cc.spec should be propagated to all nodes having matching accelerator", func() {
				n1 := createNode("n1")
				n2 := createNode("n2")

				createNodeInventory(n1.Name, []sriovv2.SriovAccelerator{
					{
						PCIAddress: "0000:15:00.1",
						VendorID:   "testvendor",
						VFs:        []sriovv2.VF{},
					},
				})

				createNodeInventory(n2.Name, []sriovv2.SriovAccelerator{
					{
						PCIAddress: "0000:15:00.2",
						VendorID:   "testvendor",
						VFs:        []sriovv2.VF{},
					},
				})

				createAcceleratorConfig("cc", func(cc *sriovv2.SriovFecClusterConfig) {
					cc.Spec.AcceleratorSelector = sriovv2.AcceleratorSelector{
						VendorID: "testvendor",
					}
					cc.Spec.PhysicalFunction = sriovv2.PhysicalFunctionConfig{
						PFDriver: "pfDriver",
						VFDriver: "vfDriver",
						VFAmount: 2,
					}
				})

				reconciler := SriovFecClusterConfigReconciler{k8sClient, log, scheme.Scheme}
				_, err := reconciler.Reconcile(context.TODO(), createDummyReconcileRequest("cc"))
				Expect(err).ToNot(HaveOccurred())

				nc := new(sriovv2.SriovFecNodeConfig)
				Expect(k8sClient.Get(context.TODO(), client.ObjectKey{Name: n1.Name, Namespace: NAMESPACE}, nc)).ToNot(HaveOccurred())
				Expect(nc.Spec.PhysicalFunctions).To(HaveLen(1))

				nc = new(sriovv2.SriovFecNodeConfig)
				Expect(k8sClient.Get(context.TODO(), client.ObjectKey{Name: n2.Name, Namespace: NAMESPACE}, nc)).ToNot(HaveOccurred())
				Expect(nc.Spec.PhysicalFunctions).To(HaveLen(1))
			})
		})

		When("updating existing cc", func() {
			Context("with nodeSelector which doesn't match to any existing node", func() {
				It("should not be reflected in any existing nc", func() {
					n1 := createNode("first-node", func(n *corev1.Node) {
						n.Labels["kubernetes.io/hostname"] = n.Name
					})

					createNodeInventory(n1.Name, []sriovv2.SriovAccelerator{
						{
							DeviceID:   n1.Name,
							PCIAddress: "0000:15:00.1",
							VFs:        []sriovv2.VF{},
						},
					})

					cc := createAcceleratorConfig("config", func(cc *sriovv2.SriovFecClusterConfig) {
						cc.Spec.NodeSelector["kubernetes.io/hostname"] = n1.Name
						cc.Spec.AcceleratorSelector = sriovv2.AcceleratorSelector{
							PCIAddress: "0000:15:00.1",
						}
					})

					reconciler := SriovFecClusterConfigReconciler{k8sClient, log, scheme.Scheme}
					_, err := reconciler.Reconcile(context.TODO(), createDummyReconcileRequest("config"))
					Expect(err).ToNot(HaveOccurred())

					// Check if node config was created out of cluster config
					nodeConfig := new(sriovv2.SriovFecNodeConfig)
					Expect(k8sClient.Get(context.TODO(), client.ObjectKey{Name: n1.Name, Namespace: NAMESPACE}, nodeConfig)).ToNot(HaveOccurred())
					Expect(nodeConfig.Spec.PhysicalFunctions).To(HaveLen(1))

					// switch nodes
					cc.Spec.NodeSelector["kubernetes.io/hostname"] = "noexisting-node"
					Expect(k8sClient.Update(context.TODO(), cc)).ToNot(HaveOccurred())

					_, err = reconciler.Reconcile(context.TODO(), createDummyReconcileRequest("config"))
					Expect(err).ToNot(HaveOccurred())

					nodeConfigList := new(sriovv2.SriovFecNodeConfigList)
					Expect(k8sClient.List(context.TODO(), nodeConfigList)).ToNot(HaveOccurred())
					Expect(nodeConfigList.Items).To(HaveLen(1))
					Expect(nodeConfigList.Items[0].Spec.PhysicalFunctions).Should(HaveLen(0))
				})
			})
			Context("with nodeSelector which match to another existing node", func() {
				It("should be reflected on another node's nc", func() {

					n1 := createNode("first-node", func(n *corev1.Node) {
						n.Labels["kubernetes.io/hostname"] = n.Name
					})
					n2 := createNode("second-node", func(n *corev1.Node) {
						n.Labels["kubernetes.io/hostname"] = n.Name
					})

					createNodeInventory(n1.Name, []sriovv2.SriovAccelerator{
						{
							DeviceID:   n1.Name,
							PCIAddress: "0000:15:00.1",
							VFs:        []sriovv2.VF{},
						},
					})

					createNodeInventory(n2.Name, []sriovv2.SriovAccelerator{
						{
							DeviceID:   n2.Name,
							PCIAddress: "0000:15:00.1",
							VFs:        []sriovv2.VF{},
						},
					})

					cc := createAcceleratorConfig("config", func(cc *sriovv2.SriovFecClusterConfig) {
						cc.Spec.NodeSelector["kubernetes.io/hostname"] = n1.Name
						cc.Spec.AcceleratorSelector = sriovv2.AcceleratorSelector{
							PCIAddress: "0000:15:00.1",
						}
					})

					reconciler := SriovFecClusterConfigReconciler{k8sClient, log, scheme.Scheme}
					_, err := reconciler.Reconcile(context.TODO(), createDummyReconcileRequest("config"))
					Expect(err).ToNot(HaveOccurred())

					// Check if node config was created out of cluster config
					nodeConfig := new(sriovv2.SriovFecNodeConfig)
					Expect(k8sClient.Get(context.TODO(), client.ObjectKey{Name: n1.Name, Namespace: NAMESPACE}, nodeConfig)).ToNot(HaveOccurred())
					Expect(nodeConfig.Spec.PhysicalFunctions).To(HaveLen(1))

					// switch nodes
					cc.Spec.NodeSelector["kubernetes.io/hostname"] = n2.Name
					Expect(k8sClient.Update(context.TODO(), cc)).ToNot(HaveOccurred())

					_, err = reconciler.Reconcile(context.TODO(), createDummyReconcileRequest(cc.Name))
					Expect(err).ToNot(HaveOccurred())

					nodeConfig = new(sriovv2.SriovFecNodeConfig)
					Expect(k8sClient.Get(context.TODO(), client.ObjectKey{Namespace: NAMESPACE, Name: n2.Name}, nodeConfig)).ToNot(HaveOccurred())
					Expect(nodeConfig.Spec.PhysicalFunctions).To(HaveLen(1))

					nodeConfig = new(sriovv2.SriovFecNodeConfig)
					Expect(k8sClient.Get(context.TODO(), client.ObjectKey{Namespace: NAMESPACE, Name: n1.Name}, nodeConfig)).ToNot(HaveOccurred())
					Expect(nodeConfig.Spec.PhysicalFunctions).To(BeEmpty())
				})
			})
		})

		When("drainSkip is specified on CC level", func() {
			It("should be rewritten to matching NC", func() {
				n1 := createNode("first-node", func(n *corev1.Node) {
					n.Labels["kubernetes.io/hostname"] = n.Name
				})

				createNodeInventory(n1.Name, []sriovv2.SriovAccelerator{
					{
						DeviceID:   n1.Name,
						PCIAddress: "0000:15:00.1",
						VFs:        []sriovv2.VF{},
					},
				})

				createAcceleratorConfig("config", func(cc *sriovv2.SriovFecClusterConfig) {
					cc.Spec.NodeSelector["kubernetes.io/hostname"] = n1.Name
					cc.Spec.AcceleratorSelector = sriovv2.AcceleratorSelector{
						PCIAddress: "0000:15:00.1",
					}
					cc.Spec.DrainSkip = true
				})

				reconciler := SriovFecClusterConfigReconciler{k8sClient, log, scheme.Scheme}
				_, err := reconciler.Reconcile(context.TODO(), createDummyReconcileRequest("config"))
				Expect(err).ToNot(HaveOccurred())

				nodeConfig := new(sriovv2.SriovFecNodeConfig)
				Expect(k8sClient.Get(context.TODO(), client.ObjectKey{Name: n1.Name, Namespace: NAMESPACE}, nodeConfig)).ToNot(HaveOccurred())
				Expect(nodeConfig.Spec.DrainSkip).To(BeTrue())
			})
		})

		When("cc has been created outside of sriov-fec operator namespace", func() {
			It("should not be reflected in any existing nc", func() {
				node := nodePrototype.DeepCopy()
				Expect(k8sClient.Create(context.TODO(), node)).ToNot(HaveOccurred())

				cc := clusterConfigPrototype.DeepCopy()
				cc.Namespace = v1.NamespaceSystem
				Expect(k8sClient.Create(context.TODO(), cc)).ToNot(HaveOccurred())

				reconciler := SriovFecClusterConfigReconciler{k8sClient, log, scheme.Scheme}
				_, err := reconciler.Reconcile(context.TODO(), createDummyReconcileRequest(clusterConfigPrototype.Name))
				Expect(err).ToNot(HaveOccurred())

				nodeConfigs := &sriovv2.SriovFecNodeConfigList{}
				Expect(k8sClient.List(context.TODO(), nodeConfigs)).ToNot(HaveOccurred())
				Expect(len(nodeConfigs.Items)).To(Equal(0))
			})
		})
	})

	var _ = Describe("Reconciler manager", func() {
		var _ = It("setup with invalid manager", func() {
			var m ctrl.Manager
			var reconciler SriovFecClusterConfigReconciler
			err := reconciler.SetupWithManager(m)
			Expect(err).To(HaveOccurred())
		})
	})

	var _ = Describe("API validators", func() {
		type kubectl interface {
			Run(args ...string) (stdout, stderr io.Reader, err error)
		}

		read := func(r io.Reader) string {
			s, e := ioutil.ReadAll(r)
			Expect(e).ToNot(HaveOccurred())
			return string(s)
		}

		var kctl kubectl
		BeforeEach(func() {
			fmt.Printf("Running initkubectl")
			kctl = testEnv.ControlPlane.KubeCtl()
		})

		_ = Context("Verifying Correct SriovFecClusterConfigs", func() {
			err := filepath.Walk("./testdata/clusterconfig/correct",
				func(path string, info os.FileInfo, err error) error {
					Expect(err).ToNot(HaveOccurred())
					if !info.IsDir() {
						It(filepath.Base(path), func() {
							_, errOut, e := kctl.Run("apply", "-f", path, "-n", "default")
							Expect(e).ToNot(HaveOccurred(), read(errOut))
						})
					}
					return nil
				},
			)

			Expect(err).ToNot(HaveOccurred())
		})
	})
})
