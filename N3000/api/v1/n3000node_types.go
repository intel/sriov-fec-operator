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

// N3000NodeSpec defines the desired state of N3000Node
type N3000NodeSpec struct {
	FPGA      []N3000Fpga    `json:"fpga,omitempty"`
	Fortville N3000Fortville `json:"fortville,omitempty"`
	// +kubebuilder:validation:Optional
	DryRun bool `json:"dryrun,omitempty"`
}

// N3000NodeStatus defines the observed state of N3000Node
type N3000NodeStatus struct {
	Conditions []metav1.Condition     `json:"conditions,omitempty"`
	FPGA       []N3000FpgaStatus      `json:"fpga,omitempty"`
	Fortville  []N3000FortvilleStatus `json:"fortville,omitempty"`
}

type N3000FpgaStatus struct {
	PciAddr          string `json:"pciAddr,omitempty"`
	DeviceID         string `json:"deviceOd,omitempty"`
	BitstreamID      string `json:"bitstreamId,omitempty"`
	BitstreamVersion string `json:"bitstreamVersion,omitempty"`
	BootPage         string `json:"bootPage,omitempty"`
	NumaNode         int    `json:"numaNode,omitempty"`
}

type N3000FortvilleStatus struct {
	Name    string                        `json:"name,omitempty"`
	PciAddr string                        `json:"pciAddr,omitempty"`
	Modules []N3000FortvilleStatusModules `json:"modules,omitempty"`
	MAC     string                        `json:"mac,omitempty"`
	SAN     string                        `json:"san,omitempty"`
}

type N3000FortvilleStatusModules struct {
	Type    string `json:"type,omitempty"`
	Version string `json:"version,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Flashed",type=string,JSONPath=`.status.conditions[?(@.type=="Flashed")].status`

// N3000Node is the Schema for the n3000nodes API
type N3000Node struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   N3000NodeSpec   `json:"spec,omitempty"`
	Status N3000NodeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// N3000NodeList contains a list of N3000Node
type N3000NodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []N3000Node `json:"items"`
}

func init() {
	SchemeBuilder.Register(&N3000Node{}, &N3000NodeList{})
}
