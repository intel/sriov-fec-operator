// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2026 Intel Corporation

package daemon

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("pf_bb_config process state", func() {
	const pci = "0000:f7:00.0"

	var origPgrep func(*logrus.Logger, string) (string, error)

	BeforeEach(func() {
		origPgrep = runPgrepCount
	})

	AfterEach(func() {
		runPgrepCount = origPgrep
	})

	pgrepReturns := func(out string, err error) {
		runPgrepCount = func(*logrus.Logger, string) (string, error) { return out, err }
	}

	It("is dead when no process matches", func() {
		pgrepReturns("0\n", nil)
		Expect(pfBbConfigProcState(logrus.New(), pci)).To(Equal(procDead))
		Expect(pfBbConfigProcIsDead(logrus.New(), pci)).To(BeTrue())
	})

	It("is alive when processes match, including ten or more", func() {
		pgrepReturns("1\n", nil)
		Expect(pfBbConfigProcState(logrus.New(), pci)).To(Equal(procAlive))
		pgrepReturns("10\n", nil)
		Expect(pfBbConfigProcState(logrus.New(), pci)).To(Equal(procAlive))
	})

	It("is unknown, not dead, when pgrep fails", func() {
		pgrepReturns("", errors.New("exec: \"pgrep\": executable file not found in $PATH"))
		Expect(pfBbConfigProcState(logrus.New(), pci)).To(Equal(procUnknown))
		Expect(pfBbConfigProcIsDead(logrus.New(), pci)).To(BeFalse())
	})

	It("is unknown, not dead, when pgrep prints something unexpected", func() {
		pgrepReturns("", nil)
		Expect(pfBbConfigProcIsDead(logrus.New(), pci)).To(BeFalse())
		pgrepReturns("garbage", nil)
		Expect(pfBbConfigProcIsDead(logrus.New(), pci)).To(BeFalse())
	})

})
