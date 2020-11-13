// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/intel/sriov-network-device-plugin/pkg/utils"
	sriovv1 "github.com/otcshare/openshift-operator/sriov-fec/api/v1"
)

const (
	redhatReleaseFilepath = "/host/etc/redhat-release"
	procCmdlineFilePath   = "/host/proc/cmdline"
	sysBusPciDevices      = "/sys/bus/pci/devices"
	sysBusPciDrivers      = "/sys/bus/pci/drivers"
	vfNumFile             = "sriov_numvfs"
	workdir               = "/sriov_artifacts"
)

var (
	errWrongOS   = errors.New("running on non-CoreOS system. Only CoreOS is supported")
	kernelParams = []string{"intel_iommu=on", "iommu=pt"}
)

type NodeConfigurator struct {
	Log logr.Logger
}

func (n *NodeConfigurator) execCmd(args []string) (string, error) {
	var cmd *exec.Cmd
	if len(args) == 0 {
		n.Log.Error(nil, "provided cmd is empty")
		return "", errors.New("cmd is empty")
	} else if len(args) == 1 {
		cmd = exec.Command(args[0])
	} else {
		cmd = exec.Command(args[0], args[1:]...)
	}

	n.Log.Info("executing command", "cmd", cmd)

	out, err := cmd.Output()
	if err != nil {
		n.Log.Error(err, "failed to execute command", "cmd", args, "output", string(out))
		return "", err
	}

	output := string(out)

	n.Log.Info("commands output", "output", output)

	return output, nil
}

func (n *NodeConfigurator) checkIfCoreOS() (bool, error) {
	if _, err := os.Stat(redhatReleaseFilepath); err == nil {
		n.Log.V(2).Info("redhat-release file exists")

		content, err := ioutil.ReadFile(redhatReleaseFilepath)
		if err != nil {
			n.Log.Error(err, "failed to read contents of redhat-release file")
			return false, err
		}

		isCoreOS := strings.Contains(string(content), "CoreOS")
		n.Log.V(2).Info("coreos", "isCoreOS", isCoreOS)
		return isCoreOS, nil

	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}

}

// anyKernelParamsMissing checks current kernel cmdline
// returns true if /proc/cmdline requires update
func (n *NodeConfigurator) isAnyKernelParamsMissing() (bool, error) {
	log := n.Log.WithName("isAnyKernelParamsMissing")

	coreOS, err := n.checkIfCoreOS()
	if err != nil {
		return false, err
	}

	if !coreOS {
		return false, errWrongOS
	}

	cmdlineBytes, err := ioutil.ReadFile(procCmdlineFilePath)
	if err != nil {
		log.Error(err, "failed to read file contents", "path", procCmdlineFilePath)
		return false, err
	}
	cmdline := string(cmdlineBytes)

	for _, param := range kernelParams {
		if !strings.Contains(cmdline, param) {
			log.Info("missing kernel param", "param", param)
			return true, nil
		}
	}

	return false, nil
}

// addMissingKernelParams adds missing kernel params to rpm-ostree kargs so after next reboot /proc/cmdline will be correct
// true is returned if reboot is required
func (n *NodeConfigurator) addMissingKernelParams() (bool, error) {
	log := n.Log.WithName("addMissingKernelParams")

	kargs, err := n.execCmd([]string{"chroot", "/host/", "rpm-ostree", "kargs"})
	if err != nil {
		return false, err
	}

	log.V(2).Info("rpm-ostree", "kargs", kargs)

	anyParamAdded := false

	for _, param := range kernelParams {
		if !strings.Contains(kargs, param) {
			log.V(2).Info("missing param - adding", "param", param)
			_, err = n.execCmd([]string{"chroot", "/host/", "rpm-ostree", "kargs", "--append", param})
			if err != nil {
				return false, nil
			}

			anyParamAdded = true
		}
	}

	log.V(2).Info("added missing params", "anyParamAdded", anyParamAdded)
	return anyParamAdded, nil
}

func (n *NodeConfigurator) loadModule(module string) error {
	_, err := n.execCmd([]string{"chroot", "/host/", "modprobe", module})
	return err
}

func (n *NodeConfigurator) rebootNode() error {
	// systemd-run command borrowed from openshift/sriov-network-operator
	_, err := n.execCmd([]string{"chroot", "/host",
		"systemd-run",
		"--unit", "sriov-fec-daemon-reboot",
		"--description", fmt.Sprintf("sriov-fec-daemon reboot"),
		"/bin/sh", "-c", "systemctl stop kubelet.service; reboot"})

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
	n.Log.Info("device's driver_override path", "path", driverOverridePath)
	if err := ioutil.WriteFile(driverOverridePath, []byte(driver), os.ModeAppend); err != nil {
		n.Log.Error(err, "failed to override driver", "path", driverOverridePath, "driver", driver)
		return err
	}

	driverBindPath := filepath.Join(sysBusPciDrivers, driver, "bind")
	n.Log.Info("driver bind path", "path", driverBindPath)
	err := ioutil.WriteFile(driverBindPath, []byte(pciAddress), os.ModeAppend)
	if err != nil {
		n.Log.Error(err, "failed to bind driver to device", "pciAddress", pciAddress, "driverBindPath", driverBindPath)
	}

	return err
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
	currentAmount := utils.GetVFconfigured(pfPCIAddress)
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

	inv, err := GetSriovInventory(log)
	if err != nil {
		log.Error(err, "failed to obtain current sriov inventory")
		return err
	}

	log.Info("current node status", "inventory", inv)

	for _, pf := range nodeConfig.PhysicalFunctions {
		acc, exists := getMatchingExistingAccelerator(inv, pf.PCIAddress)
		if !exists {
			log.Info("received unknown (not present in inventory) PciAddress", "pci", pf.PCIAddress)
			return fmt.Errorf("unknown (%s not present in inventory) PciAddress", pf.PCIAddress)
		}

		log.Info("configuring PF", "requestedConfig", pf)

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

		createdVfs, err := utils.GetVFList(pf.PCIAddress)
		if err != nil {
			log.Error(err, "failed to get list of newly created VFs")
			return err
		}

		for _, vf := range createdVfs {
			if err := n.bindDeviceToDriver(vf, pf.VFDriver); err != nil {
				return err
			}
		}

		if pf.BBDevConfig.N3000 != nil {
			bbdevConfigFilepath := filepath.Join(workdir, fmt.Sprintf("%s.ini", pf.PCIAddress))
			if err := generateN3000BBDevConfigFile(pf.BBDevConfig.N3000, bbdevConfigFilepath); err != nil {
				log.Error(err, "failed to create bbdev config file", "pci", pf.PCIAddress)
				return err
			}
			defer func() {
				if err := os.Remove(bbdevConfigFilepath); err != nil {
					log.Error(err, "failed to remove old bbdev config file", "path", bbdevConfigFilepath)
				}
			}()

			deviceName := deviceIDWhitelist[acc.DeviceID].DeviceName
			if err := runPFConfig(log, deviceName, bbdevConfigFilepath, pf.PCIAddress); err != nil {
				log.Error(err, "failed to configure device's queues", "pci", pf.PCIAddress)
				return err
			}
		} else {
			log.Info("N3000 BBDevConfig is nil - queues will not be (re)configured")
		}
	}

	return nil
}
