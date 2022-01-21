// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package daemon

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/k8snetworkplumbingwg/sriov-network-device-plugin/pkg/utils"
	sriovv2 "github.com/smart-edge-open/openshift-operator/sriov-fec/api/v2"
)

const (
	vfNumFilePciPfStub = "sriov_numvfs"
	vfNumFileIgbUio    = "max_vfs"
)

var (
	runExecCmd       = execCmd
	getVFconfigured  = utils.GetVFconfigured
	getVFList        = utils.GetVFList
	workdir          = "/sriov_artifacts"
	sysBusPciDevices = "/sys/bus/pci/devices"
	sysBusPciDrivers = "/sys/bus/pci/drivers"
)

type NodeConfigurator struct {
	Log              *logrus.Logger
	kernelController *kernelController
}

// anyKernelParamsMissing checks current kernel cmdline
// returns true if /proc/cmdline requires update
func (n *NodeConfigurator) isAnyKernelParamsMissing() (bool, error) {
	return n.kernelController.isAnyKernelParamsMissing()
}

func (n *NodeConfigurator) addMissingKernelParams() error {
	return n.kernelController.addMissingKernelParams()
}

func (n *NodeConfigurator) loadModule(module string) error {
	if module == "" {
		return fmt.Errorf("module cannot be empty string")
	}
	_, err := runExecCmd([]string{"chroot", "/host/", "modprobe", module}, n.Log)
	return err
}

func (n *NodeConfigurator) rebootNode() error {
	// systemd-run command borrowed from openshift/sriov-network-operator
	_, err := runExecCmd([]string{"chroot", "--userspec", "0", "/host",
		"systemd-run",
		"--unit", "sriov-fec-daemon-reboot",
		"--description", "sriov-fec-daemon reboot",
		"/bin/sh", "-c", "systemctl stop kubelet.service; reboot"}, n.Log)

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
		case "pci-pf-stub", "pci_pf_stub":
			unbindPath = filepath.Join(unbindPath, vfNumFilePciPfStub)
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

func (n *NodeConfigurator) applyConfig(nodeConfig sriovv2.SriovFecNodeConfigSpec) error {
	inv, err := getSriovInventory(n.Log)
	if err != nil {
		n.Log.WithError(err).Error("failed to obtain current sriov inventory")
		return err
	}

	n.Log.WithField("inventory", inv).Info("current node status")

	pciStubRegex := regexp.MustCompile("pci[-_]pf[-_]stub")
	for _, acc := range inv.SriovAccelerators {
		pf := getMatchingConfiguration(acc.PCIAddress, nodeConfig.PhysicalFunctions)

		if pf == nil {
			if len(acc.VFs) > 0 {
				n.Log.WithField("pci", acc.PCIAddress).WithField("driverName", acc.PFDriver).Info("zeroing VFs")
				if err := n.changeAmountOfVFs(acc.PFDriver, acc.PCIAddress, 0); err != nil {
					return err
				}
			}
			continue
		}

		n.Log.WithField("requestedConfig", pf).Info("configuring PF")
		if err := n.loadModule(pf.PFDriver); err != nil {
			n.Log.WithField("driver", pf.PFDriver).Info("failed to load module for PF driver")
			return err
		}

		if err := n.loadModule(pf.VFDriver); err != nil {
			n.Log.WithField("driver", pf.VFDriver).Info("failed to load module for VF driver")
			return err
		}

		if len(acc.VFs) > 0 {
			if err := n.changeAmountOfVFs(pf.PFDriver, pf.PCIAddress, 0); err != nil {
				return err
			}
		}

		if err := n.bindDeviceToDriver(pf.PCIAddress, pf.PFDriver); err != nil {
			return err
		}

		if err := n.changeAmountOfVFs(pf.PFDriver, pf.PCIAddress, pf.VFAmount); err != nil {
			return err
		}

		createdVfs, err := getVFList(pf.PCIAddress)
		if err != nil {
			n.Log.WithError(err).Error("failed to get list of newly created VFs")
			return err
		}

		for _, vf := range createdVfs {
			if err := n.bindDeviceToDriver(vf, pf.VFDriver); err != nil {
				return err
			}
		}

		if pf.BBDevConfig.N3000 != nil || pf.BBDevConfig.ACC100 != nil {
			bbdevConfigFilepath := filepath.Join(workdir, fmt.Sprintf("%s.ini", pf.PCIAddress))
			if err := generateBBDevConfigFile(pf.BBDevConfig, bbdevConfigFilepath); err != nil {
				n.Log.WithError(err).WithField("pci", pf.PCIAddress).Error("failed to create bbdev config file")
				return err
			}
			defer func() {
				if err := os.Remove(bbdevConfigFilepath); err != nil {
					n.Log.WithError(err).WithField("path", bbdevConfigFilepath).Error("failed to remove old bbdev config file")
				}
			}()

			deviceName := supportedAccelerators.Devices[acc.DeviceID]
			if err := runPFConfig(n.Log, deviceName, bbdevConfigFilepath, pf.PCIAddress); err != nil {
				n.Log.WithError(err).WithField("pci", pf.PCIAddress).Error("failed to configure device's queues")
				return err
			}
		} else {
			n.Log.Info("N3000 and ACC100 BBDevConfig are nil - queues will not be (re)configured")
		}

		if pciStubRegex.MatchString(pf.PFDriver) {
			if err := n.enableMasterBus(pf.PCIAddress); err != nil {
				return err
			}
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
