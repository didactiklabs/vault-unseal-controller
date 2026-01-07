/*
Copyright 2026.

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

const (
	// Unsealed: vault is unsealed, everything is ok -> check periodicaly if vault is unseal or not
	StatusUnsealed = "UNSEALED"
	// Changing: vault is in seal state -> launch unseal process
	StatusUnsealing = "UNSEALING"
	// Cleaning: remove job from cluster and wait for next seal
	StatusCleaning = "CLEANING"
)

// SecretRef contains information to locate a secret
type SecretRef struct {
	// Name of the secret
	//+kubebuilder:validation:Required
	Name string `json:"name"`
	// Namespace of the secret
	//+kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

// UnsealSpec defines the desired state of Unseal
type UnsealSpec struct {
	// An array of vault instances to call api endpoints for unseal, example for one instance: https://myvault01.domain.local:8200
	//+kubebuilder:validation:Required
	VaultNodes []string `json:"vaultNodes"`
	// Reference to secret containing threshold keys. Threshold keys are unseal keys required to unseal your vault instance(s)
	//+kubebuilder:validation:Required
	ThresholdKeysSecretRef SecretRef `json:"thresholdKeysSecretRef"`
	// Reference to secret containing CA certificate for validating requests against vault instances (need to be in the same namespace as the threshold keys secret)
	//+optional
	CaCertSecret string `json:"caCertSecret,omitempty"`
	// Boolean to define if you want to skip tls certificate validation. Set true of false (default is false)
	//+kubebuilder:default:=false
	TlsSkipVerify bool `json:"tlsSkipVerify,omitempty"`
	// Number of retry, default is 3
	//+kubebuilder:default:=3
	RetryCount int32 `json:"retryCount,omitempty"`
}

// UnsealStatus defines the observed state of Unseal.
type UnsealStatus struct {

	// Status of the vault
	VaultStatus string `json:"vaultStatus,omitempty"`
	// Sealed nodes
	SealedNodes []string           `json:"sealedNodes,omitempty"`
	Conditions  []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name=Age,type=date
//+kubebuilder:printcolumn:JSONPath=".status.vaultStatus",name=Vault Status,type=string
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// Unseal is the Schema for the unseals API
type Unseal struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Unseal
	// +required
	Spec UnsealSpec `json:"spec"`

	// status defines the observed state of Unseal
	// +optional
	Status UnsealStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// UnsealList contains a list of Unseal
type UnsealList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Unseal `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Unseal{}, &UnsealList{})
}
