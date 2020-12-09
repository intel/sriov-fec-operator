// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package assets

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
)

var _ = Describe("Asset Tests", func() {

	log := klogr.New()

	var _ = Describe("Manager", func() {
		var _ = It("Run Manager with no assets (setKernel false)", func() {
			var err error
			log = klogr.New().WithName("N3000Assets-Test")

			manager := Manager{Client: k8sClient, Log: log}

			err = manager.LoadAndDeploy(context.TODO(), false)
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("Run Manager with no assets (setKernel true)", func() {
			var err error
			log = klogr.New().WithName("N3000Assets-Test")

			manager := Manager{Client: k8sClient, Log: log}

			err = manager.LoadAndDeploy(context.TODO(), true)
			Expect(err).To(HaveOccurred())
		})
		var _ = It("Run Manager (setKernel true)", func() {
			var err error
			log = klogr.New().WithName("N3000Assets-Test")

			assets := []Asset{
				{
					log:  log,
					Path: "/tmp/dummy.bin",
				},
			}

			manager := Manager{Client: k8sClient,
				Log:    log,
				Assets: assets}

			err = manager.LoadAndDeploy(context.TODO(), true)
			Expect(err).To(HaveOccurred())
		})
		var _ = It("Run Manager loadFromDir (setKernel false)", func() {
			var err error
			log = klogr.New().WithName("N3000Assets-Test")

			assets := []Asset{
				{
					log:  log,
					Path: "/tmp/",
				},
			}

			manager := Manager{Client: k8sClient,
				Log:    log,
				Assets: assets}

			err = manager.LoadAndDeploy(context.TODO(), false)
			Expect(err).To(HaveOccurred())
		})

		var _ = It("Run Manager loadFromFile (setKernel false, no retries)", func() {
			var err error
			log = klogr.New().WithName("N3000Assets-Test")

			assets := []Asset{
				{
					log:           log,
					Path:          fakeAssetFile,
					substitutions: map[string]string{"one": "two"},
					BlockingReadiness: ReadinessPollConfig{
						Retries: 1,
					},
				},
			}

			manager := Manager{Client: k8sClient,
				Log:    log,
				Assets: assets,
				Owner:  fakeOwner,
				Scheme: scheme.Scheme}

			err = manager.LoadAndDeploy(context.TODO(), false)
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("Run Manager loadFromFile (setKernel false)", func() {
			var err error
			log = klogr.New().WithName("N3000Assets-Test")

			assets := []Asset{
				{
					log:           log,
					Path:          fakeAssetFile,
					substitutions: map[string]string{"one": "two"},
				},
			}

			manager := Manager{Client: k8sClient,
				Log:    log,
				Assets: assets,
				Owner:  fakeOwner,
				Scheme: scheme.Scheme}

			err = manager.LoadAndDeploy(context.TODO(), false)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
