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

	// ID represents cluster identifier
	// +optional
	ID     *string                     `json:"id,omitempty"`
	Config CivoKubernetesClusterConfig `json:"config,omitempty"`
}

// KubernetesClusterConfig is used to create a new cluster
type CivoKubernetesClusterConfig struct {
	Name              string                            `json:"name,omitempty"`
	Region            string                            `json:"region,omitempty"`
	NumTargetNodes    int                               `json:"num_target_nodes,omitempty"`
	TargetNodesSize   string                            `json:"target_nodes_size,omitempty"`
	KubernetesVersion string                            `json:"kubernetes_version,omitempty"`
	NodeDestroy       string                            `json:"node_destroy,omitempty"`
	NetworkID         string                            `json:"network_id,omitempty"`
	Tags              string                            `json:"tags,omitempty"`
	Pools             []CivoKubernetesClusterPoolConfig `json:"pools,omitempty"`
	Applications      string                            `json:"applications,omitempty"`
	InstanceFirewall  string                            `json:"instance_firewall,omitempty"`
	FirewallRule      string                            `json:"firewall_rule,omitempty"`
}

//KubernetesClusterPoolConfig is used to create a new cluster pool
type CivoKubernetesClusterPoolConfig struct {
	ID    string `json:"id,omitempty"`
	Count int    `json:"count,omitempty"`
	Size  string `json:"size,omitempty"`
}

func ToCivoPoolsStruct(config []CivoKubernetesClusterPoolConfig) []civogo.KubernetesClusterPoolConfig {
	s := make([]civogo.KubernetesClusterPoolConfig, len(config))

	for _, c := range config {
		s = append(s, civogo.KubernetesClusterPoolConfig{ID: c.ID, Count: c.Count, Size: c.Size})
	}

	return s
}

func ToCivoKubernetesStruct(config *CivoKubernetesClusterConfig) *civogo.KubernetesClusterConfig {
	return &civogo.KubernetesClusterConfig{
		Name:             config.Name,
		Region:           config.Region,
		NumTargetNodes:   config.NumTargetNodes,
		TargetNodesSize:  config.KubernetesVersion,
		NodeDestroy:      config.NodeDestroy,
		NetworkID:        config.NetworkID,
		Tags:             config.Tags,
		Pools:            ToCivoPoolsStruct(config.Pools),
		Applications:     config.Applications,
		InstanceFirewall: config.InstanceFirewall,
		FirewallRule:     config.FirewallRule,
	}
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
