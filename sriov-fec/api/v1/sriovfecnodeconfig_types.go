// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type VF struct {
	PCIAddress string `json:"pciAddress"`
	Driver     string `json:"driver"`
	DeviceID   string `json:"deviceID"`
}

type SriovAccelerator struct {
	VendorID   string `json:"vendorID"`
	DeviceID   string `json:"deviceID"`
	PCIAddress string `json:"pciAddress"`
	Driver     string `json:"driver"`
	MaxVFs     int    `json:"maxVirtualFunctions"`
	VFs        []VF   `json:"virtualFunctions"`
}

type NodeInventory struct {
	SriovAccelerators []SriovAccelerator `json:"sriovAccelerators,omitempty"`
}

// SriovFecNodeConfigSpec defines the desired state of SriovFecNodeConfig
type SriovFecNodeConfigSpec struct {
	// +kubebuilder:validation:Required

	// List of PhysicalFunctions configs
	PhysicalFunctions []PhysicalFunctionConfig `json:"physicalFunctions"`
	// +kubebuilder:validation:Optional
	DrainSkip bool `json:"drainSkip,omitempty"`
}

// SriovFecNodeConfigStatus defines the observed state of SriovFecNodeConfig
type SriovFecNodeConfigStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	Inventory  NodeInventory      `json:"inventory,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Configured",type=string,JSONPath=`.status.conditions[?(@.type=="Configured")].status`

// SriovFecNodeConfig is the Schema for the sriovfecnodeconfigs API
type SriovFecNodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SriovFecNodeConfigSpec   `json:"spec,omitempty"`
	Status SriovFecNodeConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SriovFecNodeConfigList contains a list of SriovFecNodeConfig
type SriovFecNodeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SriovFecNodeConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SriovFecNodeConfig{}, &SriovFecNodeConfigList{})
}
