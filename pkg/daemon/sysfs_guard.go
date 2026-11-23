// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2026 Intel Corporation

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

/*
 * Writes to PCI sysfs attributes (driver unbind, sriov_numvfs, reset) take
 * the device lock of the device they act on. When a userspace driver (e.g.
 * DPDK over vfio-pci) keeps a VF open, a VF unbind blocks in the kernel until
 * the VF is released, holding the VF device lock. Every later write that
 * needs that lock - sriov_numvfs on the PF, then reset on the PF - queues up
 * behind it, and a pending PF reset additionally blocks config space access
 * to the upstream bridge, which hangs lspci on the whole node.
 *
 * A timed out write cannot be cancelled: the thread stays blocked in the
 * kernel and the write is still carried out once the lock is released. The
 * helpers below therefore:
 *   - remember writes that are still blocked and refuse further writes that
 *     would queue up behind them,
 *   - read sysfs attributes that take the device lock with a timeout,
 *   - detect VFs held open through vfio before tearing a PF down.
 */

var (
	sysfsWriteTimeout = 60 * time.Second
	sysfsReadTimeout  = 10 * time.Second
	pciCmdTimeout     = 30 * time.Second
	pciCmdWaitDelay   = 5 * time.Second

	// host procfs, mounted read-only into the daemon pod
	procRoot = "/host/proc"
	// /dev/vfio of the host, mounted into the daemon pod
	devVfioPath = "/dev/vfio"
)

// commands that read or write PCI config space and can block in the kernel
// while config access to a device is blocked (pci_wait_cfg)
var pciConfigAccessCmds = map[string]bool{
	"lspci":  true,
	"setpci": true,
}

// SysfsWriteTimeoutError is returned when a sysfs write did not complete in
// time. The write is still pending in the kernel.
type SysfsWriteTimeoutError struct {
	Path string
	Data string
}

func (e *SysfsWriteTimeoutError) Error() string {
	return fmt.Sprintf("writing %q to %s did not complete within %s; the write is still pending in the kernel, "+
		"usually because the device is in use by another process (e.g. a DPDK application holding a VF through vfio-pci)",
		e.Data, e.Path, sysfsWriteTimeout)
}

// SysfsWriteBlockedError is returned when a write is refused because an
// earlier write it would wait for is still pending in the kernel.
type SysfsWriteBlockedError struct {
	Path    string
	Pending pendingSysfsWrite
}

func (e *SysfsWriteBlockedError) Error() string {
	return fmt.Sprintf("refusing to write to %s: earlier write of %q to %s is still pending in the kernel since %s; "+
		"release the device (stop the application using it) to let it complete",
		e.Path, e.Pending.data, e.Pending.path, e.Pending.since.Format(time.RFC3339))
}

// DeviceInUseError is returned when VFs of a PF are held open through vfio.
type DeviceInUseError struct {
	PF    string
	Users map[string][]string // VF PCI address -> "pid (comm)"
}

func (e *DeviceInUseError) Error() string {
	vfs := make([]string, 0, len(e.Users))
	for vf := range e.Users {
		vfs = append(vfs, vf)
	}
	sort.Strings(vfs)
	parts := make([]string, 0, len(vfs))
	for _, vf := range vfs {
		parts = append(parts, fmt.Sprintf("%s by [%s]", vf, strings.Join(e.Users[vf], ", ")))
	}
	return fmt.Sprintf("VFs of PF %s are in use through vfio: %s; stop the applications using them before reconfiguring the PF",
		e.PF, strings.Join(parts, "; "))
}

type pendingSysfsWrite struct {
	pf       string
	path     string
	data     string
	since    time.Time
	nodeWide bool
}

type sysfsWriteTracker struct {
	mu      sync.Mutex
	next    uint64
	pending map[uint64]pendingSysfsWrite
}

var sysfsWrites = newSysfsWriteTracker()

func newSysfsWriteTracker() *sysfsWriteTracker {
	return &sysfsWriteTracker{pending: map[uint64]pendingSysfsWrite{}}
}

func (t *sysfsWriteTracker) add(w pendingSysfsWrite) uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.next++
	t.pending[t.next] = w
	return t.next
}

func (t *sysfsWriteTracker) remove(id uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.pending, id)
}

// blocking returns a pending write that a new write for the given PF would
// queue up behind: a write on the same PF or one of its VFs, or a node-wide
// write (sriov_numvfs holds pci_rescan_remove_lock, reset holds the upstream
// bridge lock).
func (t *sysfsWriteTracker) blocking(pf string) *pendingSysfsWrite {
	t.mu.Lock()
	defer t.mu.Unlock()
	var found *pendingSysfsWrite
	for _, w := range t.pending {
		if w.pf == pf || w.nodeWide {
			if found == nil || w.since.Before(found.since) {
				w := w
				found = &w
			}
		}
	}
	return found
}

// pfOf returns the PF of a VF, or the address itself for a PF.
func pfOf(pciAddress string) string {
	link, err := os.Readlink(filepath.Join(sysBusPciDevices, pciAddress, "physfn"))
	if err != nil {
		return pciAddress
	}
	return filepath.Base(link)
}

func isNodeWideSysfsAttr(path string) bool {
	switch filepath.Base(path) {
	case vfNumFileDefault, vfNumFileIgbUio, "reset":
		return true
	}
	return false
}

// writeSysfs writes data to a sysfs attribute acting on pciAddress. It refuses
// to write if an earlier write it would wait for is still pending, and gives up
// waiting after sysfsWriteTimeout. A timed out write stays registered as
// pending until the kernel completes it.
func writeSysfs(pciAddress, path, data string) error {
	tracker := sysfsWrites
	pf := pfOf(pciAddress)
	if p := tracker.blocking(pf); p != nil {
		return &SysfsWriteBlockedError{Path: path, Pending: *p}
	}

	w := pendingSysfsWrite{
		pf:       pf,
		path:     path,
		data:     data,
		since:    time.Now(),
		nodeWide: isNodeWideSysfsAttr(path),
	}
	id := tracker.add(w)
	marker := createSysfsWriteMarker(w)

	done := make(chan error, 1)
	go func() {
		err := os.WriteFile(path, []byte(data), os.ModeAppend)
		tracker.remove(id)
		removeSysfsWriteMarker(marker)
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(sysfsWriteTimeout):
		return &SysfsWriteTimeoutError{Path: path, Data: data}
	}
}

// readSysfsWithTimeout reads a sysfs attribute that may block on a device
// lock (e.g. sriov_numvfs).
func readSysfsWithTimeout(path string) (string, error) {
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		data, err := os.ReadFile(path)
		done <- result{data, err}
	}()

	select {
	case r := <-done:
		return strings.TrimSpace(string(r.data)), r.err
	case <-time.After(sysfsReadTimeout):
		return "", fmt.Errorf("reading %s did not complete within %s; the device lock is held by a blocked operation", path, sysfsReadTimeout)
	}
}

// readVFconfigured returns the number of VFs enabled on a PF.
func readVFconfigured(pfPCIAddress string) (int, error) {
	data, err := readSysfsWithTimeout(filepath.Join(sysBusPciDevices, pfPCIAddress, vfNumFileDefault))
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(data)
	if err != nil {
		return 0, fmt.Errorf("unexpected content of %s for %s: %q", vfNumFileDefault, pfPCIAddress, data)
	}
	return n, nil
}

var errProcUnavailable = errors.New("host procfs is not available")

const pfBBConfigComm = "pf_bb_config"

type charDev struct {
	rdev uint64
}

// vfioDevicesOf returns the vfio character devices through which userspace
// can hold the given PCI device: its vfio group and its vfio cdev.
func vfioDevicesOf(pciAddress string) map[charDev]bool {
	devs := map[charDev]bool{}
	var paths []string

	if link, err := os.Readlink(filepath.Join(sysBusPciDevices, pciAddress, "iommu_group")); err == nil {
		group := filepath.Base(link)
		paths = append(paths, filepath.Join(devVfioPath, group), filepath.Join(devVfioPath, "noiommu-"+group))
	}
	if entries, err := os.ReadDir(filepath.Join(sysBusPciDevices, pciAddress, "vfio-dev")); err == nil {
		for _, e := range entries {
			paths = append(paths, filepath.Join(devVfioPath, "devices", e.Name()))
		}
	}

	for _, p := range paths {
		if d, ok := statCharDev(p); ok {
			devs[d] = true
		}
	}
	return devs
}

func statCharDev(path string) (charDev, bool) {
	fi, err := os.Stat(path)
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return charDev{}, false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return charDev{}, false
	}
	return charDev{rdev: uint64(st.Rdev)}, true
}

// findVfioUsers returns, for each of the given PCI devices that is held open
// through vfio, the processes holding it ("pid (comm)").
func findVfioUsers(pciAddresses []string) (map[string][]string, error) {
	devToPci := map[charDev]string{}
	for _, addr := range pciAddresses {
		for d := range vfioDevicesOf(addr) {
			devToPci[d] = addr
		}
	}
	users := map[string][]string{}
	if len(devToPci) == 0 {
		return users, nil
	}

	procs, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errProcUnavailable, err)
	}

	for _, p := range procs {
		pid, err := strconv.Atoi(p.Name())
		if err != nil || pid <= 0 {
			continue
		}
		// pf_bb_config holds the PF's vfio group; it is stopped as part of
		// the reconfiguration, and the VFs may share the PF's group when
		// the platform has no ACS.
		comm := procComm(pid)
		if comm == pfBBConfigComm {
			continue
		}
		fdDir := filepath.Join(procRoot, p.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue // process exited or not accessible
		}
		seen := map[string]bool{}
		for _, fd := range fds {
			fdPath := filepath.Join(fdDir, fd.Name())
			// Only stat fds that point at a vfio node, so that we never
			// touch files on other (possibly hung) filesystems.
			target, err := os.Readlink(fdPath)
			if err != nil || !strings.Contains(target, "vfio") {
				continue
			}
			d, ok := statCharDev(fdPath)
			if !ok {
				continue
			}
			addr, ok := devToPci[d]
			if !ok || seen[addr] {
				continue
			}
			seen[addr] = true
			users[addr] = append(users[addr], fmt.Sprintf("%d (%s)", pid, comm))
		}
	}
	return users, nil
}

func procComm(pid int) string {
	data, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "comm"))
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(data))
}

// newPciConfigCmd builds a command that accesses PCI config space. It is
// killed after pciCmdTimeout. If it is stuck in uninterruptible sleep and
// cannot be killed, Wait returns after pciCmdWaitDelay instead of hanging.
func newPciConfigCmd(args ...string) (*exec.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), pciCmdTimeout)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.WaitDelay = pciCmdWaitDelay
	return cmd, cancel
}

/*
 * Pending writes are also recorded on disk (in the pod's emptyDir), one file
 * per write, removed when the write returns. A file left behind by a previous
 * daemon instance means that instance exited, or was restarted, while the
 * write was in flight - typically blocked behind a vfio user and carried out
 * only when the device was released. The PF may then be in a state the
 * operator did not expect (e.g. a VF unbound with the VF count unchanged, which
 * the inventory check does not notice), so it is reconfigured once.
 */

type sysfsWriteMarker struct {
	PF    string    `json:"pf"`
	Path  string    `json:"path"`
	Data  string    `json:"data"`
	Since time.Time `json:"since"`
}

func sysfsWriteMarkerDir() string {
	return filepath.Join(workdir, "pending-sysfs-writes")
}

func createSysfsWriteMarker(w pendingSysfsWrite) string {
	dir := sysfsWriteMarkerDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return ""
	}
	f, err := os.CreateTemp(dir, "write-*.json")
	if err != nil {
		return ""
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(sysfsWriteMarker{PF: w.pf, Path: w.path, Data: w.data, Since: w.since}); err != nil {
		_ = os.Remove(f.Name())
		return ""
	}
	return f.Name()
}

func removeSysfsWriteMarker(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}

type staleSysfsWriteSet struct {
	mu  sync.Mutex
	pfs map[string][]sysfsWriteMarker
}

var (
	staleSysfsWrites        = &staleSysfsWriteSet{pfs: map[string][]sysfsWriteMarker{}}
	loadStaleSysfsWriteOnce sync.Once
)

// has tells whether a previous daemon instance left a write on pf unfinished
// and the PF has not been reconfigured since.
func (s *staleSysfsWriteSet) has(pf string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pfs[pf]) > 0
}

func (s *staleSysfsWriteSet) clear(pf string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pfs, pf)
}

func (s *staleSysfsWriteSet) add(m sysfsWriteMarker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pfs[m.PF] = append(s.pfs[m.PF], m)
}

// loadStaleSysfsWriteMarkers picks up the markers left by a previous daemon
// instance. It must run before this instance writes to sysfs; it is called
// from the reconciler constructors, before the manager starts.
func loadStaleSysfsWriteMarkers(log *logrus.Logger) {
	loadStaleSysfsWriteOnce.Do(func() {
		loadStaleSysfsWriteMarkersFrom(sysfsWriteMarkerDir(), staleSysfsWrites, log)
	})
}

func loadStaleSysfsWriteMarkersFrom(dir string, set *staleSysfsWriteSet, log *logrus.Logger) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		_ = os.Remove(path)
		if err != nil {
			continue
		}
		var m sysfsWriteMarker
		if err := json.Unmarshal(data, &m); err != nil || m.PF == "" {
			log.WithField("file", path).Warning("ignoring malformed pending sysfs write marker")
			continue
		}
		log.WithField("pf", m.PF).WithField("path", m.Path).WithField("data", m.Data).WithField("since", m.Since).
			Warning("a previous daemon instance did not see this sysfs write complete; the PF will be reconfigured")
		set.add(m)
	}
}
