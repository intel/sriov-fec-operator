// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2022 Intel Corporation

package v2

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
)

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
	return s.isVendorMatching(a) && s.isPciAddressMatching(a) &&
		s.isPFDriverMatching(a) && s.isMaxVFsMatching(a) && s.isDeviceIDMatching(a)
}

func (s AcceleratorSelector) isVendorMatching(a SriovAccelerator) bool {
	return s.VendorID == "" || s.VendorID == a.VendorID
}

func (s AcceleratorSelector) isPciAddressMatching(a SriovAccelerator) bool {
	return s.PCIAddress == "" || s.PCIAddress == a.PCIAddress
}

func (s AcceleratorSelector) isPFDriverMatching(a SriovAccelerator) bool {
	return s.PFDriver == "" || s.PFDriver == a.PFDriver
}

func (s AcceleratorSelector) isMaxVFsMatching(a SriovAccelerator) bool {
	return s.MaxVFs == 0 || s.MaxVFs == a.MaxVFs
}

func (s AcceleratorSelector) isDeviceIDMatching(a SriovAccelerator) bool {
	return s.DeviceID == "" || s.DeviceID == a.DeviceID
}

func (in *SriovFecNodeConfig) FindCondition(conditionType string) *metav1.Condition {
	return meta.FindStatusCondition(in.Status.Conditions, conditionType)
}

func isNil(v interface{}) bool {
	return v == nil || (reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil())
}
