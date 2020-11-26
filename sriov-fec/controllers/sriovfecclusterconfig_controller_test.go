// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	sriovv1 "github.com/otcshare/openshift-operator/sriov-fec/api/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("SriovControllerTest", func() {
	var _ = Describe("Reconciler", func() {
		var (
			node           *corev1.Node
			clusterConfig  *sriovv1.SriovFecClusterConfig
			request        ctrl.Request
			reconciler     SriovFecClusterConfigReconciler
			log            = ctrl.Log.WithName("SriovController-test")
			doDeconf       = true
			removeCluster  = true
			nodeName       = "node-dummy"
			namespacedName = types.NamespacedName{
				Name:      DEFAULT_CLUSTER_CONFIG_NAME,
				Namespace: NAMESPACE,
			}
		)

		BeforeEach(func() {
			doDeconf = true
			removeCluster = true
			node = &corev1.Node{
				ObjectMeta: v1.ObjectMeta{
					Name: nodeName,
					Labels: map[string]string{
						"fpga.intel.com/intel-accelerator-present": "",
					},
				},
			}
			clusterConfig = &sriovv1.SriovFecClusterConfig{
				ObjectMeta: v1.ObjectMeta{
					Name:      DEFAULT_CLUSTER_CONFIG_NAME,
					Namespace: NAMESPACE,
				},
				Spec: sriovv1.SriovFecClusterConfigSpec{
					Nodes: []sriovv1.NodeConfig{
						{
							NodeName: nodeName,
							PhysicalFunctions: []sriovv1.PhysicalFunctionConfig{
								{
									PCIAddress: "a123:45:71.3",
									PFDriver:   "d",
									VFDriver:   "v",
									VFAmount:   5,
									BBDevConfig: sriovv1.BBDevConfig{
										N3000: &sriovv1.N3000BBDevConfig{
											NetworkType: "FPGA_LTE",
											PFMode:      false,
											FLRTimeOut:  10,
											Downlink: sriovv1.UplinkDownlink{
												Bandwidth:   3,
												LoadBalance: 3,
												Queues: sriovv1.UplinkDownlinkQueues{
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
											Uplink: sriovv1.UplinkDownlink{
												Bandwidth:   2,
												LoadBalance: 2,
												Queues: sriovv1.UplinkDownlinkQueues{
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
						},
					},
				},
			}
		})
		AfterEach(func() {
			var err error
			if doDeconf {
				err = k8sClient.Get(context.TODO(), namespacedName, clusterConfig)
				Expect(err).NotTo(HaveOccurred())
				clusterConfig.Spec = sriovv1.SriovFecClusterConfigSpec{
					Nodes: []sriovv1.NodeConfig{},
				}
				err = k8sClient.Update(context.TODO(), clusterConfig)
				Expect(err).NotTo(HaveOccurred())
				_, err = reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
			}

			if removeCluster {
				err = k8sClient.Delete(context.TODO(), clusterConfig)
				Expect(err).ToNot(HaveOccurred())
			}

			err = k8sClient.Delete(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
		})

		var _ = It("will create node config", func() {
			// envtest is empty, create fake node
			err := k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())

			// simulate creation of cluster config by the user
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			reconciler = SriovFecClusterConfigReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: NAMESPACE,
					Name:      DEFAULT_CLUSTER_CONFIG_NAME,
				},
			}

			_, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			//Check if node config was created out of cluster config
			nodeConfigs := &sriovv1.SriovFecNodeConfigList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(nodeConfigs.Items[0].ObjectMeta.Name).To(Equal(nodeName))
		})

		var _ = It("will update config to use another node", func() {

			var err error

			//define 2nd node
			node2 := &corev1.Node{
				ObjectMeta: v1.ObjectMeta{
					Name: "dummynode2",
					Labels: map[string]string{
						"fpga.intel.com/intel-accelerator-present": "",
					},
				},
			}

			err = k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Create(context.TODO(), node2)
			Expect(err).ToNot(HaveOccurred())
			nodes := &corev1.NodeList{}
			err = k8sClient.List(context.TODO(), nodes)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodes.Items)).To(Equal(2))

			// create on node dummy (1st)
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			reconciler = SriovFecClusterConfigReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: NAMESPACE,
					Name:      DEFAULT_CLUSTER_CONFIG_NAME,
				},
			}

			_, rec_err := reconciler.Reconcile(request)
			Expect(rec_err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs := &sriovv1.SriovFecNodeConfigList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(nodeConfigs.Items[0].ObjectMeta.Name).To(Equal(nodeName))

			// switch nodes
			err = k8sClient.Get(context.TODO(), namespacedName, clusterConfig)
			Expect(err).NotTo(HaveOccurred())
			clusterConfig.Spec.Nodes[0].NodeName = "dummynode2"

			err = k8sClient.Update(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())
			_, rec_err = reconciler.Reconcile(request)
			Expect(rec_err).ToNot(HaveOccurred())

			nodeConfigs = &sriovv1.SriovFecNodeConfigList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(nodeConfigs.Items[0].ObjectMeta.Name).To(Equal("dummynode2"))

			// cleanup
			err = k8sClient.Delete(context.TODO(), node2)
			Expect(err).NotTo(HaveOccurred())
		})

		var _ = It("will fail to update config to use another non-existing node", func() {

			var err error

			err = k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
			nodes := &corev1.NodeList{}
			err = k8sClient.List(context.TODO(), nodes)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodes.Items)).To(Equal(1))

			// create on node dummy (1st)
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			reconciler = SriovFecClusterConfigReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: NAMESPACE,
					Name:      DEFAULT_CLUSTER_CONFIG_NAME,
				},
			}

			_, rec_err := reconciler.Reconcile(request)
			Expect(rec_err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs := &sriovv1.SriovFecNodeConfigList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(nodeConfigs.Items[0].ObjectMeta.Name).To(Equal("node-dummy"))

			// switch nodes
			err = k8sClient.Get(context.TODO(), namespacedName, clusterConfig)
			Expect(err).NotTo(HaveOccurred())
			clusterConfig.Spec.Nodes[0].NodeName = "node-dummy2"

			err = k8sClient.Update(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())
			_, rec_err = reconciler.Reconcile(request)
			Expect(rec_err).ToNot(HaveOccurred())

			nodeConfigs = &sriovv1.SriovFecNodeConfigList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(0))
		})

		var _ = It("will not create a node because of namespace not found", func() {
			// envtest is empty, create fake node
			err := k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())

			// simulate creation of cluster config by the user
			clusterConfig.Name = "Invalid"
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).To(HaveOccurred())

			reconciler = SriovFecClusterConfigReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: NAMESPACE,
					Name:      DEFAULT_CLUSTER_CONFIG_NAME,
				},
			}

			_, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs := &sriovv1.SriovFecNodeConfigList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(0))

			doDeconf = false
			removeCluster = false
		})

		var _ = It("will not create a node because of wrong namespace", func() {

			// envtest is empty, create fake node
			err := k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())

			// simulate creation of cluster config by the user
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			reconciler = SriovFecClusterConfigReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "wrongNamespace",
					Name:      DEFAULT_CLUSTER_CONFIG_NAME,
				},
			}

			_, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs := &sriovv1.SriovFecNodeConfigList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(0))
		})

		var _ = It("will not create a node because of wrong cluster config name", func() {

			// envtest is empty, create fake node
			err := k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())

			// simulate creation of cluster config by the user
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			reconciler = SriovFecClusterConfigReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: NAMESPACE,
					Name:      "wrongName",
				},
			}

			_, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs := &sriovv1.SriovFecNodeConfigList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(0))
		})

		var _ = It("will 0 nodes", func() {

			// envtest is empty, create fake node
			err := k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())

			clusterConfig.Spec.Nodes = []sriovv1.NodeConfig{}
			// simulate creation of cluster config by the user
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			reconciler = SriovFecClusterConfigReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: NAMESPACE,
					Name:      DEFAULT_CLUSTER_CONFIG_NAME,
				},
			}

			nodeConfigs := &sriovv1.SriovFecNodeConfigList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(0))

			_, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs = &sriovv1.SriovFecNodeConfigList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("will update node config", func() {
			// envtest is empty, create fake node
			err := k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())

			// simulate creation of cluster config by the user
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			reconciler = SriovFecClusterConfigReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: NAMESPACE,
					Name:      DEFAULT_CLUSTER_CONFIG_NAME,
				},
			}

			_, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			//Check if node config was created out of cluster config
			nodeConfigs := &sriovv1.SriovFecNodeConfigList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(nodeConfigs.Items[0].ObjectMeta.Name).To(Equal(nodeName))

			// modify node data
			err = k8sClient.Get(context.TODO(), namespacedName, clusterConfig)
			Expect(err).NotTo(HaveOccurred())
			clusterConfig.Spec.Nodes[0].PhysicalFunctions[0].PFDriver = "test"
			err = k8sClient.Update(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())
			_, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
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
})
