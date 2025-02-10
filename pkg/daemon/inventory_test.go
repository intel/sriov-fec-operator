// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2025 Intel Corporation

package daemon

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("SriovInventoryTest", func() {
	log := logrus.New()
	var _ = Context("GetSriovInventory", func() {
		var _ = It("will return error when config is nil ", func() {
			_, err := GetSriovInventory(log)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
