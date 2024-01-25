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

const (
	// LVMClusterFinalizer is the finalizer used by the cluster controller to
	// cleanup the cluster resources when a Cluster is being deleted.
	LVMClusterFinalizer = "lvmcluster.storage.domain"
)

// LVMClusterSpec defines the desired state of LVMCluster
type LVMClusterSpec struct {
	// Paused can be used to prevent controllers from processing the LVMCluster and all its associated objects.
	// +optional
	Paused bool `json:"paused,omitempty"`

	// Disks is a list of disks that will be LVM PVs.
	// +optional
	Disks []Disk `json:"disks,omitempty"`

	// VolumeGroupName is the name of the LVM VG.
	// +kubebuilder:validation:MinLength=1
	VolumeGroupName string `json:"volumeGroupName"`
}

// LVMClusterStatus defines the observed state of LVMCluster
type LVMClusterStatus struct {
	// The generation observed by the LVMCluster controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions defines current service state of the cluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// LVMCluster is the Schema for the lvmclusters API
type LVMCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LVMClusterSpec   `json:"spec,omitempty"`
	Status LVMClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LVMClusterList contains a list of LVMCluster
type LVMClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LVMCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LVMCluster{}, &LVMClusterList{})
}

// GetConditions returns the set of conditions for this object.
func (c *LVMCluster) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (c *LVMCluster) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}
