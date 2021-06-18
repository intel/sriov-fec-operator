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

	"github.com/go-logr/logr"
	"github.com/k8snetworkplumbingwg/sriov-network-device-plugin/pkg/utils"
	sriovv1 "github.com/open-ness/openshift-operator/sriov-fec/api/v1"
)

var (
	sysBusPciDevices = "/sys/bus/pci/devices"
	sysBusPciDrivers = "/sys/bus/pci/drivers"
	vfNumFile        = "sriov_numvfs"
	workdir          = "/sriov_artifacts"
	runExecCmd       = execCmd
	getVFconfigured  = utils.GetVFconfigured
	getVFList        = utils.GetVFList
)

type NodeConfigurator struct {
	Log              logr.Logger
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
	log := n.Log.WithName("loadModule")
	_, err := runExecCmd([]string{"chroot", "/host/", "modprobe", module}, log)
	return err
}

func (n *NodeConfigurator) rebootNode() error {
	log := n.Log.WithName("rebootNode")
	// systemd-run command borrowed from openshift/sriov-network-operator
	_, err := runExecCmd([]string{"chroot", "--userspec", "0", "/host",
		"systemd-run",
		"--unit", "sriov-fec-daemon-reboot",
		"--description", "sriov-fec-daemon reboot",
		"/bin/sh", "-c", "systemctl stop kubelet.service; reboot"}, log)

	return err
}

func (n *NodeConfigurator) isDeviceBoundToDriver(pciAddr string) (bool, error) {
	path := filepath.Join(sysBusPciDevices, pciAddr, "driver")

	if _, err := os.Stat(path); err == nil {
		n.Log.V(2).Info("device is bound to driver", "path", path)
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
		n.Log.Error(err, "failed to read device's driver symlink", "path", deviceDriverPath)
		return err
	}
	n.Log.V(2).Info("driver to unbound device from", "pciAddress", pciAddress, "driver", driverPath)
	unbindPath := filepath.Join(driverPath, "unbind")
	err = ioutil.WriteFile(unbindPath, []byte(pciAddress), os.ModeAppend)
	if err != nil {
		n.Log.Error(err, "failed to unbind driver from device", "pciAddress", pciAddress, "unbindPath", unbindPath)
	}

	return err
}

func (n *NodeConfigurator) bindDeviceToDriver(pciAddress, driver string) error {
	if isBound, err := n.isDeviceBoundToDriver(pciAddress); err != nil {
		n.Log.Error(err, "failed to check if device is bound to driver", "pci", pciAddress)
		return err
	} else if isBound {
		if err := n.unbindDeviceFromDriver(pciAddress); err != nil {
			n.Log.Error(err, "failed to unbind device from driver", "pci", pciAddress)
			return err
		}
	}

	driverOverridePath := filepath.Join(sysBusPciDevices, pciAddress, "driver_override")
	n.Log.V(2).Info("device's driver_override path", "path", driverOverridePath)
	if err := ioutil.WriteFile(driverOverridePath, []byte(driver), os.ModeAppend); err != nil {
		n.Log.Error(err, "failed to override driver", "path", driverOverridePath, "driver", driver)
		return err
	}

	driverBindPath := filepath.Join(sysBusPciDrivers, driver, "bind")
	n.Log.V(2).Info("driver bind path", "path", driverBindPath)
	err := ioutil.WriteFile(driverBindPath, []byte(pciAddress), os.ModeAppend)
	if err != nil {
		n.Log.Error(err, "failed to bind driver to device", "pciAddress", pciAddress, "driverBindPath", driverBindPath)
	}

	return err
}

func (n *NodeConfigurator) enableMasterBus(pciAddr string) error {
	log := n.Log.WithName("enableMasterBus")
	const MASTER_BUS_BIT int64 = 4
	cmd := []string{"chroot", "/host/", "setpci", "-v", "-s", pciAddr, "COMMAND"}
	out, err := runExecCmd(cmd, log)
	if err != nil {
		log.Error(err, "failed to get the PCI flags for: "+pciAddr)
		return err
	}

	values := strings.Split(out, " = ")
	if len(values) != 2 {
		return fmt.Errorf("unexpected output form \"%s\": %s", strings.Join(cmd, " "), out)
	}

	v, err := strconv.ParseInt(strings.Replace(values[1], "\n", "", 1), 16, 16)
	if err != nil {
		log.Error(err, "failed to parse the value", "value", v)
		return err
	}

	if v&MASTER_BUS_BIT == MASTER_BUS_BIT {
		log.V(4).Info("MasterBus already set for " + pciAddr)
		return nil
	}

	v = v | MASTER_BUS_BIT
	cmd = []string{"chroot", "/host/", "setpci", "-v", "-s", pciAddr, fmt.Sprintf("COMMAND=0%x", v)}
	out, err = runExecCmd(cmd, log)
	if err != nil {
		log.Error(err, "failed to set MasterBus bit", "output", out)
		return err
	}

	log.V(2).Info("MasterBus set", "pci", pciAddr, "output", out)
	return nil
}

func getMatchingExistingAccelerator(inventory *sriovv1.NodeInventory, pciAddress string) (sriovv1.SriovAccelerator, bool) {
	for _, acc := range inventory.SriovAccelerators {
		if acc.PCIAddress == pciAddress {
			return acc, true
		}
	}
	return sriovv1.SriovAccelerator{}, false
}

func (n *NodeConfigurator) changeAmountOfVFs(pfPCIAddress string, vfsAmount int) error {
	currentAmount := getVFconfigured(pfPCIAddress)
	if currentAmount == vfsAmount {
		return nil
	}

	writeVfs := func(pfPCIAddress string, vfsAmount int) error {
		unbindPath := filepath.Join(sysBusPciDevices, pfPCIAddress, vfNumFile)
		err := ioutil.WriteFile(unbindPath, []byte(strconv.Itoa(vfsAmount)), os.ModeAppend)
		if err != nil {
			n.Log.Error(err, "failed to set new amount of VFs for PF", "pf", pfPCIAddress, "vfsAmount", vfsAmount)
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

func (n *NodeConfigurator) applyConfig(nodeConfig sriovv1.SriovFecNodeConfigSpec) error {
	log := n.Log.WithName("applyConfig")

	inv, err := getSriovInventory(log)
	if err != nil {
		log.Error(err, "failed to obtain current sriov inventory")
		return err
	}

	log.V(4).Info("current node status", "inventory", inv)
	pciStubRegex := regexp.MustCompile("pci[-_]pf[-_]stub")
	for _, pf := range nodeConfig.PhysicalFunctions {
		acc, exists := getMatchingExistingAccelerator(inv, pf.PCIAddress)
		if !exists {
			log.Info("received unknown (not present in inventory) PciAddress", "pci", pf.PCIAddress)
			return fmt.Errorf("unknown (%s not present in inventory) PciAddress", pf.PCIAddress)
		}

		log.V(4).Info("configuring PF", "requestedConfig", pf)

		if err := n.loadModule(pf.PFDriver); err != nil {
			log.Info("failed to load module for PF driver", "driver", pf.PFDriver)
			return err
		}

		if err := n.loadModule(pf.VFDriver); err != nil {
			log.Info("failed to load module for VF driver", "driver", pf.VFDriver)
			return err
		}

		if len(acc.VFs) > 0 {
			if err := n.changeAmountOfVFs(pf.PCIAddress, 0); err != nil {
				return err
			}
		}

		if err := n.bindDeviceToDriver(pf.PCIAddress, pf.PFDriver); err != nil {
			return err
		}

		if err := n.changeAmountOfVFs(pf.PCIAddress, pf.VFAmount); err != nil {
			return err
		}

		createdVfs, err := getVFList(pf.PCIAddress)
		if err != nil {
			log.Error(err, "failed to get list of newly created VFs")
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
				log.Error(err, "failed to create bbdev config file", "pci", pf.PCIAddress)
				return err
			}
			defer func() {
				if err := os.Remove(bbdevConfigFilepath); err != nil {
					log.Error(err, "failed to remove old bbdev config file", "path", bbdevConfigFilepath)
				}
			}()

			deviceName := supportedAccelerators.Devices[acc.DeviceID]
			if err := runPFConfig(log, deviceName, bbdevConfigFilepath, pf.PCIAddress); err != nil {
				log.Error(err, "failed to configure device's queues", "pci", pf.PCIAddress)
				return err
			}
		} else {
			log.V(4).Info("N3000 and ACC100 BBDevConfig are nil - queues will not be (re)configured")
		}

		if pciStubRegex.MatchString(pf.PFDriver) {
			if err := n.enableMasterBus(pf.PCIAddress); err != nil {
				return err
			}
		}
	}

	return nil
}
