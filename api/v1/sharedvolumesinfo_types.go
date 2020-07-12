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

// SharedVolumesInfoSpec defines the desired state of SharedVolumesInfo
type SharedVolumesInfoSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Volumes []Volume `json:"volumes,omitempty"`
}

// SharedVolumesInfoStatus defines the observed state of SharedVolumesInfo
type SharedVolumesInfoStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// SharedVolumesInfo is the Schema for the sharedvolumesinfoes API
type SharedVolumesInfo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SharedVolumesInfoSpec   `json:"spec,omitempty"`
	Status SharedVolumesInfoStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SharedVolumesInfoList contains a list of SharedVolumesInfo
type SharedVolumesInfoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SharedVolumesInfo `json:"items"`
}

// Volume contains shared volume info.
type Volume struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Address   string `json:"address,omitempty"`
}

func init() {
	SchemeBuilder.Register(&SharedVolumesInfo{}, &SharedVolumesInfoList{})
}
