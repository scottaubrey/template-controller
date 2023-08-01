/*
Copyright 2022.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ListGithubDeploymentsSpec defines the desired state of ListGithubDeployments
type ListGithubDeploymentsSpec struct {
	// Interval is the interval at which to query the Gitlab API.
	// Defaults to 5m.
	// +optional
	// +kubebuilder:default:="5m"
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	Interval metav1.Duration `json:"interval"`

	GithubProject `json:",inline"`

	// Head specifies the head to filter for
	// +optional
	Ref *string `json:"head,omitempty"`

	// Head specifies the head to filter for
	// +optional
	Sha *string `json:"head,omitempty"`

	// Base specifies the base to filter for
	// +optional
	Task *string `json:"base,omitempty"`

	// Base specifies the base to filter for
	// +optional
	Environment *string `json:"base,omitempty"`

	// State is an additional PR filter to get only those with a certain state. Default: "all"
	// +optional
	// +kubebuilder:validation:Enum=all;open;closed
	// +kubebuilder:default:="all"
	State string `json:"state,omitempty"`

	// Limit limits the maximum number of deployments to fetch. Defaults to 100
	// +kubebuilder:default:=100
	Limit int `json:"limit"`
}

// ListGithubDeploymentsStatus defines the observed state of ListGithubDeployments
type ListGithubDeploymentsStatus struct {
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Deployments []runtime.RawExtension `json:"deployments,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ListGithubDeployments is the Schema for the listgithubdeployments API
type ListGithubDeployments struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ListGithubDeploymentsSpec   `json:"spec,omitempty"`
	Status ListGithubDeploymentsStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ListGithubDeploymentsList contains a list of ListGithubDeployments
type ListGithubDeploymentsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ListGithubDeployments `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ListGithubDeployments{}, &ListGithubDeploymentsList{})
}
