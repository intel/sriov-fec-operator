// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package controllers

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fpgav1 "github.com/otcshare/openshift-operator/N3000/api/v1"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("ExampleTest", func() {

	var node *corev1.Node
	var clusterConfig *fpgav1.N3000Cluster
	var request ctrl.Request
	var reconciler N3000ClusterReconciler
	log := klogr.New()
	doDeconf := true
	removeCluster := true
	namespacedName := types.NamespacedName{
		Name:      DEFAULT_N3000_CONFIG_NAME,
		Namespace: namespace,
	}

	BeforeEach(func() {

		doDeconf = true

		removeCluster = true

		node = &corev1.Node{
			ObjectMeta: v1.ObjectMeta{
				Name: "dummy",
				Labels: map[string]string{
					"fpga.intel.com/intel-accelerator-present": "",
				},
			},
		}

		clusterConfig = &fpgav1.N3000Cluster{
			ObjectMeta: v1.ObjectMeta{
				Name:      DEFAULT_N3000_CONFIG_NAME,
				Namespace: namespace,
			},
			Spec: fpgav1.N3000ClusterSpec{
				Nodes: []fpgav1.N3000ClusterNode{
					{
						NodeName: "dummy",
						Fortville: fpgav1.N3000Fortville{
							FirmwareURL: "http://exampleurl.com",
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
			clusterConfig.Spec = fpgav1.N3000ClusterSpec{
				Nodes: []fpgav1.N3000ClusterNode{},
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

	var _ = Describe("Reconciler", func() {
		var _ = It("will create node config", func() {

			// envtest is empty, create fake node
			err := k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())

			// simulate creation of cluster config by the user
			clusterConfig.Spec.Nodes[0].Fortville.FirmwareURL = "/tmp/dummy.bin"
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			log = klogr.New().WithName("N3000ClusterReconciler-Test")
			reconciler = N3000ClusterReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      DEFAULT_N3000_CONFIG_NAME,
				},
			}

			_, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs := &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(nodeConfigs.Items[0].ObjectMeta.Name).To(Equal("n3000node-dummy"))
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
			clusterConfig.Spec.Nodes[0].Fortville.FirmwareURL = "/tmp/dummy.bin"
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			log = klogr.New().WithName("N3000ClusterReconciler-Test")
			reconciler = N3000ClusterReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      DEFAULT_N3000_CONFIG_NAME,
				},
			}

			_, rec_err := reconciler.Reconcile(request)
			Expect(rec_err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs := &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(nodeConfigs.Items[0].ObjectMeta.Name).To(Equal("n3000node-dummy"))

			// switch nodes
			err = k8sClient.Get(context.TODO(), namespacedName, clusterConfig)
			Expect(err).NotTo(HaveOccurred())
			clusterConfig.Spec = fpgav1.N3000ClusterSpec{
				Nodes: []fpgav1.N3000ClusterNode{
					{
						NodeName: "dummynode2",
					},
				},
			}
			clusterConfig.Spec.Nodes[0].Fortville.FirmwareURL = "/tmp/dummynode2.bin"
			err = k8sClient.Update(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())
			_, rec_err = reconciler.Reconcile(request)
			Expect(rec_err).ToNot(HaveOccurred())

			nodeConfigs = &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(nodeConfigs.Items[0].ObjectMeta.Name).To(Equal("n3000node-dummynode2"))

			// cleanup
			err = k8sClient.Delete(context.TODO(), node2)
			Expect(err).NotTo(HaveOccurred())
		})

		var _ = It("will update config to use another URL", func() {

			var err error

			err = k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
			nodes := &corev1.NodeList{}
			err = k8sClient.List(context.TODO(), nodes)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodes.Items)).To(Equal(1))

			clusterConfig.Spec.Nodes[0].Fortville.FirmwareURL = "/tmp/dummy.bin"
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			log = klogr.New().WithName("N3000ClusterReconciler-Test")
			reconciler = N3000ClusterReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      DEFAULT_N3000_CONFIG_NAME,
				},
			}

			_, rec_err := reconciler.Reconcile(request)
			Expect(rec_err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs := &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(nodeConfigs.Items[0].ObjectMeta.Name).To(Equal("n3000node-dummy"))

			err = k8sClient.Get(context.TODO(), namespacedName, clusterConfig)
			Expect(err).NotTo(HaveOccurred())
			new_url := "https://new-url.com"
			clusterConfig.Spec.Nodes[0].Fortville.FirmwareURL = new_url
			err = k8sClient.Update(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())
			_, rec_err = reconciler.Reconcile(request)
			Expect(rec_err).ToNot(HaveOccurred())

			nodeConfigs = &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(nodeConfigs.Items[0].Spec.Fortville.FirmwareURL).To(Equal(new_url))

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
			clusterConfig.Spec.Nodes[0].Fortville.FirmwareURL = "/tmp/dummy.bin"
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			log = klogr.New().WithName("N3000ClusterReconciler-Test")
			reconciler = N3000ClusterReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      DEFAULT_N3000_CONFIG_NAME,
				},
			}

			_, rec_err := reconciler.Reconcile(request)
			Expect(rec_err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs := &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(nodeConfigs.Items[0].ObjectMeta.Name).To(Equal("n3000node-dummy"))

			// switch nodes
			err = k8sClient.Get(context.TODO(), namespacedName, clusterConfig)
			Expect(err).NotTo(HaveOccurred())
			clusterConfig.Spec = fpgav1.N3000ClusterSpec{
				Nodes: []fpgav1.N3000ClusterNode{
					{
						NodeName: "dummynode2",
					},
				},
			}
			clusterConfig.Spec.Nodes[0].Fortville.FirmwareURL = "/tmp/dummynode2.bin"
			err = k8sClient.Update(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())
			_, rec_err = reconciler.Reconcile(request)
			Expect(rec_err).ToNot(HaveOccurred())

			nodeConfigs = &fpgav1.N3000NodeList{}
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

			log = klogr.New().WithName("N3000ClusterReconciler-Test")
			reconciler = N3000ClusterReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      DEFAULT_N3000_CONFIG_NAME,
				},
			}

			_, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs := &fpgav1.N3000NodeList{}
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

			log = klogr.New().WithName("N3000ClusterReconciler-Test")
			reconciler = N3000ClusterReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "wrongNamespace",
					Name:      DEFAULT_N3000_CONFIG_NAME,
				},
			}

			_, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs := &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(0))
		})

		var _ = It("will not create a node - url ok", func() {

			// envtest is empty, create fake node
			err := k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())

			clusterConfig.Spec.Nodes[0].Fortville.FirmwareURL = "/tmp/dummy.bin"
			// simulate creation of cluster config by the user
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			log = klogr.New().WithName("N3000ClusterReconciler-Test")
			reconciler = N3000ClusterReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      DEFAULT_N3000_CONFIG_NAME,
				},
			}

			_, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs := &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(nodeConfigs.Items[0].ObjectMeta.Name).To(Equal("n3000node-dummy"))
		})

		var _ = It("will 0 nodes", func() {

			// envtest is empty, create fake node
			err := k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())

			clusterConfig.Spec.Nodes = []fpgav1.N3000ClusterNode{}
			// simulate creation of cluster config by the user
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			log = klogr.New().WithName("N3000ClusterReconciler-Test")
			reconciler = N3000ClusterReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      DEFAULT_N3000_CONFIG_NAME,
				},
			}

			nodeConfigs := &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(0))

			_, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs = &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
		})

		var _ = It("will leave 1st node of 2", func() {

			// envtest is empty, create fake node
			err := k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())

			clusterConfig.Spec.Nodes = []fpgav1.N3000ClusterNode{
				{
					NodeName: "dummy",
				},
				{
					NodeName: "dummy2",
				},
			}
			clusterConfig.Spec.Nodes[0].Fortville.FirmwareURL = "/tmp/dummy.bin"
			clusterConfig.Spec.Nodes[1].Fortville.FirmwareURL = "/tmp/dummy2.bin"

			// simulate creation of cluster config by the user
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			log = klogr.New().WithName("N3000ClusterReconciler-Test")
			reconciler = N3000ClusterReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      DEFAULT_N3000_CONFIG_NAME,
				},
			}

			nodeConfigs := &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(0))

			_, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs = &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(nodeConfigs.Items[0].ObjectMeta.Name).To(Equal("n3000node-dummy"))
		})

		var _ = It("will leave 2nd node of 2", func() {

			node.ObjectMeta.Name = "dummy2"
			// envtest is empty, create fake node
			err := k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())

			clusterConfig.Spec.Nodes = []fpgav1.N3000ClusterNode{
				{
					NodeName: "dummy",
				},
				{
					NodeName: "dummy2",
				},
			}
			clusterConfig.Spec.Nodes[0].Fortville.FirmwareURL = "/tmp/dummy.bin"
			clusterConfig.Spec.Nodes[1].Fortville.FirmwareURL = "/tmp/dummy2.bin"

			// simulate creation of cluster config by the user
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			log = klogr.New().WithName("N3000ClusterReconciler-Test")
			reconciler = N3000ClusterReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      DEFAULT_N3000_CONFIG_NAME,
				},
			}

			nodeConfigs := &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(0))

			_, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs = &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(nodeConfigs.Items[0].ObjectMeta.Name).To(Equal("n3000node-dummy2"))
		})

		var _ = It("will leave none of 2 nodes", func() {

			node.ObjectMeta.Name = "dummy3"
			// envtest is empty, create fake node
			err := k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())

			clusterConfig.Spec.Nodes = []fpgav1.N3000ClusterNode{
				{
					NodeName: "dummy",
				},
				{
					NodeName: "dummy2",
				},
			}
			clusterConfig.Spec.Nodes[0].Fortville.FirmwareURL = "/tmp/dummy.bin"
			clusterConfig.Spec.Nodes[0].Fortville.FirmwareURL = "/tmp/dummy2.bin"

			// simulate creation of cluster config by the user
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			log = klogr.New().WithName("N3000ClusterReconciler-Test")
			reconciler = N3000ClusterReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      DEFAULT_N3000_CONFIG_NAME,
				},
			}

			nodeConfigs := &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(0))

			_, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs = &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(0))

			doDeconf = false
		})

		var _ = It("will leave no nodes", func() {

			node.ObjectMeta.Name = "dummy3"
			// envtest is empty, create fake node
			err := k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())

			// simulate creation of cluster config by the user
			clusterConfig.Spec.Nodes[0].Fortville.FirmwareURL = "/tmp/dummy.bin"
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			log = klogr.New().WithName("N3000ClusterReconciler-Test")
			reconciler = N3000ClusterReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request = ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      DEFAULT_N3000_CONFIG_NAME,
				},
			}

			nodeConfigs := &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(0))

			_, err = reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())

			// Check if node config was created out of cluster config
			nodeConfigs = &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(0))
			doDeconf = false
		})
	})

	var _ = Describe("Reconciler manager", func() {
		var _ = It("setup with invalid manager", func() {
			var m ctrl.Manager

			err := reconciler.SetupWithManager(m)
			Expect(err).To(HaveOccurred())

			err = k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())

			doDeconf = false
			removeCluster = false
		})
	})

})
