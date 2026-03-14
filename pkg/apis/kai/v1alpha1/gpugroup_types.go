// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GPUGroupPhase string

const (
	GPUGroupPhaseAccepted  GPUGroupPhase = "Accepted"
	GPUGroupPhaseAllocated GPUGroupPhase = "Allocated"
	GPUGroupPhaseFailed    GPUGroupPhase = "Failed"
)

// GPUGroupSpec defines the desired state of GPUGroup
type GPUGroupSpec struct {
	// Number of GPUs to allocate for this GPUGroup.
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="gpuCount is immutable"
	GPUCount int32 `json:"gpuCount"`

	// Maximum number of Pods that can be attached to this GPUGroup.
	// Nil means unlimited. Can only be increased after creation.
	// +optional
	// +kubebuilder:validation:Minimum=1
	MaxAttachedPods *int32 `json:"maxAttachedPods,omitempty"`
}

// GPUGroupStatus defines the observed state of GPUGroup
type GPUGroupStatus struct {
	// Phase of this GPUGroup
	// +optional
	Phase GPUGroupPhase `json:"phase,omitempty"`

	// Human-readable message explaining what caused the current phase
	// +optional
	PhaseMessage string `json:"phaseMessage,omitempty"`

	// Node name that GPUs for this group were allocated from
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// UUIDs of the allocated GPUs within the node
	// +optional
	GPUSUUIDs []string `json:"gpusUUIDs,omitempty"`

	// Names of Pods that have this GPUGroup's GPUs attached
	// +optional
	AttachedPodsNames []string `json:"attachedPodsNames,omitempty"`

	// Unique member IDs of Pods that have this GPUGroup's GPUs attached
	// +optional
	UniqueMemberIDs []string `json:"uniqueMemberIDs,omitempty"`

	// Conditions represent the latest available observations of the GPUGroup's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Node",type=string,JSONPath=`.status.nodeName`
// +kubebuilder:printcolumn:name="GPUs",type=integer,JSONPath=`.spec.gpuCount`

// GPUGroup is the Schema for the gpugroups API
type GPUGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GPUGroupSpec   `json:"spec,omitempty"`
	Status GPUGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GPUGroupList contains a list of GPUGroup
type GPUGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GPUGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GPUGroup{}, &GPUGroupList{})
}
