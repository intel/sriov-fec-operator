// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2025 Intel Corporation

package v1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SyncStatus string

const (
	vrb1maxQueueGroups        = 16
	vrb2maxQueueGroups        = 32
	vrb1maxVfNums             = 16
	vrb2maxQueuesPerOperation = 256
)

var (
	// InProgressSync indicates that the synchronization of the CR is in progress
	InProgressSync SyncStatus = "InProgress"
	// SucceededSync indicates that the synchronization of the CR succeeded
	SucceededSync SyncStatus = "Succeeded"
	// FailedSync indicates that the synchronization of the CR failed
	FailedSync SyncStatus = "Failed"
	// IgnoredSync indicates that the CR is ignored
	IgnoredSync SyncStatus = "Ignored"
)

type QueueGroupConfig struct {
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32
	NumQueueGroups int `json:"numQueueGroups"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=64
	NumAqsPerGroups int `json:"numAqsPerGroups"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=12
	AqDepthLog2 int `json:"aqDepthLog2"`
}

// ACC100BBDevConfig specifies variables to configure ACC100 with
type ACC100BBDevConfig struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:false
	// +kubebuilder:validation:Enum=false
	PFMode bool `json:"pfMode,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=64
	NumVfBundles int `json:"numVfBundles"`
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=1024
	// +kubebuilder:default:1024
	// +kubebuilder:validation:Optional
	MaxQueueSize int              `json:"maxQueueSize,omitempty"`
	Uplink4G     QueueGroupConfig `json:"uplink4G"`
	Downlink4G   QueueGroupConfig `json:"downlink4G"`
	Uplink5G     QueueGroupConfig `json:"uplink5G"`
	Downlink5G   QueueGroupConfig `json:"downlink5G"`
}

// FFTLutParam specifies variables required to use custom fft bin file
type FFTLutParam struct {
	// Path to .tar.gz SRS-FFT file
	// +kubebuilder:validation:Pattern=`^((http|https)://.*\.tar\.gz)?$`
	FftUrl string `json:"fftUrl"`
	// SHA-1 checksum of .tar.gz SRS-FFT File
	// +kubebuilder:validation:Pattern=`^([a-fA-F0-9]{40})?$`
	FftChecksum string `json:"fftChecksum"`
}

// VRB1BBDevConfig specifies variables to configure ACC200 with
type VRB1BBDevConfig struct {
	ACC100BBDevConfig `json:",inline"`
	QFFT              QueueGroupConfig `json:"qfft"`
	FFTLut            FFTLutParam      `json:"fftLut,omitempty"`
}

type VRB2BBDevConfig struct {
	ACC100BBDevConfig `json:",inline"`
	QFFT              QueueGroupConfig `json:"qfft"`
	QMLD              QueueGroupConfig `json:"qmld"`
	FFTLut            FFTLutParam      `json:"fftLut,omitempty"`
}

func (in *VRB1BBDevConfig) Validate() error {
	totalQueueGroups := in.Uplink4G.NumQueueGroups + in.Downlink4G.NumQueueGroups + in.Uplink5G.NumQueueGroups + in.Downlink5G.NumQueueGroups + in.QFFT.NumQueueGroups
	if totalQueueGroups > vrb1maxQueueGroups {
		return fmt.Errorf("total number of requested queue groups (4G/5G/QFFT) %v exceeds the maximum (%d)", totalQueueGroups, vrb1maxQueueGroups)
	}
	return nil
}

func (in *VRB2BBDevConfig) Validate() error {
	totalQueueGroups := in.Uplink4G.NumQueueGroups + in.Downlink4G.NumQueueGroups + in.Uplink5G.NumQueueGroups + in.Downlink5G.NumQueueGroups + in.QFFT.NumQueueGroups + in.QMLD.NumQueueGroups
	if totalQueueGroups > vrb2maxQueueGroups {
		return fmt.Errorf("total number of requested queue groups (4G/5G/QFFT/QMLD) %v exceeds the maximum (%d)", totalQueueGroups, vrb2maxQueueGroups)
	}
	return nil
}

// BBDevConfig is a struct containing configuration for various FEC cards
type BBDevConfig struct {
	VRB1 *VRB1BBDevConfig `json:"vrb1,omitempty"`
	VRB2 *VRB2BBDevConfig `json:"vrb2,omitempty"`
}

type validator interface {
	Validate() error
}

func (in *BBDevConfig) Validate() error {

	if err := hasAmbiguousBBDevConfigs(*in); err != nil {
		return err
	}

	for _, config := range []interface{}{in.VRB1, in.VRB2} {
		if !isNil(config) {
			if validator, ok := config.(validator); ok {
				return validator.Validate()
			}
		}
	}

	return nil
}

// PhysicalFunctionConfig defines a possible configuration of a single Physical Function (PF), i.e. card
type PhysicalFunctionConfig struct {
	// PFDriver to bound the PFs to
	//+kubebuilder:validation:Pattern=`(pci-pf-stub|pci_pf_stub|igb_uio|vfio-pci)`
	PFDriver string `json:"pfDriver"`
	// VFDriver to bound the VFs to
	VFDriver string `json:"vfDriver"`
	// VFAmount is an amount of VFs to be created
	// +kubebuilder:validation:Minimum=1
	VFAmount int `json:"vfAmount"`
	// BBDevConfig is a config for PF's queues
	BBDevConfig BBDevConfig `json:"bbDevConfig"`
}

type PhysicalFunctionConfigExt struct {
	// PCIAdress is a Physical Functions's PCI address that will be configured according to this spec
	// +kubebuilder:validation:Pattern=`^[a-fA-F0-9]{4}:[a-fA-F0-9]{2}:[01][a-fA-F0-9]\.[0-7]$`
	PCIAddress string `json:"pciAddress"`

	// PFDriver to bound the PFs to
	//+kubebuilder:validation:Pattern=`(pci-pf-stub|pci_pf_stub|igb_uio|vfio-pci)`
	PFDriver string `json:"pfDriver"`

	// VFDriver to bound the VFs to
	VFDriver string `json:"vfDriver"`

	// VFAmount is an amount of VFs to be created
	// +kubebuilder:validation:Minimum=0
	VFAmount int `json:"vfAmount"`

	// BBDevConfig is a config for PF's queues
	BBDevConfig BBDevConfig `json:"bbDevConfig"`

	// VrbResourceName is optional for custom resource name for sriov-device-plugin
	VrbResourceName string `json:"vrbResourceName"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SriovVrbClusterConfigSpec defines the desired state of SriovVrbClusterConfig
type SriovVrbClusterConfigSpec struct {

	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// Selector describes target node for this spec
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// Selector describes target accelerator for this spec
	AcceleratorSelector AcceleratorSelector `json:"acceleratorSelector,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// Physical function (card) config
	PhysicalFunction PhysicalFunctionConfig `json:"physicalFunction"`

	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// Higher priority policies can override lower ones.
	Priority int `json:"priority,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// Skips drain process when true; default false. Should be true if operator is running on SNO
	DrainSkip *bool `json:"drainSkip,omitempty"`

	// Indicates custom resource name for sriov-device-plugin
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9-_]+$`
	VrbResourceName string `json:"vrbResourceName,omitempty"`
}

type AcceleratorSelector struct {
	VendorID string `json:"vendorID,omitempty"`
	DeviceID string `json:"deviceID,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^[a-fA-F0-9]{4}:[a-fA-F0-9]{2}:[01][a-fA-F0-9]\.[0-7]$`
	PCIAddress string `json:"pciAddress,omitempty"`
	//+kubebuilder:validation:Pattern=`(pci-pf-stub|pci_pf_stub|igb_uio|vfio-pci)`
	PFDriver string `json:"driver,omitempty"`
	MaxVFs   int    `json:"maxVirtualFunctions,omitempty"`
}

// SriovVrbClusterConfigStatus defines the observed state of SriovVrbClusterConfig
type SriovVrbClusterConfigStatus struct {
	// Indicates the synchronization status of the CR
	// +operator-sdk:csv:customresourcedefinitions:type=status
	SyncStatus    SyncStatus `json:"syncStatus,omitempty"`
	LastSyncError string     `json:"lastSyncError,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=svcc

// SriovVrbClusterConfig is the Schema for the SriovVrbClusterConfigs API
type SriovVrbClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SriovVrbClusterConfigSpec   `json:"spec,omitempty"`
	Status SriovVrbClusterConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SriovVrbClusterConfigList contains a list of SriovVrbClusterConfig
type SriovVrbClusterConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SriovVrbClusterConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SriovVrbClusterConfig{}, &SriovVrbClusterConfigList{})
}
