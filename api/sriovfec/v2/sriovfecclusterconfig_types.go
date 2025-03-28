// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2025 Intel Corporation

package v2

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SyncStatus string

const (
	acc100maxQueueGroups = 8
	acc200maxQueueGroups = 16
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

func (udq *UplinkDownlinkQueues) String() string {
	return fmt.Sprintf("%d,%d,%d,%d,%d,%d,%d,%d", udq.VF0, udq.VF1, udq.VF2, udq.VF3,
		udq.VF4, udq.VF5, udq.VF6, udq.VF7)
}

type UplinkDownlinkQueues struct {
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32
	VF0 int `json:"vf0"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32
	VF1 int `json:"vf1"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32
	VF2 int `json:"vf2"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32
	VF3 int `json:"vf3"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32
	VF4 int `json:"vf4"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32
	VF5 int `json:"vf5"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32
	VF6 int `json:"vf6"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32
	VF7 int `json:"vf7"`
}

type UplinkDownlink struct {
	// +kubebuilder:validation:Minimum=0
	Bandwidth int `json:"bandwidth"`
	// +kubebuilder:validation:Minimum=0
	LoadBalance int                  `json:"loadBalance"`
	Queues      UplinkDownlinkQueues `json:"queues"`
}

// N3000BBDevConfig specifies variables to configure N3000 with
type N3000BBDevConfig struct {
	// +kubebuilder:validation:Enum=FPGA_5GNR;FPGA_LTE
	NetworkType string `json:"networkType"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:false
	// +kubebuilder:validation:Enum=false
	PFMode bool `json:"pfMode,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	FLRTimeOut int            `json:"flrTimeout"`
	Downlink   UplinkDownlink `json:"downlink"`
	Uplink     UplinkDownlink `json:"uplink"`
}

type QueueGroupConfig struct {
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=16
	NumQueueGroups int `json:"numQueueGroups"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=16
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
	// +kubebuilder:validation:Maximum=16
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

func (in *ACC100BBDevConfig) Validate() error {
	totalQueueGroups := in.Uplink4G.NumQueueGroups + in.Downlink4G.NumQueueGroups + in.Uplink5G.NumQueueGroups + in.Downlink5G.NumQueueGroups
	if totalQueueGroups > acc100maxQueueGroups {
		return fmt.Errorf("total number of requested queue groups (4G/5G) %v exceeds the maximum (%d)", totalQueueGroups, acc100maxQueueGroups)
	}
	return nil
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

// ACC200BBDevConfig specifies variables to configure ACC200 with
type ACC200BBDevConfig struct {
	ACC100BBDevConfig `json:",inline"`
	QFFT              QueueGroupConfig `json:"qfft"`
	FFTLut            FFTLutParam      `json:"fftLut,omitempty"`
}

func (in *ACC200BBDevConfig) Validate() error {
	totalQueueGroups := in.Uplink4G.NumQueueGroups + in.Downlink4G.NumQueueGroups + in.Uplink5G.NumQueueGroups + in.Downlink5G.NumQueueGroups + in.QFFT.NumQueueGroups
	if totalQueueGroups > acc200maxQueueGroups {
		return fmt.Errorf("total number of requested queue groups (4G/5G/QFFT) %v exceeds the maximum (%d)", totalQueueGroups, acc200maxQueueGroups)
	}
	return nil
}

// BBDevConfig is a struct containing configuration for various FEC cards
type BBDevConfig struct {
	N3000  *N3000BBDevConfig  `json:"n3000,omitempty"`
	ACC100 *ACC100BBDevConfig `json:"acc100,omitempty"`
	ACC200 *ACC200BBDevConfig `json:"acc200,omitempty"`
}

type validator interface {
	Validate() error
}

func (in *BBDevConfig) Validate() error {

	if err := hasAmbiguousBBDevConfigs(*in); err != nil {
		return err
	}

	for _, config := range []interface{}{in.ACC200, in.ACC100, in.N3000} {
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
}

// SriovFecClusterConfigSpec defines the desired state of SriovFecClusterConfig
type SriovFecClusterConfigSpec struct {

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

// SriovFecClusterConfigStatus defines the observed state of SriovFecClusterConfig
type SriovFecClusterConfigStatus struct {
	// Indicates the synchronization status of the CR
	// +operator-sdk:csv:customresourcedefinitions:type=status
	SyncStatus    SyncStatus `json:"syncStatus,omitempty"`
	LastSyncError string     `json:"lastSyncError,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=sfcc

// SriovFecClusterConfig is the Schema for the sriovfecclusterconfigs API
// +operator-sdk:csv:customresourcedefinitions:displayName="SriovFecClusterConfig",resources={{SriovFecNodeConfig,v2,node}}
type SriovFecClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SriovFecClusterConfigSpec   `json:"spec,omitempty"`
	Status SriovFecClusterConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SriovFecClusterConfigList contains a list of SriovFecClusterConfig
type SriovFecClusterConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SriovFecClusterConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SriovFecClusterConfig{}, &SriovFecClusterConfigList{})
}
