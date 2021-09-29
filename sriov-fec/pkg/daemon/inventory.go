// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package daemon

import (
	"errors"

	"github.com/go-logr/logr"
	"github.com/jaypipes/ghw"
	"github.com/k8snetworkplumbingwg/sriov-network-device-plugin/pkg/utils"
	sriovv1 "github.com/smart-edge-open/openshift-operator/sriov-fec/api/v1"
)

func GetSriovInventory(log logr.Logger) (*sriovv1.NodeInventory, error) {
	pci, err := ghw.PCI()
	if err != nil {
		log.Error(err, "failed to get PCI info")
		return nil, err
	}

	devices := pci.ListDevices()
	if len(devices) == 0 {
		log.V(4).Info("got 0 pci devices")
		err := errors.New("pci.ListDevices() returned 0 devices")
		return nil, err
	}

	accelerators := &sriovv1.NodeInventory{
		SriovAccelerators: []sriovv1.SriovAccelerator{},
	}

	for _, device := range devices {

		_, isWhitelisted := supportedAccelerators.VendorID[device.Vendor.ID]
		if !(isWhitelisted &&
			device.Class.ID == supportedAccelerators.Class &&
			device.Subclass.ID == supportedAccelerators.SubClass) {
			continue
		}

		if _, ok := supportedAccelerators.Devices[device.Product.ID]; !ok {
			log.V(4).Info("ignoring unsupported device", "device.Product.ID", device.Product.ID)
			continue
		}

		if !utils.IsSriovPF(device.Address) {
			log.V(4).Info("ignoring non SriovPF capable device", "pci", device.Address)
			continue
		}

		driver, err := utils.GetDriverName(device.Address)
		if err != nil {
			log.V(4).Info("unable to get driver for device", "pci", device.Address, "reason", err.Error())
			driver = ""
		}

		acc := sriovv1.SriovAccelerator{
			VendorID:   device.Vendor.ID,
			DeviceID:   device.Product.ID,
			PCIAddress: device.Address,
			Driver:     driver,
			MaxVFs:     utils.GetSriovVFcapacity(device.Address),
			VFs:        []sriovv1.VF{},
		}

		vfs, err := utils.GetVFList(device.Address)
		if err != nil {
			log.Error(err, "failed to get list of VFs for device", "pci", device.Address)
		}

		for _, vf := range vfs {
			vfInfo := sriovv1.VF{
				PCIAddress: vf,
			}

			driver, err := utils.GetDriverName(vf)
			if err != nil {
				log.V(4).Info("failed to get driver name for VF", "pci", vf, "pf", device.Address, "reason", err.Error())
			} else {
				vfInfo.Driver = driver
			}

			if vfDeviceInfo := pci.GetDevice(vf); vfDeviceInfo == nil {
				log.V(4).Info("failed to get device info for vf", "pci", vf)
			} else {
				vfInfo.DeviceID = vfDeviceInfo.Product.ID
			}

			acc.VFs = append(acc.VFs, vfInfo)
		}

		accelerators.SriovAccelerators = append(accelerators.SriovAccelerators, acc)
	}

	return accelerators, nil
}
