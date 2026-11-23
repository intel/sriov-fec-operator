// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2026 Intel Corporation

package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"

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

	Describe("waitForPfBBConfigExit", func() {
		var origTimeout, origInterval time.Duration

		BeforeEach(func() {
			origTimeout, origInterval = pfBBConfigStopTimeout, pfBBConfigStopPollInterval
			pfBBConfigStopTimeout = 300 * time.Millisecond
			pfBBConfigStopPollInterval = 10 * time.Millisecond
		})

		AfterEach(func() {
			pfBBConfigStopTimeout, pfBBConfigStopPollInterval = origTimeout, origInterval
		})

		It("returns once the process is gone", func() {
			outputs := []string{"1", "1", "0"}
			runPgrepCount = func(*logrus.Logger, string) (string, error) {
				out := outputs[0]
				if len(outputs) > 1 {
					outputs = outputs[1:]
				}
				return out, nil
			}
			p := &pfBBConfigController{log: logrus.New()}
			Expect(p.waitForPfBBConfigExit(pci)).To(Succeed())
		})

		It("fails if the process does not exit in time", func() {
			pgrepReturns("1", nil)
			p := &pfBBConfigController{log: logrus.New()}
			Expect(p.waitForPfBBConfigExit(pci)).To(MatchError(ContainSubstring("has not exited")))
		})

		It("fails if the state cannot be determined", func() {
			pgrepReturns("", errors.New("boom"))
			p := &pfBBConfigController{log: logrus.New()}
			Expect(p.waitForPfBBConfigExit(pci)).To(MatchError(ContainSubstring("state: unknown")))
		})
	})
})

var _ = Describe("pending sysfs write markers", func() {
	const pf = "0000:1d:00.0"

	var (
		tmp                      string
		origWorkdir, origDevices string
		origWriteTimeout         time.Duration
		origTracker              *sysfsWriteTracker
	)

	BeforeEach(func() {
		var err error
		tmp, err = os.MkdirTemp("", "sysfs-markers")
		Expect(err).ToNot(HaveOccurred())
		origWorkdir, origDevices, origWriteTimeout, origTracker = workdir, sysBusPciDevices, sysfsWriteTimeout, sysfsWrites
		workdir = filepath.Join(tmp, "work")
		sysBusPciDevices = filepath.Join(tmp, "devices")
		sysfsWriteTimeout = 200 * time.Millisecond
		sysfsWrites = newSysfsWriteTracker()
	})

	AfterEach(func() {
		workdir, sysBusPciDevices, sysfsWriteTimeout, sysfsWrites = origWorkdir, origDevices, origWriteTimeout, origTracker
		Expect(os.RemoveAll(tmp)).To(Succeed())
	})

	markers := func() []string {
		entries, _ := os.ReadDir(sysfsWriteMarkerDir())
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return names
	}

	It("keeps a marker while a write is pending and removes it once the write completes", func() {
		path := filepath.Join(sysBusPciDevices, pf, "reset")
		Expect(os.MkdirAll(filepath.Dir(path), 0755)).To(Succeed())
		Expect(syscall.Mkfifo(path, 0644)).To(Succeed())

		var timeoutErr *SysfsWriteTimeoutError
		Expect(errors.As(writeSysfs(pf, path, "1"), &timeoutErr)).To(BeTrue())
		Expect(markers()).To(HaveLen(1))

		f, err := os.OpenFile(path, os.O_RDONLY, 0)
		Expect(err).ToNot(HaveOccurred())
		_, _ = f.Read(make([]byte, 4))
		Expect(f.Close()).To(Succeed())

		Eventually(markers).Should(BeEmpty())
	})

	It("loads markers left by a previous instance, removes them and remembers the PF", func() {
		createSysfsWriteMarker(pendingSysfsWrite{pf: pf, path: "/sys/bus/pci/drivers/vfio-pci/unbind", data: "0000:1d:00.1", since: time.Now()})
		Expect(os.WriteFile(filepath.Join(sysfsWriteMarkerDir(), "write-bad.json"), []byte("{"), 0600)).To(Succeed())
		Expect(markers()).To(HaveLen(2))

		set := &staleSysfsWriteSet{pfs: map[string][]sysfsWriteMarker{}}
		loadStaleSysfsWriteMarkersFrom(sysfsWriteMarkerDir(), set, logrus.New())

		Expect(markers()).To(BeEmpty())
		Expect(set.has(pf)).To(BeTrue())
		Expect(set.has("0000:3b:00.0")).To(BeFalse())

		set.clear(pf)
		Expect(set.has(pf)).To(BeFalse())
	})

	It("does nothing when there is no marker directory", func() {
		set := &staleSysfsWriteSet{pfs: map[string][]sysfsWriteMarker{}}
		loadStaleSysfsWriteMarkersFrom(sysfsWriteMarkerDir(), set, logrus.New())
		Expect(set.pfs).To(BeEmpty())
	})
})
