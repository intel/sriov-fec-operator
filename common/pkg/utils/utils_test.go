// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2021 Intel Corporation
package utils

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Main suite")
}

var _ = Describe("Utils", func() {
	var _ = Describe("LoadDiscoveryConfig", func() {
		var _ = It("will fail if the file does not exist", func() {
			cfg, err := LoadDiscoveryConfig("notExistingFile.json")
			Expect(err).To(HaveOccurred())
			Expect(cfg).To(Equal(AcceleratorDiscoveryConfig{}))
		})
		var _ = It("will fail if the file is not json", func() {
			cfg, err := LoadDiscoveryConfig("testdata/invalid.json")
			Expect(err).To(HaveOccurred())
			Expect(cfg).To(Equal(AcceleratorDiscoveryConfig{}))
		})
		var _ = It("will load the valid config successfully", func() {
			cfg, err := LoadDiscoveryConfig("testdata/valid.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).To(Equal(AcceleratorDiscoveryConfig{
				VendorID:  map[string]string{"0000": "test", "0001": "test1"},
				Class:     "00",
				SubClass:  "00",
				Devices:   map[string]string{"test": "test"},
				NodeLabel: "LABEL",
			}))
		})
	})
})
