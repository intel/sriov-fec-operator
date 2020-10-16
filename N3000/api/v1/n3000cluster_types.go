// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

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

type N3000ClusterSyncStatus string

var (
	// InprogressSync indicates that the Cluster in the progress of sync
	InprogressSync N3000ClusterSyncStatus = "InProgress"
	// SucceededSync indicates that the Cluster succeeded to sync
	SucceededSync N3000ClusterSyncStatus = "Succeeded"
	// FailedSync indicated that the Cluster failed to sync
	FailedSync N3000ClusterSyncStatus = "Failed"
)

type N3000Fpga struct {
	// +kubebuilder:validation:Pattern=[a-zA-Z0-9\.\-\/]+
	UserImageURL string `json:"userImageURL,omitempty"`
	// +kubebuilder:validation:Pattern=`[a-fA-F0-9]{4}:[a-fA-F0-9]{2}:[a-fA-F0-9]{2}\.[0-9]`
	PCIAddr string `json:"PCIAddr,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=[a-z0-9]+
	CheckSum string `json:"checksum,omitempty"`
}

type N3000Fortville struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=[a-zA-Z0-9\.\-\/]+
	FirmwareURL string `json:"firmwareURL,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=inventory;update
	Command string `json:"command,omitempty"`
	// +kubebuilder:validation:Optional
	MACs []FortvilleMAC `json:"macs,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=[a-z0-9]+
	CheckSum string `json:"checksum,omitempty"`
}

type FortvilleMAC struct {
	// +kubebuilder:validation:Pattern=`[A-F0-9]{12}`
	MAC string `json:"mac,omitempty"`
}

type N3000ClusterNode struct {
	// +kubebuilder:validation:Pattern=[a-z0-9\.\-]+
	NodeName  string         `json:"nodeName"`
	FPGA      []N3000Fpga    `json:"fpga,omitempty"`
	Fortville N3000Fortville `json:"fortville,omitempty"`
}

// N3000ClusterSpec defines the desired state of N3000Cluster
type N3000ClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Nodes []N3000ClusterNode `json:"nodes"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Type:=bool
	DryRun bool `json:"dryrun,omitempty"`
}

// N3000ClusterStatus defines the observed state of N3000Cluster
type N3000ClusterStatus struct {
	SyncStatus    N3000ClusterSyncStatus `json:"syncStatus,omitempty"`
	LastSyncError string                 `json:"lastSyncError,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// N3000Cluster is the Schema for the n3000clusters API
type N3000Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   N3000ClusterSpec   `json:"spec,omitempty"`
	Status N3000ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// N3000ClusterList contains a list of N3000Cluster
type N3000ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []N3000Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&N3000Cluster{}, &N3000ClusterList{})
}