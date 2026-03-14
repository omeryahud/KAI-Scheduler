// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"testing"

	"k8s.io/utils/ptr"
)

func TestGPUGroupValidateCreate(t *testing.T) {
	tests := []struct {
		name      string
		gpuGroup  *GPUGroup
		expectErr bool
	}{
		{
			name: "valid GPUGroup",
			gpuGroup: &GPUGroup{
				Spec: GPUGroupSpec{
					GPUCount: 2,
				},
			},
			expectErr: false,
		},
		{
			name: "valid GPUGroup with maxAttachedPods",
			gpuGroup: &GPUGroup{
				Spec: GPUGroupSpec{
					GPUCount:        1,
					MaxAttachedPods: ptr.To(int32(3)),
				},
			},
			expectErr: false,
		},
		{
			name: "invalid gpuCount zero",
			gpuGroup: &GPUGroup{
				Spec: GPUGroupSpec{
					GPUCount: 0,
				},
			},
			expectErr: true,
		},
		{
			name: "invalid gpuCount negative",
			gpuGroup: &GPUGroup{
				Spec: GPUGroupSpec{
					GPUCount: -1,
				},
			},
			expectErr: true,
		},
		{
			name: "invalid maxAttachedPods zero",
			gpuGroup: &GPUGroup{
				Spec: GPUGroupSpec{
					GPUCount:        1,
					MaxAttachedPods: ptr.To(int32(0)),
				},
			},
			expectErr: true,
		},
	}

	validator := &GPUGroup{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validator.ValidateCreate(context.Background(), tt.gpuGroup)
			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateCreate() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestGPUGroupValidateUpdate(t *testing.T) {
	tests := []struct {
		name      string
		oldGroup  *GPUGroup
		newGroup  *GPUGroup
		expectErr bool
	}{
		{
			name: "no changes",
			oldGroup: &GPUGroup{
				Spec: GPUGroupSpec{GPUCount: 2},
			},
			newGroup: &GPUGroup{
				Spec: GPUGroupSpec{GPUCount: 2},
			},
			expectErr: false,
		},
		{
			name: "gpuCount changed",
			oldGroup: &GPUGroup{
				Spec: GPUGroupSpec{GPUCount: 2},
			},
			newGroup: &GPUGroup{
				Spec: GPUGroupSpec{GPUCount: 4},
			},
			expectErr: true,
		},
		{
			name: "maxAttachedPods increased",
			oldGroup: &GPUGroup{
				Spec: GPUGroupSpec{
					GPUCount:        2,
					MaxAttachedPods: ptr.To(int32(3)),
				},
			},
			newGroup: &GPUGroup{
				Spec: GPUGroupSpec{
					GPUCount:        2,
					MaxAttachedPods: ptr.To(int32(5)),
				},
			},
			expectErr: false,
		},
		{
			name: "maxAttachedPods decreased",
			oldGroup: &GPUGroup{
				Spec: GPUGroupSpec{
					GPUCount:        2,
					MaxAttachedPods: ptr.To(int32(5)),
				},
			},
			newGroup: &GPUGroup{
				Spec: GPUGroupSpec{
					GPUCount:        2,
					MaxAttachedPods: ptr.To(int32(3)),
				},
			},
			expectErr: true,
		},
		{
			name: "maxAttachedPods set to nil when previously set",
			oldGroup: &GPUGroup{
				Spec: GPUGroupSpec{
					GPUCount:        2,
					MaxAttachedPods: ptr.To(int32(3)),
				},
			},
			newGroup: &GPUGroup{
				Spec: GPUGroupSpec{
					GPUCount: 2,
				},
			},
			expectErr: true,
		},
		{
			name: "maxAttachedPods set when previously nil",
			oldGroup: &GPUGroup{
				Spec: GPUGroupSpec{
					GPUCount: 2,
				},
			},
			newGroup: &GPUGroup{
				Spec: GPUGroupSpec{
					GPUCount:        2,
					MaxAttachedPods: ptr.To(int32(3)),
				},
			},
			expectErr: false,
		},
		{
			name: "maxAttachedPods unchanged",
			oldGroup: &GPUGroup{
				Spec: GPUGroupSpec{
					GPUCount:        2,
					MaxAttachedPods: ptr.To(int32(3)),
				},
			},
			newGroup: &GPUGroup{
				Spec: GPUGroupSpec{
					GPUCount:        2,
					MaxAttachedPods: ptr.To(int32(3)),
				},
			},
			expectErr: false,
		},
	}

	validator := &GPUGroup{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validator.ValidateUpdate(context.Background(), tt.oldGroup, tt.newGroup)
			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateUpdate() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}
