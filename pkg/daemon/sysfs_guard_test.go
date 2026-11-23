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

var _ = Describe("sysfs guard", func() {
	const (
		pf    = "0000:1d:00.0"
		vf    = "0000:1d:00.1"
		other = "0000:3b:00.0"
	)

	var (
		tmp                               string
		origDevices, origProc, origVfio   string
		origWriteTimeout, origReadTimeout time.Duration
		origGetVFList                     func(string) ([]string, error)
		origTracker                       *sysfsWriteTracker
	)

	// mkfifo creates a FIFO; writing to it blocks until a reader opens it,
	// like a sysfs write blocked on a device lock.
	mkfifo := func(path string) {
		Expect(os.MkdirAll(filepath.Dir(path), 0755)).To(Succeed())
		Expect(syscall.Mkfifo(path, 0644)).To(Succeed())
	}

	// release unblocks a write pending on a FIFO.
	release := func(path string) {
		f, err := os.OpenFile(path, os.O_RDONLY, 0)
		Expect(err).ToNot(HaveOccurred())
		buf := make([]byte, 16)
		_, _ = f.Read(buf)
		Expect(f.Close()).To(Succeed())
	}

	makeVF := func() {
		Expect(os.MkdirAll(filepath.Join(tmp, "devices", vf), 0755)).To(Succeed())
		Expect(os.Symlink("../"+pf, filepath.Join(tmp, "devices", vf, "physfn"))).To(Succeed())
	}

	BeforeEach(func() {
		var err error
		tmp, err = os.MkdirTemp("", "sysfs-guard")
		Expect(err).ToNot(HaveOccurred())

		origDevices, origProc, origVfio = sysBusPciDevices, procRoot, devVfioPath
		origWriteTimeout, origReadTimeout = sysfsWriteTimeout, sysfsReadTimeout
		origGetVFList = getVFList
		origTracker = sysfsWrites

		sysBusPciDevices = filepath.Join(tmp, "devices")
		procRoot = filepath.Join(tmp, "proc")
		devVfioPath = filepath.Join(tmp, "vfio")
		sysfsWriteTimeout = 200 * time.Millisecond
		sysfsReadTimeout = 200 * time.Millisecond
		sysfsWrites = newSysfsWriteTracker()

		Expect(os.MkdirAll(filepath.Join(sysBusPciDevices, pf), 0755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(sysBusPciDevices, other), 0755)).To(Succeed())
	})

	AfterEach(func() {
		sysBusPciDevices, procRoot, devVfioPath = origDevices, origProc, origVfio
		sysfsWriteTimeout, sysfsReadTimeout = origWriteTimeout, origReadTimeout
		getVFList = origGetVFList
		sysfsWrites = origTracker
		Expect(os.RemoveAll(tmp)).To(Succeed())
	})

	Describe("writeSysfs", func() {
		It("writes to the attribute", func() {
			path := filepath.Join(sysBusPciDevices, pf, "reset")
			Expect(os.WriteFile(path, nil, 0644)).To(Succeed())
			Expect(writeSysfs(pf, path, "1")).To(Succeed())
			Expect(os.ReadFile(path)).To(Equal([]byte("1")))
			Expect(sysfsWrites.blocking(pf)).To(BeNil())
		})

		It("times out and refuses further writes on the same PF until the pending write completes", func() {
			makeVF()
			unbind := filepath.Join(tmp, "drivers", "vfio-pci", "unbind")
			mkfifo(unbind)

			var timeoutErr *SysfsWriteTimeoutError
			Expect(errors.As(writeSysfs(vf, unbind, vf), &timeoutErr)).To(BeTrue())

			// the VF unbind is keyed by its PF
			numvfs := filepath.Join(sysBusPciDevices, pf, vfNumFileDefault)
			var blockedErr *SysfsWriteBlockedError
			Expect(errors.As(writeSysfs(pf, numvfs, "0"), &blockedErr)).To(BeTrue())
			Expect(blockedErr.Pending.path).To(Equal(unbind))

			// an unbind is not node-wide: another PF can still be written
			otherReset := filepath.Join(sysBusPciDevices, other, "reset")
			Expect(os.WriteFile(otherReset, nil, 0644)).To(Succeed())
			Expect(writeSysfs(other, otherReset, "1")).To(Succeed())

			release(unbind)
			Eventually(func() *pendingSysfsWrite { return sysfsWrites.blocking(pf) }).Should(BeNil())
		})

		It("treats a pending sriov_numvfs write as node-wide", func() {
			numvfs := filepath.Join(sysBusPciDevices, pf, vfNumFileDefault)
			Expect(os.Remove(filepath.Join(sysBusPciDevices, pf))).To(Succeed())
			mkfifo(numvfs)

			var timeoutErr *SysfsWriteTimeoutError
			Expect(errors.As(writeSysfs(pf, numvfs, "0"), &timeoutErr)).To(BeTrue())

			otherNumvfs := filepath.Join(sysBusPciDevices, other, vfNumFileDefault)
			var blockedErr *SysfsWriteBlockedError
			Expect(errors.As(writeSysfs(other, otherNumvfs, "0"), &blockedErr)).To(BeTrue())

			release(numvfs)
			Eventually(func() *pendingSysfsWrite { return sysfsWrites.blocking(other) }).Should(BeNil())
		})
	})

	Describe("readVFconfigured", func() {
		It("returns the number of VFs", func() {
			Expect(os.WriteFile(filepath.Join(sysBusPciDevices, pf, vfNumFileDefault), []byte("8\n"), 0644)).To(Succeed())
			Expect(readVFconfigured(pf)).To(Equal(8))
		})

		It("times out instead of hanging on a blocked attribute", func() {
			path := filepath.Join(sysBusPciDevices, pf, vfNumFileDefault)
			Expect(os.Remove(filepath.Join(sysBusPciDevices, pf))).To(Succeed())
			mkfifo(path)
			_, err := readVFconfigured(pf)
			Expect(err).To(MatchError(ContainSubstring("did not complete")))

			// unblock the leaked reader
			f, err := os.OpenFile(path, os.O_WRONLY, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(f.Close()).To(Succeed())
		})
	})

	Describe("findVfioUsers and ensurePFCanBeReconfigured", func() {
		const group = "42"

		BeforeEach(func() {
			makeVF()
			Expect(os.Symlink("../../kernel/iommu_groups/"+group, filepath.Join(sysBusPciDevices, vf, "iommu_group"))).To(Succeed())
			// stand-in for the vfio group character device
			Expect(os.MkdirAll(devVfioPath, 0755)).To(Succeed())
			Expect(os.Symlink("/dev/null", filepath.Join(devVfioPath, group))).To(Succeed())
			Expect(os.MkdirAll(procRoot, 0755)).To(Succeed())
			getVFList = func(string) ([]string, error) { return []string{vf}, nil }
		})

		addProcess := func(pid, comm string, fdTargets ...string) {
			fdDir := filepath.Join(procRoot, pid, "fd")
			Expect(os.MkdirAll(fdDir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(procRoot, pid, "comm"), []byte(comm+"\n"), 0644)).To(Succeed())
			for i, t := range fdTargets {
				Expect(os.Symlink(t, filepath.Join(fdDir, string(rune('3'+i))))).To(Succeed())
			}
		}

		nc := func() *NodeConfigurator {
			return &NodeConfigurator{Log: logrus.New()}
		}

		It("finds a process holding the VF's vfio group", func() {
			addProcess("1234", "dpdk-app", "/etc/hostname", filepath.Join(devVfioPath, group))
			addProcess("99", "other", "/etc/hostname")

			users, err := findVfioUsers([]string{vf})
			Expect(err).ToNot(HaveOccurred())
			Expect(users).To(Equal(map[string][]string{vf: {"1234 (dpdk-app)"}}))

			err = nc().ensurePFCanBeReconfigured(pf)
			var inUse *DeviceInUseError
			Expect(errors.As(err, &inUse)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("1234 (dpdk-app)"))
		})

		It("ignores pf_bb_config, which is stopped as part of the reconfiguration", func() {
			addProcess("77", "pf_bb_config", filepath.Join(devVfioPath, group))
			Expect(nc().ensurePFCanBeReconfigured(pf)).To(Succeed())
		})

		It("allows reconfiguration when no process holds the VFs", func() {
			addProcess("99", "other", "/etc/hostname")
			Expect(nc().ensurePFCanBeReconfigured(pf)).To(Succeed())
		})

		It("skips the vfio check when the host procfs is not mounted", func() {
			Expect(os.RemoveAll(procRoot)).To(Succeed())
			Expect(nc().ensurePFCanBeReconfigured(pf)).To(Succeed())
		})

		It("refuses reconfiguration while a write on the PF is pending", func() {
			sysfsWrites.add(pendingSysfsWrite{pf: pf, path: "unbind", data: vf, since: time.Now()})
			var blockedErr *SysfsWriteBlockedError
			Expect(errors.As(nc().ensurePFCanBeReconfigured(pf), &blockedErr)).To(BeTrue())
		})
	})
})
