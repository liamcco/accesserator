/*
Copyright 2025.

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

package v1alpha

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecurityConfigSpec defines the desired state of SecurityConfig.
type SecurityConfigSpec struct {
	// Tokenx indicates whether a sidecar (called Texas) is started with the application referred to by `applicationRef`
	// that provides an endpoint which is available to the application on the env var TEXAS_URL.
	// The endpoint conforms to the OAuth 2.0 Token Exchange standard (RFC 8693).
	// accessPolicies in the Application manifest of the application referred to by applicationRef
	// will be used to restrict which applications can exchange tokens where the specified application is the intended audience.
	//
	// +kubebuilder:validation:Optional
	Tokenx *TokenXSpec `json:"tokenx,omitempty"`

	// ApplicationRef is a reference to the name of the SKIP application for which this SecurityConfig applies.
	//
	// +kubebuilder:validation:Required
	ApplicationRef string `json:"applicationRef,omitempty"`
}

// TokenXSpec defines the configuration for token exchange sidecar.
//
// +kubebuilder:object:generate=true
type TokenXSpec struct {
	// Enabled indicates whether the TokenX sidecar should be included for the application.
	//
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`
}

// SecurityConfigStatus defines the observed state of SecurityConfig.
type SecurityConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the SecurityConfig resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SecurityConfig is the Schema for the securityconfigs API
type SecurityConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of SecurityConfig
	// +required
	Spec SecurityConfigSpec `json:"spec"`

	// status defines the observed state of SecurityConfig
	// +optional
	Status SecurityConfigStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SecurityConfigList contains a list of SecurityConfig
type SecurityConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []SecurityConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecurityConfig{}, &SecurityConfigList{})
}
