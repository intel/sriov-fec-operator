// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2026 Intel Corporation

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

var _ = Describe("redactArgs", func() {
	It("hides the VFIO token passed to pf_bb_config", func() {
		args := []string{pfConfigAppFilepath, "VRB2", "-c", "/tmp/x.ini", "-p", "0000:07:00.0", "-v", "02bddbbf-bbb0-4d79-886b-91bad3fbb510", "-f", "/x.bin"}
		got := redactArgs(args)
		Expect(got).To(Equal([]string{pfConfigAppFilepath, "VRB2", "-c", "/tmp/x.ini", "-p", "0000:07:00.0", "-v", "<redacted>", "-f", "/x.bin"}))
		Expect(args[7]).To(Equal("02bddbbf-bbb0-4d79-886b-91bad3fbb510"), "the original args must not be modified")
	})

	It("leaves other commands alone, e.g. setpci -v", func() {
		args := []string{"setpci", "-v", "-s", "0000:b8:00.0", "COMMAND=06:06"}
		Expect(redactArgs(args)).To(Equal(args))
	})
})
