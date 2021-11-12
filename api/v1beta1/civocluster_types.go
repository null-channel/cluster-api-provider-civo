/*
Copyright 2021.

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

package v1beta1

import (
	"github.com/civo/civogo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/api/v1beta1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CivoClusterSpec defines the desired state of CivoCluster
type CivoClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	ControlPlaneEndpoint v1beta1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`

	// Config represents Civo Kubernetes cluster config
	Config civogo.KubernetesClusterConfig `json:"config,omitempty"`
}

// CivoClusterStatus defines the observed state of CivoCluster
type CivoClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Ready denotes that the cluster (infrastructure) is ready.
	// +optional
	Ready bool `json:"ready"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CivoCluster is the Schema for the civoclusters API
type CivoCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CivoClusterSpec   `json:"spec,omitempty"`
	Status CivoClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CivoClusterList contains a list of CivoCluster
type CivoClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CivoCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CivoCluster{}, &CivoClusterList{})
}
