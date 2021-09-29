// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package drainhelper

import (
	"context"
	"github.com/smart-edge-open/openshift-operator/common/pkg/utils"
	"os"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

var _ = Describe("DrainHelper Tests", func() {

	log := utils.NewLogger()
	var clientSet clientset.Clientset

	var _ = Describe("DrainHelper", func() {

		var _ = BeforeEach(func() {
			var err error

			err = os.Setenv("DRAIN_TIMEOUT_SECONDS", "5")
			Expect(err).ToNot(HaveOccurred())

			err = os.Setenv("LEASE_DURATION_SECONDS", "15")
			Expect(err).ToNot(HaveOccurred())

		})

		var _ = It("Create simple DrainHelper", func() {
			dh := NewDrainHelper(log, &clientSet, "node", "namespace")
			Expect(dh).ToNot(Equal(nil))
		})

		var _ = It("Create simple DrainHelper with invalid drain timeout", func() {
			var err error

			timeoutVal := 5
			timeoutValStr := "0x" + strconv.Itoa(timeoutVal)

			err = os.Setenv("DRAIN_TIMEOUT_SECONDS", timeoutValStr)
			Expect(err).ToNot(HaveOccurred())

			dh := NewDrainHelper(log, &clientSet, "node", "namespace")
			Expect(dh).ToNot(Equal(nil))
			Expect(dh.drainer.Timeout).ToNot(Equal(time.Duration(timeoutVal) * time.Second))
		})

		var _ = It("Create simple DrainHelper with invalid lease time duration", func() {
			var err error

			leaseVal := 5
			leaseValStr := "0x" + strconv.Itoa(leaseVal)

			err = os.Setenv("LEASE_DURATION_SECONDS", leaseValStr)
			Expect(err).ToNot(HaveOccurred())

			dh := NewDrainHelper(log, &clientSet, "node", "namespace")
			Expect(dh).ToNot(Equal(nil))
			Expect(dh.leaderElectionConfig.LeaseDuration).ToNot(Equal(time.Duration(leaseVal) * time.Second))
		})

		var _ = It("Create and run simple DrainHelper with lease time too short", func() {
			var err error

			clientConfig := &restclient.Config{}
			cset, err := clientset.NewForConfig(clientConfig)
			Expect(err).ToNot(HaveOccurred())

			dh := NewDrainHelper(log, cset, "node", "namespace")
			Expect(dh).ToNot(Equal(nil))

			err = dh.Run(func(c context.Context) bool { return true }, true)
			Expect(err).To(HaveOccurred())
		})

		var _ = It("Fail DrainHelper.cordonAndDrain because of no nodes", func() {
			var err error

			clientConfig := &restclient.Config{}
			cset, err := clientset.NewForConfig(clientConfig)
			Expect(err).ToNot(HaveOccurred())

			dh := NewDrainHelper(log, cset, "node", "namespace")
			Expect(dh).ToNot(Equal(nil))

			err = dh.cordonAndDrain(context.Background())
			Expect(err).To(HaveOccurred())
		})

		var _ = It("Fail DrainHelper.uncordon because of no nodes", func() {
			var err error

			clientConfig := &restclient.Config{}
			cset, err := clientset.NewForConfig(clientConfig)
			Expect(err).ToNot(HaveOccurred())

			dh := NewDrainHelper(log, cset, "node", "namespace")
			Expect(dh).ToNot(Equal(nil))

			err = dh.uncordon(context.Background())
			Expect(err).To(HaveOccurred())
		})

		var _ = It("Run logWriter", func() {
			var err error

			clientConfig := &restclient.Config{}
			cset, err := clientset.NewForConfig(clientConfig)
			Expect(err).ToNot(HaveOccurred())

			dh := NewDrainHelper(log, cset, "node", "namespace")
			Expect(dh).ToNot(Equal(nil))

			outString := "Out test"
			count, err := dh.drainer.Out.Write([]byte(outString))
			Expect(err).ToNot(HaveOccurred())
			Expect(count).To(Equal(len(outString)))

			erroutString := "ErrOut test"
			count, err = dh.drainer.ErrOut.Write([]byte(erroutString))
			Expect(err).ToNot(HaveOccurred())
			Expect(count).To(Equal(len(erroutString)))
		})

		var _ = It("Run OnPodDeletedOrEvicted", func() {
			var err error

			clientConfig := &restclient.Config{}
			cset, err := clientset.NewForConfig(clientConfig)
			Expect(err).ToNot(HaveOccurred())

			dh := NewDrainHelper(log, cset, "node", "namespace")
			Expect(dh).ToNot(Equal(nil))

			pod := corev1.Pod{}
			dh.drainer.OnPodDeletedOrEvicted(&pod, true)
		})

		var _ = It("Drain and cordon the node", func() {
			var err error

			// Create a Node
			node := &corev1.Node{
				ObjectMeta: v1.ObjectMeta{
					Name: "dummy",
				},
			}

			err = k8sClient.Create(context.Background(), node)
			Expect(err).ToNot(HaveOccurred())

			cset, err := clientset.NewForConfig(cfg)
			Expect(err).ToNot(HaveOccurred())

			dh := NewDrainHelper(log, cset, "dummy", "namespace")
			Expect(dh).ToNot(Equal(nil))

			err = dh.cordonAndDrain(context.Background())
			Expect(err).ToNot(HaveOccurred())

			// Cleanup
			err = k8sClient.Delete(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
		})

		var _ = It("Drain, cordon and uncordon the node", func() {
			var err error

			// Create a Node
			node := &corev1.Node{
				ObjectMeta: v1.ObjectMeta{
					Name: "dummy",
				},
			}

			err = k8sClient.Create(context.Background(), node)
			Expect(err).ToNot(HaveOccurred())

			cset, err := clientset.NewForConfig(cfg)
			Expect(err).ToNot(HaveOccurred())

			dh := NewDrainHelper(log, cset, "dummy", "namespace")
			Expect(dh).ToNot(Equal(nil))

			err = dh.cordonAndDrain(context.Background())
			Expect(err).ToNot(HaveOccurred())

			err = dh.uncordon(context.Background())
			Expect(err).ToNot(HaveOccurred())

			// Cleanup
			err = k8sClient.Delete(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
		})

		var _ = It("Create and run simple DrainHelper with drain true", func() {
			var err error
			// Create a Node
			node := &corev1.Node{
				ObjectMeta: v1.ObjectMeta{
					Name: "dummy",
				},
			}

			err = k8sClient.Create(context.Background(), node)
			Expect(err).ToNot(HaveOccurred())

			cset, err := clientset.NewForConfig(cfg)
			Expect(err).ToNot(HaveOccurred())

			err = os.Setenv("DRAIN_TIMEOUT_SECONDS", "5")
			Expect(err).ToNot(HaveOccurred())

			err = os.Setenv("LEASE_DURATION_SECONDS", "16")
			Expect(err).ToNot(HaveOccurred())

			dh := NewDrainHelper(log, cset, "dummy", "default")
			Expect(dh).ToNot(Equal(nil))

			err = dh.Run(func(c context.Context) bool { return true }, true)
			Expect(err).ToNot(HaveOccurred())

			// Cleanup
			err = k8sClient.Delete(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
		})

		var _ = It("Create and run simple DrainHelper with drain false", func() {
			var err error
			// Create a Node
			node := &corev1.Node{
				ObjectMeta: v1.ObjectMeta{
					Name: "dummy",
				},
			}

			err = k8sClient.Create(context.Background(), node)
			Expect(err).ToNot(HaveOccurred())

			cset, err := clientset.NewForConfig(cfg)
			Expect(err).ToNot(HaveOccurred())

			err = os.Setenv("DRAIN_TIMEOUT_SECONDS", "5")
			Expect(err).ToNot(HaveOccurred())

			err = os.Setenv("LEASE_DURATION_SECONDS", "16")
			Expect(err).ToNot(HaveOccurred())

			dh := NewDrainHelper(log, cset, "dummy", "default")
			Expect(dh).ToNot(Equal(nil))

			err = dh.Run(func(c context.Context) bool { return true }, false)
			Expect(err).ToNot(HaveOccurred())

			// Cleanup
			err = k8sClient.Delete(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
