// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kaiv1alpha1 "github.com/kai-scheduler/KAI-scheduler/pkg/apis/kai/v1alpha1"
)

func TestMapGPUGroupEventToTemplate(t *testing.T) {
	tests := []struct {
		name           string
		gpuGroup       *kaiv1alpha1.GPUGroup
		expectedLength int
		expectedName   string
	}{
		{
			name: "GPUGroup owned by template",
			gpuGroup: &kaiv1alpha1.GPUGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gpu-group-1",
					Namespace: "test-ns",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "GPUGroupTemplate",
							Name: "my-template",
							UID:  types.UID("uid-1"),
						},
					},
				},
			},
			expectedLength: 1,
			expectedName:   "my-template",
		},
		{
			name: "GPUGroup not owned by template",
			gpuGroup: &kaiv1alpha1.GPUGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gpu-group-1",
					Namespace: "test-ns",
				},
			},
			expectedLength: 0,
		},
		{
			name: "GPUGroup owned by different kind",
			gpuGroup: &kaiv1alpha1.GPUGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gpu-group-1",
					Namespace: "test-ns",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "SomeOtherKind",
							Name: "other-owner",
						},
					},
				},
			},
			expectedLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := mapGPUGroupEventToTemplate(nil, tt.gpuGroup)
			if len(requests) != tt.expectedLength {
				t.Errorf("expected %d requests, got %d", tt.expectedLength, len(requests))
				return
			}
			if tt.expectedLength > 0 {
				if requests[0].Name != tt.expectedName {
					t.Errorf("expected name %s, got %s", tt.expectedName, requests[0].Name)
				}
				if requests[0].Namespace != tt.gpuGroup.Namespace {
					t.Errorf("expected namespace %s, got %s", tt.gpuGroup.Namespace, requests[0].Namespace)
				}
			}
		})
	}
}

func TestStringSlicesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{name: "both nil", a: nil, b: nil, expected: true},
		{name: "both empty", a: []string{}, b: []string{}, expected: true},
		{name: "equal", a: []string{"a", "b"}, b: []string{"a", "b"}, expected: true},
		{name: "different length", a: []string{"a"}, b: []string{"a", "b"}, expected: false},
		{name: "different values", a: []string{"a", "b"}, b: []string{"a", "c"}, expected: false},
		{name: "nil vs empty", a: nil, b: []string{}, expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stringSlicesEqual(tt.a, tt.b); got != tt.expected {
				t.Errorf("stringSlicesEqual() = %v, expected %v", got, tt.expected)
			}
		})
	}
}
