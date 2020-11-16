/*
Copyright 2020.

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

// ZooKeeperClusterSpec defines the desired state of ZooKeeperCluster
type ZooKeeperClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Minimum=3
	// +kubebuilder:validation:Maximum=255
	Replicas int `json:"replicas,omitempty"`
}

// ZooKeeperClusterStatus defines the observed state of ZooKeeperCluster
type ZooKeeperClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Nodes []string `json:"nodes,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName="zkc"

// ZooKeeperCluster is the Schema for the zookeeperclusters API
type ZooKeeperCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZooKeeperClusterSpec   `json:"spec,omitempty"`
	Status ZooKeeperClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZooKeeperClusterList contains a list of ZooKeeperCluster
type ZooKeeperClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZooKeeperCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ZooKeeperCluster{}, &ZooKeeperClusterList{})
}
