// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GPUGroupTemplateData defines the template for GPUGroups created from a GPUGroupTemplate
type GPUGroupTemplateData struct {
	// Metadata for GPUGroups created from this template
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec for GPUGroups created from this template
	// +required
	Spec GPUGroupSpec `json:"spec"`
}

// GPUGroupTemplateSpec defines the desired state of GPUGroupTemplate
type GPUGroupTemplateSpec struct {
	// Template specifies the metadata and spec of GPUGroups created from this template
	// +required
	// +kubebuilder:validation:Required
	Template GPUGroupTemplateData `json:"template"`
}

// GPUGroupTemplateStatus defines the observed state of GPUGroupTemplate
type GPUGroupTemplateStatus struct {
	// Names of GPUGroups created from this template
	// +optional
	TemplatedGPUGroupsNames []string `json:"templatedGPUGroupsNames,omitempty"`

	// Conditions represent the latest available observations of the GPUGroupTemplate's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// GPUGroupTemplate is the Schema for the gpugrouptemplates API
type GPUGroupTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GPUGroupTemplateSpec   `json:"spec,omitempty"`
	Status GPUGroupTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GPUGroupTemplateList contains a list of GPUGroupTemplate
type GPUGroupTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GPUGroupTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GPUGroupTemplate{}, &GPUGroupTemplateList{})
}
