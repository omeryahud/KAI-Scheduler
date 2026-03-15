// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"testing"
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

func TestGPUGroupValidateDelete(t *testing.T) {
	tests := []struct {
		name      string
		gpuGroup  *GPUGroup
		expectErr bool
	}{
		{
			name: "no attached pods",
			gpuGroup: &GPUGroup{
				Status: GPUGroupStatus{},
			},
			expectErr: false,
		},
		{
			name: "has attached pods",
			gpuGroup: &GPUGroup{
				Status: GPUGroupStatus{
					AttachedPodsNames: []string{"pod-1", "pod-2"},
				},
			},
			expectErr: true,
		},
		{
			name: "empty attached pods list",
			gpuGroup: &GPUGroup{
				Status: GPUGroupStatus{
					AttachedPodsNames: []string{},
				},
			},
			expectErr: false,
		},
	}

	validator := &GPUGroup{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validator.ValidateDelete(context.Background(), tt.gpuGroup)
			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateDelete() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}
