// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2025 Intel Corporation

package daemon

import (
	"errors"
	"strings"

	commonUtils "github.com/intel/sriov-fec-operator/pkg/common/utils"
	"github.com/jaypipes/ghw/pkg/pci"
	"github.com/sirupsen/logrus"

	sriovv2 "github.com/intel/sriov-fec-operator/api/sriovfec/v2"
	vrbv1 "github.com/intel/sriov-fec-operator/api/sriovvrb/v1"
	"github.com/jaypipes/ghw"
	"github.com/k8snetworkplumbingwg/sriov-network-device-plugin/pkg/utils"
)

func getVFDeviceInfo(log *logrus.Logger, pciInfo *ghw.PCIInfo, pfAddress, vfAddress string) (string, string) {

	driver, err := utils.GetDriverName(vfAddress)
	if err != nil {
		driver = ""
		log.WithFields(logrus.Fields{
			"pci":    vfAddress,
			"pf":     pfAddress,
			"reason": err.Error(),
		}).Info("failed to get driver name for VF")
	}

	deviceID := ""
	if vfDeviceInfo := pciInfo.GetDevice(vfAddress); vfDeviceInfo == nil {
		log.WithField("pci", vfAddress).Info("failed to get device info for vf")
	} else {
		deviceID = vfDeviceInfo.Product.ID
	}

	return driver, deviceID
}

func GetSriovInventory(log *logrus.Logger) (*sriovv2.NodeInventory, error) {
	pciInfo, err := ghw.PCI()
	if err != nil {
		log.WithError(err).Error("failed to get PCI info")
		return nil, err
	}

	devices := pciInfo.ListDevices()
	if len(devices) == 0 {
		log.Info("got 0 pci devices")
		err := errors.New("pci.ListDevices() returned 0 devices")
		return nil, err
	}

	accelerators := &sriovv2.NodeInventory{
		SriovAccelerators: []sriovv2.SriovAccelerator{},
	}

	for _, device := range commonUtils.Filter(devices, isKnownDevice) {
		if !utils.IsSriovPF(device.Address) {
			log.WithField("pci", device.Address).Info("ignoring non SriovPF capable device")
			continue
		}

		driver, err := utils.GetDriverName(device.Address)
		if err != nil {
			driver = ""
			if strings.Contains(err.Error(), "no such file or directory") {
				log.WithField("pci", device.Address).WithField("reason", err.Error()).Debug("driver link does not exist for device")
			} else {
				log.WithField("pci", device.Address).WithField("reason", err.Error()).Info("unable to get driver for device")
			}
		}

		acc := sriovv2.SriovAccelerator{
			VendorID:   device.Vendor.ID,
			DeviceID:   device.Product.ID,
			PCIAddress: device.Address,
			PFDriver:   driver,
			MaxVFs:     utils.GetSriovVFcapacity(device.Address),
			VFs:        []sriovv2.VF{},
		}

		vfs, err := utils.GetVFList(device.Address)
		if err != nil {
			log.WithError(err).WithField("pci", device.Address).Error("failed to get list of VFs for device")
		}

		for _, vf := range vfs {
			vfInfo := sriovv2.VF{
				PCIAddress: vf,
			}

			vfInfo.Driver, vfInfo.DeviceID = getVFDeviceInfo(log, pciInfo, device.Address, vf)

			acc.VFs = append(acc.VFs, vfInfo)
		}

		accelerators.SriovAccelerators = append(accelerators.SriovAccelerators, acc)
	}

	return accelerators, nil
}

func VrbGetSriovInventory(log *logrus.Logger) (*vrbv1.NodeInventory, error) {
	pciInfo, err := ghw.PCI()
	if err != nil {
		log.WithError(err).Error("failed to get PCI info")
		return nil, err
	}

	devices := pciInfo.ListDevices()
	if len(devices) == 0 {
		log.Info("got 0 pci devices")
		err := errors.New("pci.ListDevices() returned 0 devices")
		return nil, err
	}

	accelerators := &vrbv1.NodeInventory{
		SriovAccelerators: []vrbv1.SriovAccelerator{},
	}

	for _, device := range commonUtils.Filter(devices, VrbisKnownDevice) {
		if !utils.IsSriovPF(device.Address) {
			log.WithField("pci", device.Address).Info("ignoring non SriovPF capable device")
			continue
		}

		driver, err := utils.GetDriverName(device.Address)
		if err != nil {
			driver = ""
			if strings.Contains(err.Error(), "no such file or directory") {
				log.WithField("pci", device.Address).WithField("reason", err.Error()).Debug("driver link does not exist for device")
			} else {
				log.WithField("pci", device.Address).WithField("reason", err.Error()).Info("unable to get driver for device")
			}
		}

		acc := vrbv1.SriovAccelerator{
			VendorID:   device.Vendor.ID,
			DeviceID:   device.Product.ID,
			PCIAddress: device.Address,
			PFDriver:   driver,
			MaxVFs:     utils.GetSriovVFcapacity(device.Address),
			VFs:        []vrbv1.VF{},
		}

		vfs, err := utils.GetVFList(device.Address)
		if err != nil {
			log.WithError(err).WithField("pci", device.Address).Error("failed to get list of VFs for device")
		}

		for _, vf := range vfs {
			vfInfo := vrbv1.VF{
				PCIAddress: vf,
			}

			vfInfo.Driver, vfInfo.DeviceID = getVFDeviceInfo(log, pciInfo, device.Address, vf)

			acc.VFs = append(acc.VFs, vfInfo)
		}

		accelerators.SriovAccelerators = append(accelerators.SriovAccelerators, acc)
	}

	return accelerators, nil
}

func isKnownDevice(device *pci.Device) bool {
	_, hasKnownVendor := supportedAccelerators.VendorID[device.Vendor.ID]
	_, hasKnownDeviceID := supportedAccelerators.Devices[device.Product.ID]

	return hasKnownVendor &&
		hasKnownDeviceID &&
		device.Class.ID == supportedAccelerators.Class &&
		device.Subclass.ID == supportedAccelerators.SubClass
}

func VrbisKnownDevice(device *pci.Device) bool {
	_, hasKnownVendor := VrbsupportedAccelerators.VendorID[device.Vendor.ID]
	_, hasKnownDeviceID := VrbsupportedAccelerators.Devices[device.Product.ID]

	return hasKnownVendor &&
		hasKnownDeviceID &&
		device.Class.ID == VrbsupportedAccelerators.Class &&
		device.Subclass.ID == VrbsupportedAccelerators.SubClass
}
