// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2025 Intel Corporation

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VF struct {
	PCIAddress string `json:"pciAddress"`
	Driver     string `json:"driver"`
	DeviceID   string `json:"deviceID"`
}

type SriovAccelerator struct {
	VendorID   string `json:"vendorID"`
	DeviceID   string `json:"deviceID"`
	PCIAddress string `json:"pciAddress"`
	PFDriver   string `json:"driver"`
	MaxVFs     int    `json:"maxVirtualFunctions"`
	VFs        []VF   `json:"virtualFunctions"`
}

type NodeInventory struct {
	SriovAccelerators []SriovAccelerator `json:"sriovAccelerators,omitempty"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SriovVrbNodeConfigSpec defines the desired state of SriovVrbNodeConfig
type SriovVrbNodeConfigSpec struct {
	// List of PhysicalFunctions configs
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	PhysicalFunctions []PhysicalFunctionConfigExt `json:"physicalFunctions"`

	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// Skips drain process when true; default false. Should be true if operator is running on SNO
	DrainSkip bool `json:"drainSkip,omitempty"`
}

// SriovVrbNodeConfigStatus defines the observed state of SriovVrbNodeConfig
type SriovVrbNodeConfigStatus struct {
	PfBbConfVersion string `json:"pfBbConfVersion,omitempty"`
	// Provides information about device update status
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Provides information about FPGA inventory on the node
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Inventory NodeInventory `json:"inventory,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Configured",type=string,JSONPath=`.status.conditions[?(@.type=="Configured")].reason`
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=svnc

// SriovVrbNodeConfig is the Schema for the SriovVrbNodeConfigs API
type SriovVrbNodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SriovVrbNodeConfigSpec   `json:"spec,omitempty"`
	Status SriovVrbNodeConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SriovVrbNodeConfigList contains a list of SriovVrbNodeConfig
type SriovVrbNodeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SriovVrbNodeConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SriovVrbNodeConfig{}, &SriovVrbNodeConfigList{})
}
