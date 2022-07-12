// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package daemon

import (
	"fmt"
	sriovv2 "github.com/smart-edge-open/sriov-fec-operator/api/v2"
	"github.com/k8snetworkplumbingwg/sriov-network-device-plugin/pkg/utils"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"path/filepath"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
	"time"
)

const (
	vfNumFileDefault = "sriov_numvfs"
	vfNumFileIgbUio  = "max_vfs"
)

var (
	runExecCmd       = execCmd
	getVFconfigured  = utils.GetVFconfigured
	getVFList        = utils.GetVFList
	workdir          = "/sriov_artifacts"
	sysBusPciDevices = "/sys/bus/pci/devices"
	sysBusPciDrivers = "/sys/bus/pci/drivers"
)

func NewNodeConfigurator(logger *logrus.Logger, PfBBConfigController *pfBBConfigController, client client.Client, nodeNameRef types.NamespacedName) *NodeConfigurator {
	return &NodeConfigurator{
		Client:               client,
		Log:                  logger,
		nodeNameRef:          nodeNameRef,
		pfBBConfigController: PfBBConfigController,
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
	_, err := runExecCmd([]string{"chroot", "/host/", "modprobe", module}, n.Log)
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
	err = ioutil.WriteFile(unbindPath, []byte(pciAddress), os.ModeAppend)
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
	if err := ioutil.WriteFile(driverOverridePath, []byte(driver), os.ModeAppend); err != nil {
		n.Log.WithError(err).WithField("path", driverOverridePath).WithField("driver", driver).Error("failed to override driver")
		return err
	}

	driverBindPath := filepath.Join(sysBusPciDrivers, driver, "bind")
	n.Log.WithField("path", driverBindPath).Info("driver bind path")
	err := ioutil.WriteFile(driverBindPath, []byte(pciAddress), os.ModeAppend)
	if err != nil {
		n.Log.WithError(err).WithField("pciAddress", pciAddress).WithField("driverBindPath", driverBindPath).Error("failed to bind driver to device")
	}

	return err
}

func (n *NodeConfigurator) enableMasterBus(pciAddr string) error {
	const MASTER_BUS_BIT int64 = 4
	cmd := []string{"chroot", "/host/", "setpci", "-v", "-s", pciAddr, "COMMAND"}
	out, err := runExecCmd(cmd, n.Log)
	if err != nil {
		n.Log.WithError(err).Error("failed to get the PCI flags for: " + pciAddr)
		return err
	}

	values := strings.Split(out, " = ")
	if len(values) != 2 {
		return fmt.Errorf("unexpected output form \"%s\": %s", strings.Join(cmd, " "), out)
	}

	v, err := strconv.ParseInt(strings.Replace(values[1], "\n", "", 1), 16, 16)
	if err != nil {
		n.Log.WithError(err).WithField("value", v).Error("failed to parse the value")
		return err
	}

	if v&MASTER_BUS_BIT == MASTER_BUS_BIT {
		n.Log.Info("MasterBus already set for " + pciAddr)
		return nil
	}

	v = v | MASTER_BUS_BIT
	cmd = []string{"chroot", "/host/", "setpci", "-v", "-s", pciAddr, fmt.Sprintf("COMMAND=0%x", v)}
	out, err = runExecCmd(cmd, n.Log)
	if err != nil {
		n.Log.WithField("output", out).WithError(err).Error("failed to set MasterBus bit")
		return err
	}

	n.Log.WithField("pci", pciAddr).WithField("output", out).Info("MasterBus set")
	return nil
}

func (n *NodeConfigurator) changeAmountOfVFs(driver string, pfPCIAddress string, vfsAmount int) error {
	currentAmount := getVFconfigured(pfPCIAddress)
	if currentAmount == vfsAmount {
		return nil
	}

	writeVfs := func(pfPCIAddress string, vfsAmount int) error {
		unbindPath := filepath.Join(sysBusPciDevices, pfPCIAddress)

		switch driver {
		case "pci-pf-stub", "pci_pf_stub", "vfio-pci":
			unbindPath = filepath.Join(unbindPath, vfNumFileDefault)
		case "igb_uio":
			unbindPath = filepath.Join(unbindPath, vfNumFileIgbUio)
		default:
			return fmt.Errorf("unknown driver %v", driver)
		}

		err := ioutil.WriteFile(unbindPath, []byte(strconv.Itoa(vfsAmount)), os.ModeAppend)
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
	if err := ioutil.WriteFile(path, []byte(strconv.Itoa(1)), os.ModeAppend); err != nil {
		return fmt.Errorf("failed to execute Function Level Reset for PF (%s): %s", pfPCIAddress, err)
	}

	// In some cases we have noticed that FLR can take a while - according to spec it shouldn't be longer than 100ms.
	// if FLR is running on card, then pf-bb-config will fail.
	time.Sleep(500 * time.Millisecond)
	return nil
}

func (n *NodeConfigurator) cleanAcceleratorConfig(acc sriovv2.SriovAccelerator) error {

	n.Log.Infof("cleaning configuration on %s", acc.PCIAddress)

	if err := unbindVFs(n, acc); err != nil {
		return err
	}

	if err := removeVFs(n, acc); err != nil {
		return err
	}

	if err := n.flrReset(acc.PCIAddress); err != nil {
		return err
	}

	if err := n.pfBBConfigController.stopPfBBConfig(acc.PCIAddress); err != nil {
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

func (n *NodeConfigurator) ApplySpec(nodeConfig sriovv2.SriovFecNodeConfigSpec) error {
	inv, err := getSriovInventory(n.Log)
	if err != nil {
		n.Log.WithError(err).Error("failed to obtain current sriov inventory")
		return err
	}

	n.Log.WithField("inventory", inv).Info("current node status")

	for _, acc := range inv.SriovAccelerators {
		requestedConfig := getMatchingConfiguration(acc.PCIAddress, nodeConfig.PhysicalFunctions)
		if requestedConfig == nil {
			if len(acc.VFs) > 0 {
				n.Log.WithField("pci", acc.PCIAddress).WithField("driverName", acc.PFDriver).Info("zeroing VFs")
				if err := n.cleanAcceleratorConfig(acc); err != nil {
					return err
				}
			}

			continue
		}
		if err := n.configureAccelerator(acc, requestedConfig); err != nil {
			return err
		}
	}

	return nil
}

func (n *NodeConfigurator) configureAccelerator(acc sriovv2.SriovAccelerator, requestedConfig *sriovv2.PhysicalFunctionConfigExt) error {
	n.Log.WithField("requestedConfig", requestedConfig).Info("configuring PF")

	if err := n.cleanAcceleratorConfig(acc); err != nil {
		return err
	}

	if err := loadDrivers(n, requestedConfig.PFDriver, requestedConfig.VFDriver); err != nil {
		return err
	}

	if err := n.bindDeviceToDriver(requestedConfig.PCIAddress, requestedConfig.PFDriver); err != nil {
		return err
	}

	if err := n.changeAmountOfVFs(requestedConfig.PFDriver, requestedConfig.PCIAddress, requestedConfig.VFAmount); err != nil {
		return err
	}

	pciStubRegex := regexp.MustCompile("pci[-_]pf[-_]stub")
	if pciStubRegex.MatchString(requestedConfig.PFDriver) {
		if err := n.enableMasterBus(requestedConfig.PCIAddress); err != nil {
			return err
		}
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

	if err := n.pfBBConfigController.initializePfBBConfig(acc, requestedConfig); err != nil {
		return err
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
