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
	node := &corev1.Node{
		ObjectMeta: v1.ObjectMeta{
			Name: "dummy",
			Labels: map[string]string{
				"node-role.kubernetes.io/worker": "",
			},
		},
	}

	clusterConfig := &fpgav1.N3000Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name:      DEFAULT_N3000_CONFIG_NAME,
			Namespace: namespace,
		},
		Spec: fpgav1.N3000ClusterSpec{
			Nodes: []fpgav1.N3000ClusterNode{
				{
					NodeName: "dummy",
				},
			},
		},
	}

	var _ = Describe("Reconciler", func() {
		var _ = It("will create node config", func() {
			// envtest is empty, create fake node
			err := k8sClient.Create(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())

			// simulate creation of cluster config by the user
			err = k8sClient.Create(context.TODO(), clusterConfig)
			Expect(err).ToNot(HaveOccurred())

			log := klogr.New().WithName("N3000ClusterReconciler-Test")
			reconciler := N3000ClusterReconciler{
				Client: k8sClient,
				Scheme: scheme.Scheme,
				Log:    log,
			}

			request := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      DEFAULT_N3000_CONFIG_NAME,
				},
			}

			reconciler.Reconcile(request)

			// Check if node config was created out of cluster config
			nodeConfigs := &fpgav1.N3000NodeList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(nodeConfigs.Items[0].ObjectMeta.Name).To(Equal("n3000node-dummy"))
		})
	})
})
