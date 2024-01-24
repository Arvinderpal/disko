/*
Copyright 2024.

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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// LVMNodeSpec defines the desired state of LVMNode
type LVMNodeSpec struct {
}

// LVMNodeStatus defines the observed state of LVMNode
type LVMNodeStatus struct {
	// The generation observed by the LVMCluster controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions defines current service state of the cluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// LVMNode is the Schema for the lvmnodes API
type LVMNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LVMNodeSpec   `json:"spec,omitempty"`
	Status LVMNodeStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LVMNodeList contains a list of LVMNode
type LVMNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LVMNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LVMNode{}, &LVMNodeList{})
}

// GetConditions returns the set of conditions for this object.
func (c *LVMNode) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (c *LVMNode) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}
