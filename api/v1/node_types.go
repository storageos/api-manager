/*
MIT License

Copyright (c) 2021 StorageOS

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "make" to regenerate code after modifying this file

const (
	NodeHealthOnline  NodeHealth = "online"
	NodeHealthOffline NodeHealth = "offline"
	NodeHealthUnknown NodeHealth = "unknown"
)

// NodeHealth is a string representation of the node health.
type NodeHealth string

// NodeSpec defines the desired state of Node.
type NodeSpec struct {
	// Endpoint at which we operate our dataplane's dfs service. (used for IO
	// operations) This value is set on startup by the corresponding environment
	// variable (IO_ADVERTISE_ADDRESS).
	IoEndpoint string `json:"ioEndpoint,omitempty"`

	// Endpoint at which we operate our dataplane's supervisor service (used for
	// sync). This value is set on startup by the corresponding environment
	// variable (SUPERVISOR_ADVERTISE_ADDRESS).
	SupervisorEndpoint string `json:"supervisorEndpoint,omitempty"`

	// Endpoint at which we operate our health checking service. This value is
	// set on startup by the corresponding environment variable
	// (GOSSIP_ADVERTISE_ADDRESS).
	GossipEndpoint string `json:"gossipEndpoint,omitempty"`

	// Endpoint at which we operate our clustering GRPC API. This value is set
	// on startup by the corresponding environment variable
	// (INTERNAL_API_ADVERTISE_ADDRESS).
	ClusteringEndpoint string `json:"clusteringEndpoint,omitempty"`
}

// NodeStatus defines the observed state of the Node.
type NodeStatus struct {
	// Health of the node.
	Health NodeHealth `json:"health,omitempty"`

	// Capacity of the node.
	Capacity CapacityStats `json:"capacity,omitempty"`
}

// CapacityStats describes the node's storage capacity.
type CapacityStats struct {
	// Total bytes in the filesystem
	Total uint64 `json:"total,omitempty"`
	// Free bytes in the filesystem available to root user
	Free uint64 `json:"free,omitempty"`
	// Byte value available to an unprivileged user
	Available uint64 `json:"available,omitempty"`
}

// +kubebuilder:object:root=true

// Node is the Schema for the nodes API.
type Node struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeSpec   `json:"spec,omitempty"`
	Status NodeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NodeList contains a list of Node.
type NodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Node `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Node{}, &NodeList{})
}
