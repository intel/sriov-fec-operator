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
	SriovAccelerators []SriovAccelerator `json:"sriovAccelerators"`
}

// SriovFecNodeConfigSpec defines the desired state of SriovFecNodeConfig
type SriovFecNodeConfigSpec struct {
	// +kubebuilder:validation:Required

	// If true, then the first card config will be used for all cards.
	// pciAddress will be ignored.
	OneCardConfigForAll bool `json:"oneCardConfigForAll"`

	// +kubebuilder:validation:Required

	// List of card configs
	Cards []CardConfig `json:"cards"`
}

// SriovFecNodeConfigStatus defines the observed state of SriovFecNodeConfig
type SriovFecNodeConfigStatus struct {
	SyncStatus    SyncStatus    `json:"syncStatus,omitempty"`
	LastSyncError string        `json:"lastSyncError,omitempty"`
	Inventory     NodeInventory `json:"inventory,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SyncStatus",type=string,JSONPath=`.status.syncStatus`

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
