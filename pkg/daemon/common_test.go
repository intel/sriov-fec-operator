// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"github.com/intel/sriov-fec-operator/pkg/common/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("common", func() {
	log = utils.NewLogger()
	var _ = Context("execCmd", func() {
		var _ = It("will return error when args is empty ", func() {
			_, err := execCmd([]string{}, log)
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will return error when exec doesn't exist ", func() {
			_, err := execCmd([]string{"dummyExecFile"}, log)
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will call exec ", func() {
			_, err := execCmd([]string{"ls"}, log)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
