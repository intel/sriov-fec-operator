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

type N3000Fpga struct {
	// +kubebuilder:validation:Pattern=[a-zA-Z0-9\.\-\/]+
	UserImageURL string `json:"userImageURL"`
	// +kubebuilder:validation:Pattern=`[a-fA-F0-9]{4}:[a-fA-F0-9]{2}:[a-fA-F0-9]{2}\.[0-9]`
	PCIAddr string `json:"PCIAddr"`
}

type N3000Fortville struct {
	// +kubebuilder:validation:Pattern=[a-zA-Z0-9\.\-\/]+
	FirmwareURL string `json:"firmwareURL"`
	// +kubebuilder:validation:Enum=inventory;update
	Command string         `json:"command"`
	MACs    []FortvilleMAC `json:"macs"`
}

type FortvilleMAC struct {
	// +kubebuilder:validation:Pattern=`[A-F0-9]{12}`
	MAC string `json:"mac"`
}

type N3000Node struct {
	// +kubebuilder:validation:Pattern=[a-z0-9\.\-]+
	NodeName  string         `json:"nodeName"`
	FPGA      []N3000Fpga    `json:"fpga,omitempty"`
	Fortville N3000Fortville `json:"fortville,omitempty"`
}

// N3000Spec defines the desired state of N3000
type N3000Spec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Nodes []N3000Node `json:"nodes"`
}

// N3000Status defines the observed state of N3000
type N3000Status struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	State string `json:"state"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// N3000 is the Schema for the n3000s API
type N3000 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   N3000Spec   `json:"spec,omitempty"`
	Status N3000Status `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// N3000List contains a list of N3000
type N3000List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []N3000 `json:"items"`
}

func init() {
	SchemeBuilder.Register(&N3000{}, &N3000List{})
}