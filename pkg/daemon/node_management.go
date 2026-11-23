// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2026 Intel Corporation

package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sriovv2 "github.com/intel/sriov-fec-operator/api/sriovfec/v2"
	vrbv1 "github.com/intel/sriov-fec-operator/api/sriovvrb/v1"
	sriovutils "github.com/intel/sriov-fec-operator/pkg/common/utils"
	"github.com/k8snetworkplumbingwg/sriov-network-device-plugin/pkg/utils"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	vfNumFileDefault = "sriov_numvfs"
	vfNumFileIgbUio  = "max_vfs"
)

var (
	runExecCmd       = execCmd
	getVFconfigured  = readVFconfigured
	getVFList        = utils.GetVFList
	workdir          = "/tmp"
	sysBusPciDevices = "/sys/bus/pci/devices"
	sysBusPciDrivers = "/sys/bus/pci/drivers"
)

func NewNodeConfigurator(logger *logrus.Logger, pfBBConfigController *pfBBConfigController, client client.Client, nodeNameRef types.NamespacedName) *NodeConfigurator {
	return &NodeConfigurator{
		Client:               client,
		Log:                  logger,
		nodeNameRef:          nodeNameRef,
		pfBBConfigController: pfBBConfigController,
	}
}

type NodeConfigurator struct {
	client.Client
	Log                  *logrus.Logger
	nodeNameRef          types.NamespacedName
	pfBBConfigController *pfBBConfigController
}

func (n *NodeConfigurator) loadModule(module string) error {
	if module == "" {
		return fmt.Errorf("module cannot be empty string")
	}
	_, err := runExecCmd(append([]string{"modprobe", module}, appendMandatoryArgs(module)...), n.Log)
	return err
}

func (n *NodeConfigurator) isDeviceBoundToDriver(pciAddr string) (bool, error) {
	path := filepath.Join(sysBusPciDevices, pciAddr, "driver")

	if _, err := os.Stat(path); err == nil {
		n.Log.WithField("path", path).Info("device is bound to driver")
		return true, nil

	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func (n *NodeConfigurator) unbindDeviceFromDriver(pciAddress string) error {
	deviceDriverPath := filepath.Join(sysBusPciDevices, pciAddress, "driver")
	driverPath, err := filepath.EvalSymlinks(deviceDriverPath)
	if err != nil {
		n.Log.WithError(err).WithField("path", deviceDriverPath).Error("failed to read device's driver symlink")
		return err
	}
	n.Log.WithField("pciAddress", pciAddress).WithField("driver", driverPath).Info("driver to unbound device from")
	unbindPath := filepath.Join(driverPath, "unbind")
	err = writeSysfs(pciAddress, unbindPath, pciAddress)
	if err != nil {
		n.Log.WithError(err).WithField("pciAddress", pciAddress).WithField("unbindPath", unbindPath).Error("failed to unbind driver from device")
	}

	return err
}

func (n *NodeConfigurator) unbindIfBound(pciAddress string) error {
	if isBound, err := n.isDeviceBoundToDriver(pciAddress); err != nil {
		n.Log.WithField("pci", pciAddress).WithError(err).Error("failed to check if device is bound to driver")
		return err
	} else if isBound {
		if err := n.unbindDeviceFromDriver(pciAddress); err != nil {
			n.Log.WithField("pci", pciAddress).WithError(err).Error("failed to unbind device from driver")
			return err
		}
	}
	return nil
}

func (n *NodeConfigurator) bindDeviceToDriver(pciAddress, driver string) error {
	if isBound, err := n.isDeviceBoundToDriver(pciAddress); err != nil {
		n.Log.WithField("pci", pciAddress).WithError(err).Error("failed to check if device is bound to driver")
		return err
	} else if isBound {
		if err := n.unbindDeviceFromDriver(pciAddress); err != nil {
			n.Log.WithField("pci", pciAddress).WithError(err).Error("failed to unbind device from driver")
			return err
		}
	}

	driverOverridePath := filepath.Join(sysBusPciDevices, pciAddress, "driver_override")
	n.Log.WithField("path", driverOverridePath).Info("device's driver_override path")
	if err := writeSysfs(pciAddress, driverOverridePath, driver); err != nil {
		n.Log.WithError(err).WithField("path", driverOverridePath).WithField("driver", driver).Error("failed to override driver")
		return err
	}

	driverBindPath := filepath.Join(sysBusPciDrivers, driver, "bind")
	n.Log.WithField("path", driverBindPath).Info("driver bind path")
	err := writeSysfs(pciAddress, driverBindPath, pciAddress)
	if err != nil {
		n.Log.WithError(err).WithField("pciAddress", pciAddress).WithField("driverBindPath", driverBindPath).Error("failed to bind driver to device")
	}

	return err
}

func (n *NodeConfigurator) configureCommandRegister(pciAddr string) error {
	// Configures PCI COMMAND register that enables
	// 0X02 bit - PCI_COMMAND_MEMORY which is required for MMIO in pf-bb-config
	// 0X04 bit - PCI_COMMAND_MASTER which required for PF to correctly manage VFs
	// The ":06" mask sets only these two bits. Without it the whole register
	// would be overwritten, clearing e.g. INTx Disable and SERR# Enable.
	cmd := []string{"setpci", "-v", "-s", pciAddr, "COMMAND=06:06"}
	_, err := runExecCmd(cmd, n.Log)
	if err != nil {
		n.Log.WithError(err).Error("failed to configure PCI command bridge for card: " + pciAddr)
		return err
	}
	return nil
}

func (n *NodeConfigurator) changeAmountOfVFs(driver string, pfPCIAddress string, vfsAmount int) error {
	currentAmount, err := getVFconfigured(pfPCIAddress)
	if err != nil {
		n.Log.WithError(err).WithField("pf", pfPCIAddress).Error("failed to read current amount of VFs")
		return fmt.Errorf("failed to read current amount of VFs for PF (%s): %w", pfPCIAddress, err)
	}
	if currentAmount == vfsAmount {
		return nil
	}

	writeVfs := func(pfPCIAddress string, vfsAmount int) error {
		unbindPath := filepath.Join(sysBusPciDevices, pfPCIAddress)

		switch driver {
		case sriovutils.PciPfStubDash, sriovutils.PciPfStubUnderscore, sriovutils.VfioPci:
			unbindPath = filepath.Join(unbindPath, vfNumFileDefault)
		case sriovutils.IgbUio:
			unbindPath = filepath.Join(unbindPath, vfNumFileIgbUio)
		default:
			return fmt.Errorf("unknown driver %v", driver)
		}

		err := writeSysfs(pfPCIAddress, unbindPath, strconv.Itoa(vfsAmount))
		if err != nil {
			n.Log.WithError(err).WithField("pf", pfPCIAddress).WithField("vfsAmount", vfsAmount).Error("failed to set new amount of VFs for PF")
			return fmt.Errorf("failed to set new amount of VFs (%d) for PF (%s): %w", vfsAmount, pfPCIAddress, err)
		}
		return nil
	}

	if currentAmount > 0 {
		if err := writeVfs(pfPCIAddress, 0); err != nil {
			return err
		}
	}

	if vfsAmount > 0 {
		return writeVfs(pfPCIAddress, vfsAmount)
	}

	return nil
}

func (n *NodeConfigurator) flrReset(pfPCIAddress string) error {
	n.Log.Infof("executing FLR for %s", pfPCIAddress)

	path := filepath.Join(sysBusPciDevices, pfPCIAddress, "reset")
	if err := writeSysfs(pfPCIAddress, path, strconv.Itoa(1)); err != nil {
		return fmt.Errorf("failed to execute Function Level Reset for PF (%s): %w", pfPCIAddress, err)
	}

	// Add a 1-second delay between PF FLR and starting pf_bb_config
	// to ensure that the PF is fully reset
	time.Sleep(1 * time.Second)

	return nil
}

func (n *NodeConfigurator) cleanAcceleratorConfig(acc sriovv2.SriovAccelerator) error {
	n.Log.Infof("cleaning configuration on %s", acc.PCIAddress)

	if err := n.ensurePFCanBeReconfigured(acc.PCIAddress); err != nil {
		return err
	}

	if err := n.pfBBConfigController.stopPfBBConfig(acc.PCIAddress); err != nil {
		return err
	}

	if err := unbindVFs(n, acc); err != nil {
		return err
	}

	if err := removeVFs(n, acc); err != nil {
		return err
	}

	n.Log.WithField("pciAddress", acc.PCIAddress).Debug("unbind PF device from driver")
	if err := n.unbindIfBound(acc.PCIAddress); err != nil {
		return err
	}

	if err := n.flrReset(acc.PCIAddress); err != nil {
		return err
	}

	// the PF is now in a known state
	staleSysfsWrites.clear(acc.PCIAddress)

	return nil
}

func (n *NodeConfigurator) VrbcleanAcceleratorConfig(acc vrbv1.SriovAccelerator) error {
	n.Log.Infof("cleaning configuration on %s", acc.PCIAddress)

	if err := n.ensurePFCanBeReconfigured(acc.PCIAddress); err != nil {
		return err
	}

	if err := n.pfBBConfigController.stopPfBBConfig(acc.PCIAddress); err != nil {
		return err
	}

	if err := VrbunbindVFs(n, acc); err != nil {
		return err
	}

	if err := VrbremoveVFs(n, acc); err != nil {
		return err
	}

	n.Log.WithField("pciAddress", acc.PCIAddress).Debug("unbind PF device from driver")
	if err := n.unbindIfBound(acc.PCIAddress); err != nil {
		return err
	}

	if err := n.flrReset(acc.PCIAddress); err != nil {
		return err
	}

	// the PF is now in a known state
	staleSysfsWrites.clear(acc.PCIAddress)

	return nil
}

// ensurePFCanBeReconfigured checks, before anything on the PF is touched, that
// tearing it down will not block in the kernel:
//   - no earlier sysfs write on this PF, one of its VFs, or a node-wide
//     attribute is still pending (a VF unbind that timed out removes the VF's
//     "driver" link right away, so the VF looks unbound although the unbind
//     still waits for the vfio user),
//   - no VF of the PF is held open through vfio by a userspace driver: its
//     vfio group cannot be opened (EBUSY), or a process in the host /proc
//     holds it.
//
// Without this, the teardown queues sriov_numvfs and reset writes behind the
// blocked unbind; the pending reset blocks config access to the upstream
// bridge and hangs lspci on the node.
func (n *NodeConfigurator) ensurePFCanBeReconfigured(pfPCIAddress string) error {
	if p := sysfsWrites.blocking(pfPCIAddress); p != nil {
		err := &SysfsWriteBlockedError{Path: filepath.Join(sysBusPciDevices, pfPCIAddress), Pending: *p}
		n.Log.WithError(err).WithField("pf", pfPCIAddress).Error("PF cannot be reconfigured")
		return err
	}

	vfs, err := getVFList(pfPCIAddress)
	if err != nil {
		n.Log.WithError(err).WithField("pf", pfPCIAddress).Error("failed to get list of VFs")
		return err
	}
	if len(vfs) == 0 {
		return nil
	}

	// A VF is in use if its vfio group cannot be opened (EBUSY). That does
	// not work for VFs that share the PF's group (no ACS): pf_bb_config
	// holds that group. For those, and to name the holders, look at the
	// processes in the host /proc if it is mounted.
	pfGroup := iommuGroupOf(pfPCIAddress)
	var busy, sharedWithPF []string
	for _, vf := range vfs {
		group := iommuGroupOf(vf)
		switch {
		case group == "":
			continue
		case group == pfGroup:
			sharedWithPF = append(sharedWithPF, vf)
		default:
			inUse, err := vfioGroupInUse(group)
			if err != nil {
				return fmt.Errorf("cannot check whether VF %s is in use through vfio group %s: %w", vf, group, err)
			}
			if inUse {
				busy = append(busy, vf)
			}
		}
	}

	users := map[string][]string{}
	if toScan := append(append([]string{}, busy...), sharedWithPF...); len(toScan) > 0 {
		found, err := findVfioUsers(toScan)
		switch {
		case errors.Is(err, errProcUnavailable):
			if len(sharedWithPF) > 0 {
				n.Log.WithError(err).WithField("pf", pfPCIAddress).WithField("vfs", sharedWithPF).
					Warning("cannot check whether VFs sharing the PF's IOMMU group are in use, mount the host /proc at " + procRoot)
			}
		case err != nil:
			return err
		default:
			users = found
		}
	}
	for _, vf := range busy {
		if len(users[vf]) == 0 {
			users[vf] = []string{"unknown process"}
		}
	}

	if len(users) > 0 {
		err := &DeviceInUseError{PF: pfPCIAddress, Users: users}
		n.Log.WithError(err).WithField("pf", pfPCIAddress).Error("PF cannot be reconfigured")
		return err
	}
	return nil
}

func removeVFs(nc *NodeConfigurator, acc sriovv2.SriovAccelerator) error {
	if len(acc.VFs) > 0 {
		if err := nc.changeAmountOfVFs(acc.PFDriver, acc.PCIAddress, 0); err != nil {
			return err
		}
	}
	return nil
}

func VrbremoveVFs(nc *NodeConfigurator, acc vrbv1.SriovAccelerator) error {
	if len(acc.VFs) > 0 {
		if err := nc.changeAmountOfVFs(acc.PFDriver, acc.PCIAddress, 0); err != nil {
			return err
		}
	}
	return nil
}

func unbindVFs(nc *NodeConfigurator, acc sriovv2.SriovAccelerator) error {
	existingVfs, err := getVFList(acc.PCIAddress)
	if err != nil {
		nc.Log.WithError(err).Error("failed to get list of newly created VFs")
		return err
	}

	for _, vf := range existingVfs {
		if err := nc.unbindIfBound(vf); err != nil {
			return err
		}
	}
	return nil
}

func VrbunbindVFs(nc *NodeConfigurator, acc vrbv1.SriovAccelerator) error {
	existingVfs, err := getVFList(acc.PCIAddress)
	if err != nil {
		nc.Log.WithError(err).Error("failed to get list of newly created VFs")
		return err
	}

	for _, vf := range existingVfs {
		if err := nc.unbindIfBound(vf); err != nil {
			return err
		}
	}
	return nil
}

func loadDrivers(nc *NodeConfigurator, pfDriver string, vfDriver string) error {
	if err := nc.loadModule(pfDriver); err != nil {
		nc.Log.WithField("driver", pfDriver).Info("failed to load module for PF driver")
		return err
	}

	if err := nc.loadModule(vfDriver); err != nil {
		nc.Log.WithField("driver", vfDriver).Info("failed to load module for VF driver")
		return err
	}
	return nil
}

func (n *NodeConfigurator) ApplySpec(nodeConfig sriovv2.SriovFecNodeConfigSpec, fecDeviceUpdateRequired map[string]bool) error {
	inv, err := getSriovInventory(n.Log)
	if err != nil {
		n.Log.WithError(err).Error("failed to obtain current sriov inventory")
		return err
	}

	n.Log.WithField("inventory", inv).Info("current node status")

	for _, acc := range inv.SriovAccelerators {
		if !fecDeviceUpdateRequired[acc.PCIAddress] {
			continue
		}
		requestedConfig := getMatchingConfiguration(acc.PCIAddress, nodeConfig.PhysicalFunctions)
		if requestedConfig == nil {
			if len(acc.VFs) > 0 {
				n.Log.WithField("pci", acc.PCIAddress).WithField("driverName", acc.PFDriver).Info("zeroing VFs")
				if err := n.cleanAcceleratorConfig(acc); err != nil {
					return err
				}
			}
			// not managed by the operator: nothing to reconfigure
			staleSysfsWrites.clear(acc.PCIAddress)

			continue
		}
		if err := n.configureAccelerator(acc, requestedConfig); err != nil {
			return err
		}
	}

	return nil
}

func (n *NodeConfigurator) VrbApplySpec(nodeConfig vrbv1.SriovVrbNodeConfigSpec, vrbDeviceUpdateRequired map[string]bool) error {
	inv, err := VrbgetSriovInventory(n.Log)
	if err != nil {
		n.Log.WithError(err).Error("failed to obtain current sriov inventory")
		return err
	}

	n.Log.WithField("inventory", inv).Info("current node status")

	for _, acc := range inv.SriovAccelerators {
		if !vrbDeviceUpdateRequired[acc.PCIAddress] {
			continue
		}
		requestedConfig := VrbgetMatchingConfiguration(acc.PCIAddress, nodeConfig.PhysicalFunctions)
		if requestedConfig == nil {
			if len(acc.VFs) > 0 {
				n.Log.WithField("pci", acc.PCIAddress).WithField("driverName", acc.PFDriver).Info("zeroing VFs")
				if err := n.VrbcleanAcceleratorConfig(acc); err != nil {
					return err
				}
			}
			// not managed by the operator: nothing to reconfigure
			staleSysfsWrites.clear(acc.PCIAddress)

			continue
		}
		if err := n.VrbconfigureAccelerator(acc, requestedConfig); err != nil {
			return err
		}
	}

	return nil
}

func (n *NodeConfigurator) loadAndBindDrivers(pciAddress, pfDriver, vfDriver string) error {

	if err := loadDrivers(n, pfDriver, vfDriver); err != nil {
		return err
	}

	if err := n.bindDeviceToDriver(pciAddress, pfDriver); err != nil {
		return err
	}

	return nil
}

func (n *NodeConfigurator) configureAccelerator(acc sriovv2.SriovAccelerator, requestedConfig *sriovv2.PhysicalFunctionConfigExt) error {
	n.Log.WithField("requestedConfig", requestedConfig).Info("configuring PF")

	if err := n.cleanAcceleratorConfig(acc); err != nil {
		return err
	}

	if err := n.loadAndBindDrivers(requestedConfig.PCIAddress, requestedConfig.PFDriver, requestedConfig.VFDriver); err != nil {
		return err
	}

	if requestedConfig.BBDevConfig.N3000 != nil {
		if err := n.configureCommandRegister(requestedConfig.PCIAddress); err != nil {
			return err
		}
	}

	if err := n.pfBBConfigController.initializePfBBConfig(acc, requestedConfig); err != nil {
		return err
	}

	if err := n.changeAmountOfVFs(requestedConfig.PFDriver, requestedConfig.PCIAddress, requestedConfig.VFAmount); err != nil {
		return err
	}

	createdVfs, err := getVFList(acc.PCIAddress)
	if err != nil {
		n.Log.WithError(err).Error("failed to get list of newly created VFs")
		return err
	}

	for _, vf := range createdVfs {
		if err := n.bindDeviceToDriver(vf, requestedConfig.VFDriver); err != nil {
			return err
		}
	}

	return nil

}

func (n *NodeConfigurator) VrbconfigureAccelerator(acc vrbv1.SriovAccelerator, requestedConfig *vrbv1.PhysicalFunctionConfigExt) error {
	n.Log.WithField("requestedConfig", requestedConfig).Info("configuring PF")

	if err := n.VrbcleanAcceleratorConfig(acc); err != nil {
		return err
	}

	if err := n.loadAndBindDrivers(requestedConfig.PCIAddress, requestedConfig.PFDriver, requestedConfig.VFDriver); err != nil {
		return err
	}

	if err := n.pfBBConfigController.VrbinitializePfBBConfig(acc, requestedConfig); err != nil {
		return err
	}

	if err := n.changeAmountOfVFs(requestedConfig.PFDriver, requestedConfig.PCIAddress, requestedConfig.VFAmount); err != nil {
		return err
	}

	createdVfs, err := getVFList(acc.PCIAddress)
	if err != nil {
		n.Log.WithError(err).Error("failed to get list of newly created VFs")
		return err
	}

	for _, vf := range createdVfs {
		if err := n.bindDeviceToDriver(vf, requestedConfig.VFDriver); err != nil {
			return err
		}
	}

	return nil

}

func getMatchingConfiguration(pciAddress string, configurations []sriovv2.PhysicalFunctionConfigExt) *sriovv2.PhysicalFunctionConfigExt {
	for _, configuration := range configurations {
		if configuration.PCIAddress == pciAddress {
			return &configuration
		}
	}
	return nil
}

func VrbgetMatchingConfiguration(pciAddress string, configurations []vrbv1.PhysicalFunctionConfigExt) *vrbv1.PhysicalFunctionConfigExt {
	for _, configuration := range configurations {
		if configuration.PCIAddress == pciAddress {
			return &configuration
		}
	}
	return nil
}

func appendMandatoryArgs(driver string) []string {
	if strings.EqualFold(driver, sriovutils.VfioPci) {
		return []string{"enable_sriov=1", "disable_idle_d3=1"}
	}
	return []string{}
}
