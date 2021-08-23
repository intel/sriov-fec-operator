// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package v2

type ByPriority []SriovFecClusterConfig

func (a ByPriority) Len() int {
	return len(a)
}

func (a ByPriority) Less(i, j int) bool {
	if a[i].Spec.Priority != a[j].Spec.Priority {
		return a[i].Spec.Priority > a[j].Spec.Priority
	}
	return a[i].GetName() < a[j].GetName()
}

func (a ByPriority) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (s AcceleratorSelector) Matches(a SriovAccelerator) bool {
	if s.VendorID != "" && s.VendorID != a.VendorID {
		return false
	}
	if s.PCIAddress != "" && s.PCIAddress != a.PCIAddress {
		return false
	}
	if s.PFDriver != "" && s.PFDriver != a.PFDriver {
		return false
	}
	if s.MaxVFs != 0 && s.MaxVFs != a.MaxVFs {
		return false
	}
	if s.DeviceID != "" && s.DeviceID != a.DeviceID {
		return false
	}

	return true
}
