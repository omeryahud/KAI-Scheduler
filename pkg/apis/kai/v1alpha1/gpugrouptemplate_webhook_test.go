// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"testing"

	"k8s.io/utils/ptr"
)

func TestGPUGroupTemplateValidateCreate(t *testing.T) {
	tests := []struct {
		name      string
		template  *GPUGroupTemplate
		expectErr bool
	}{
		{
			name: "valid template",
			template: &GPUGroupTemplate{
				Spec: GPUGroupTemplateSpec{
					Template: GPUGroupTemplateData{
						Spec: GPUGroupSpec{
							GPUCount: 2,
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "valid template with maxAttachedPods",
			template: &GPUGroupTemplate{
				Spec: GPUGroupTemplateSpec{
					Template: GPUGroupTemplateData{
						Spec: GPUGroupSpec{
							GPUCount:        1,
							MaxAttachedPods: ptr.To(int32(5)),
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "invalid gpuCount zero",
			template: &GPUGroupTemplate{
				Spec: GPUGroupTemplateSpec{
					Template: GPUGroupTemplateData{
						Spec: GPUGroupSpec{
							GPUCount: 0,
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "invalid maxAttachedPods zero",
			template: &GPUGroupTemplate{
				Spec: GPUGroupTemplateSpec{
					Template: GPUGroupTemplateData{
						Spec: GPUGroupSpec{
							GPUCount:        2,
							MaxAttachedPods: ptr.To(int32(0)),
						},
					},
				},
			},
			expectErr: true,
		},
	}

	validator := &GPUGroupTemplate{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validator.ValidateCreate(context.Background(), tt.template)
			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateCreate() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}
