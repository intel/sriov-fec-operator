/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type UplinkDownlinkQueues struct {
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32

	VF0 int `json:"vf0,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32

	VF1 int `json:"vf1,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32

	VF2 int `json:"vf2,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32

	VF3 int `json:"vf3,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32

	VF4 int `json:"vf4,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32

	VF5 int `json:"vf5,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32

	VF6 int `json:"vf6,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=32

	VF7 int `json:"vf7,omitempty"`
}

type UplinkDownlink struct {
	// +kubebuilder:validation:Required

	Bandwidth int `json:"bandwidth"`

	// +kubebuilder:validation:Required

	LoadBalance int `json:"loadBalance"`

	// +kubebuilder:validation:Required

	Queues UplinkDownlinkQueues `json:"queues"`
}

type CardQueues struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=FPGA_5GNR;FPGA_LTE

	NetworkType string `json:"networkType"`

	// +kubebuilder:validation:Required

	PFMode bool `json:"pfMode"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0

	FLRTimeOut int `json:"flrTimeout"`

	// +kubebuilder:validation:Required

	Downlink UplinkDownlink `json:"downlink"`

	// +kubebuilder:validation:Required

	Uplink UplinkDownlink `json:"uplink"`
}

type CardConfig struct {
	// PCIAdress is a card's PCI address that will be configured according to this spec
	PCIAddress string `json:"pciAddress,omitempty"`

	// +kubebuilder:validation:Required

	// VendorID is ID of card's vendor
	VendorID string `json:"vendorID"`

	// +kubebuilder:validation:Required

	// PFDeviceID
	PFDeviceID string `json:"pfDeviceID"`

	// +kubebuilder:validation:Required

	// PFDriver to bound the PFs to
	PFDriver string `json:"pfDriver"`

	// +kubebuilder:validation:Required

	// VFDeviceID
	VFDeviceID string `json:"vfDeviceID"`

	// +kubebuilder:validation:Required

	// VFDriver to bound the VFs to
	VFDriver string `json:"vfDriver"`

	// +kubebuilder:validation:Required

	// VFAmount is an amount of VFs to be created
	VFAmount int `json:"vfAmount"`

	// +kubebuilder:validation:Required

	// QueuesConfiguration is a config for card's queues
	QueuesConfiguration CardQueues `json:"queuesConfiguration"`
}

type NodeConfig struct {
	// Name of the node
	NodeName string `json:"nodeName,omitempty"`

	// +kubebuilder:validation:Required

	// If true, then the first card config will be used for all cards.
	// pciAddress will be ignored.
	OneCardConfigForAll bool `json:"oneCardConfigForAll"`

	// +kubebuilder:validation:Required

	// List of card configs
	Cards []CardConfig `json:"cards"`
}

// SriovFecClusterConfigSpec defines the desired state of SriovFecClusterConfig
type SriovFecClusterConfigSpec struct {
	// +kubebuilder:validation:Required

	// If true, then the first node config will be used for all nodes.
	// nodeName will be ignored. First card config will be used, pciAddress will be ignored
	OneNodeConfigForAll bool `json:"oneNodeConfigForAll"`

	// +kubebuilder:validation:Required

	// List of node configurations
	Nodes []NodeConfig `json:"nodes"`
}

// SriovFecClusterConfigStatus defines the observed state of SriovFecClusterConfig
type SriovFecClusterConfigStatus struct {
	SyncStatus    string `json:"syncStatus,omitempty"`
	LastSyncError string `json:"lastSyncError,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SyncStatus",type=string,JSONPath=`.status.syncStatus`

// SriovFecClusterConfig is the Schema for the sriovfecclusterconfigs API
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
