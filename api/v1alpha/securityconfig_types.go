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

	// +kubebuilder:validation:Optional
	Opa *OpaSpec `json:"opa,omitempty"`

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

// OpaSpec defines the configuration for OPA sidecar.
//
// +kubebuilder:object:generate=true
type OpaSpec struct {
	// Enabled indicates whether the OPA sidecar should be included for the application.
	//
	// +kubebuilder:validation:Required
	Enabled           bool              `json:"enabled"`
	GithubCredentials GithubCredentials `json:"githubCredentials,omitempty"`
	BundlePublicKey   string            `json:"bundlePublicKey,omitempty"`

	// BundleResource is the OCI bundle reference, e.g. ghcr.io/org/opa-bundle:latest.
	BundleResource string `json:"bundleResource,omitempty"`
}

type GithubCredentials struct {
	ClientTokenKey string `json:"clientTokenKey"` // --> "github_token"
	ClientTokenRef string `json:"clientTokenRef"` // --> "opa-github-secret"
}

// SecurityConfigStatus defines the observed state of SecurityConfig.
type SecurityConfigStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	Phase              Phase              `json:"phase,omitempty"`
	Message            string             `json:"message,omitempty"`
	Ready              bool               `json:"ready"`
}

type Phase string

const (
	PhasePending Phase = "Pending"
	PhaseReady   Phase = "Ready"
	PhaseFailed  Phase = "Failed"
	PhaseInvalid Phase = "Invalid"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`

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

func (s *SecurityConfig) InitializeStatus() {
	if s.Status.Conditions == nil {
		s.Status.Conditions = []metav1.Condition{}
	}
	s.Status.ObservedGeneration = s.GetGeneration()
	s.Status.Ready = false
	s.Status.Phase = PhasePending
}

func (s *SecurityConfigStatus) SetPhaseInvalid(msg string) {
	s.Phase = PhaseInvalid
	s.Ready = false
	s.Message = msg
}

func SetConditionInvalid(cond *metav1.Condition, msg string) {
	cond.Status = metav1.ConditionFalse
	cond.Reason = "InvalidConfiguration"
	cond.Message = msg
}

func (s *SecurityConfigStatus) SetPhasePending(msg string) {
	s.Phase = PhasePending
	s.Ready = false
	s.Message = msg
}

func SetConditionPending(cond *metav1.Condition, msg string) {
	cond.Status = metav1.ConditionUnknown
	cond.Reason = "ReconciliationPending"
	cond.Message = msg
}

func (s *SecurityConfigStatus) SetPhaseFailed(msg string) {
	s.Phase = PhaseFailed
	s.Ready = false
	s.Message = msg
}

func SetConditionFailed(cond *metav1.Condition, msg string) {
	cond.Status = metav1.ConditionFalse
	cond.Reason = "ReconciliationFailed"
	cond.Message = msg
}

func (s *SecurityConfigStatus) SetPhaseReady(msg string) {
	s.Phase = PhaseReady
	s.Ready = true
	s.Message = msg
}

func SetConditionReady(cond *metav1.Condition, msg string) {
	cond.Status = metav1.ConditionTrue
	cond.Reason = "ReconciliationSuccess"
	cond.Message = msg
}
