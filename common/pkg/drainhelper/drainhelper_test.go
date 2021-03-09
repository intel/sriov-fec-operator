// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package drainhelper

import (
	"context"
	"os"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2/klogr"
)

var _ = Describe("DrainHelper Tests", func() {

	log := klogr.New()
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
			log = klogr.New().WithName("N3000DrainHelper-Test")

			dh := NewDrainHelper(log, &clientSet, "node", "namespace")
			Expect(dh).ToNot(Equal(nil))
		})

		var _ = It("Create simple DrainHelper with invalid drain timeout", func() {
			var err error
			log = klogr.New().WithName("N3000DrainHelper-Test")

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
			log = klogr.New().WithName("N3000DrainHelper-Test")

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
			log = klogr.New().WithName("N3000DrainHelper-Test")

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
			log = klogr.New().WithName("N3000DrainHelper-Test")

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
			log = klogr.New().WithName("N3000DrainHelper-Test")

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
			log = klogr.New().WithName("N3000DrainHelper-Test")

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
			log = klogr.New().WithName("N3000DrainHelper-Test")

			clientConfig := &restclient.Config{}
			cset, err := clientset.NewForConfig(clientConfig)
			Expect(err).ToNot(HaveOccurred())

			dh := NewDrainHelper(log, cset, "node", "namespace")
			Expect(dh).ToNot(Equal(nil))

			pod := corev1.Pod{}
			dh.drainer.OnPodDeletedOrEvicted(&pod, true)
		})
	})
})
