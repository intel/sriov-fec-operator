// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2023 Intel Corporation

package daemon

import (
	"errors"

	commonUtils "github.com/smart-edge-open/sriov-fec-operator/pkg/common/utils"
	"github.com/jaypipes/ghw/pkg/pci"
	"github.com/sirupsen/logrus"

	sriovv2 "github.com/smart-edge-open/sriov-fec-operator/api/sriovfec/v2"
	vrbv1 "github.com/smart-edge-open/sriov-fec-operator/api/sriovvrb/v1"
	"github.com/jaypipes/ghw"
	"github.com/k8snetworkplumbingwg/sriov-network-device-plugin/pkg/utils"
)

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
			log.WithField("pci", device.Address).WithField("reason", err.Error()).Info("unable to get driver for device")
			driver = ""
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

			driver, err := utils.GetDriverName(vf)
			if err != nil {
				log.WithFields(logrus.Fields{
					"pci":    vf,
					"pf":     device.Address,
					"reason": err.Error(),
				}).Info("failed to get driver name for VF")
			} else {
				vfInfo.Driver = driver
			}

			if vfDeviceInfo := pciInfo.GetDevice(vf); vfDeviceInfo == nil {
				log.WithField("pci", vf).Info("failed to get device info for vf")
			} else {
				vfInfo.DeviceID = vfDeviceInfo.Product.ID
			}

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
			log.WithField("pci", device.Address).WithField("reason", err.Error()).Info("unable to get driver for device")
			driver = ""
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

			driver, err := utils.GetDriverName(vf)
			if err != nil {
				log.WithFields(logrus.Fields{
					"pci":    vf,
					"pf":     device.Address,
					"reason": err.Error(),
				}).Info("failed to get driver name for VF")
			} else {
				vfInfo.Driver = driver
			}

			if vfDeviceInfo := pciInfo.GetDevice(vf); vfDeviceInfo == nil {
				log.WithField("pci", vf).Info("failed to get device info for vf")
			} else {
				vfInfo.DeviceID = vfDeviceInfo.Product.ID
			}

			acc.VFs = append(acc.VFs, vfInfo)
		}

		accelerators.SriovAccelerators = append(accelerators.SriovAccelerators, acc)
	}

	return accelerators, nil
}

func isKnownDevice(device *pci.Device) bool {
	_, hasKnownVendor := supportedAccelerators.VendorID[device.Vendor.ID]
	_, hasKnownDeviceId := supportedAccelerators.Devices[device.Product.ID]

	return hasKnownVendor &&
		hasKnownDeviceId &&
		device.Class.ID == supportedAccelerators.Class &&
		device.Subclass.ID == supportedAccelerators.SubClass
}

func VrbisKnownDevice(device *pci.Device) bool {
	_, hasKnownVendor := VrbsupportedAccelerators.VendorID[device.Vendor.ID]
	_, hasKnownDeviceId := VrbsupportedAccelerators.Devices[device.Product.ID]

	return hasKnownVendor &&
		hasKnownDeviceId &&
		device.Class.ID == VrbsupportedAccelerators.Class &&
		device.Subclass.ID == VrbsupportedAccelerators.SubClass
}
