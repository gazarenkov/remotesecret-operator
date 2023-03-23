/*
Copyright 2023.

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
)

type RemoteSecretState string

const (
	RemoteSecretInitialState      RemoteSecretState = "Initial"
	RemoteSecretDataStoredState   RemoteSecretState = "Stored"
	RemoteSecretDataInjectedState RemoteSecretState = "Injected"
	RemoteSecretInvalidState      RemoteSecretState = "Invalid"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RemoteSecretSpec defines the desired state of RemoteSecret
type RemoteSecretSpec struct {
	// +kubebuilder:validation:Required
	Type string `json:"type,omitempty"`
	// +kubebuilder:validation:Required
	ClusterName string `json:"cluster,omitempty"`

	// for manually exposing/deposing the secret (maybe to delete in future?)

	// +kubebuilder:validation:Required
	//ExposedAs bool `json:"exposed-as,omitempty"`

	// +kubebuilder:validation:Required
	Stored bool `json:"stored,omitempty"`

	ExposedAs bool `json:"exposed-as"`
	//Properties map[string]string `json:"properties,omitempty"`
}

// RemoteSecretStatus defines the observed state of RemoteSecret
type RemoteSecretStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	Phase      RemoteSecretState  `json:"phase,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RemoteSecret is the Schema for the RemoteSecrets API
type RemoteSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RemoteSecretSpec   `json:"spec,omitempty"`
	Status RemoteSecretStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RemoteSecretList contains a list of RemoteSecret
type RemoteSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteSecret `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RemoteSecret{}, &RemoteSecretList{})
}
