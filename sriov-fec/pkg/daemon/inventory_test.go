// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("SriovInventoryTest", func() {
	log := ctrl.Log.WithName("SriovDaemon-test")
	var _ = Context("GetSriovInventory", func() {
		var _ = It("will return error when config is nil ", func() {
			_, err := GetSriovInventory(log)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
