// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kaiv1alpha1 "github.com/kai-scheduler/KAI-scheduler/pkg/apis/kai/v1alpha1"
)

func TestIsReservationPod(t *testing.T) {
	tests := []struct {
		name     string
		podName  string
		expected bool
	}{
		{
			name:     "reservation pod",
			podName:  "gpu-group-reservation-my-group",
			expected: true,
		},
		{
			name:     "consumer pod",
			podName:  "consumer-1",
			expected: false,
		},
		{
			name:     "pod name equals prefix",
			podName:  "gpu-group-reservation",
			expected: false,
		},
		{
			name:     "old-style reservation pod without gpu-group prefix",
			podName:  "gpu-reservation-my-group",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: tt.podName}}
			if got := isReservationPod(pod); got != tt.expected {
				t.Errorf("isReservationPod() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestIsPodScheduled(t *testing.T) {
	tests := []struct {
		name     string
		pod      *v1.Pod
		expected bool
	}{
		{
			name: "pod with nodeName set",
			pod: &v1.Pod{
				Spec: v1.PodSpec{NodeName: "node-1"},
			},
			expected: true,
		},
		{
			name: "pod with PodScheduled condition true",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{Type: v1.PodScheduled, Status: v1.ConditionTrue},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod with PodScheduled condition false",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{Type: v1.PodScheduled, Status: v1.ConditionFalse},
					},
				},
			},
			expected: false,
		},
		{
			name:     "pending pod with no conditions and no nodeName",
			pod:      &v1.Pod{},
			expected: false,
		},
		{
			name: "running pod with nodeName",
			pod: &v1.Pod{
				Spec:   v1.PodSpec{NodeName: "node-1"},
				Status: v1.PodStatus{Phase: v1.PodRunning},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPodScheduled(tt.pod); got != tt.expected {
				t.Errorf("isPodScheduled() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestIsPodHealthy(t *testing.T) {
	tests := []struct {
		name     string
		pod      *v1.Pod
		expected bool
	}{
		{
			name: "running and ready",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					Conditions: []v1.PodCondition{
						{Type: v1.PodReady, Status: v1.ConditionTrue},
					},
				},
			},
			expected: true,
		},
		{
			name: "running but not ready",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					Conditions: []v1.PodCondition{
						{Type: v1.PodReady, Status: v1.ConditionFalse},
					},
				},
			},
			expected: false,
		},
		{
			name: "running with no ready condition",
			pod: &v1.Pod{
				Status: v1.PodStatus{Phase: v1.PodRunning},
			},
			expected: false,
		},
		{
			name: "pending",
			pod: &v1.Pod{
				Status: v1.PodStatus{Phase: v1.PodPending},
			},
			expected: false,
		},
		{
			name: "succeeded",
			pod: &v1.Pod{
				Status: v1.PodStatus{Phase: v1.PodSucceeded},
			},
			expected: false,
		},
		{
			name: "failed",
			pod: &v1.Pod{
				Status: v1.PodStatus{Phase: v1.PodFailed},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPodHealthy(tt.pod); got != tt.expected {
				t.Errorf("isPodHealthy() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestReconcilePhase(t *testing.T) {
	healthyPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "gpu-group-reservation-test"},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			Conditions: []v1.PodCondition{
				{Type: v1.PodReady, Status: v1.ConditionTrue},
			},
		},
	}
	unhealthyPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "gpu-group-reservation-test"},
		Status:     v1.PodStatus{Phase: v1.PodFailed},
	}

	tests := []struct {
		name          string
		initialPhase  kaiv1alpha1.GPUGroupPhase
		reservationPod *v1.Pod
		expectedPhase kaiv1alpha1.GPUGroupPhase
		expectMessage bool
	}{
		{
			name:           "allocated with healthy reservation pod stays allocated",
			initialPhase:   kaiv1alpha1.GPUGroupPhaseAllocated,
			reservationPod: healthyPod,
			expectedPhase:  kaiv1alpha1.GPUGroupPhaseAllocated,
			expectMessage:  false,
		},
		{
			name:           "allocated with unhealthy reservation pod transitions to failed",
			initialPhase:   kaiv1alpha1.GPUGroupPhaseAllocated,
			reservationPod: unhealthyPod,
			expectedPhase:  kaiv1alpha1.GPUGroupPhaseFailed,
			expectMessage:  true,
		},
		{
			name:           "failed with healthy reservation pod transitions to allocated",
			initialPhase:   kaiv1alpha1.GPUGroupPhaseFailed,
			reservationPod: healthyPod,
			expectedPhase:  kaiv1alpha1.GPUGroupPhaseAllocated,
			expectMessage:  true,
		},
		{
			name:           "failed with unhealthy reservation pod stays failed",
			initialPhase:   kaiv1alpha1.GPUGroupPhaseFailed,
			reservationPod: unhealthyPod,
			expectedPhase:  kaiv1alpha1.GPUGroupPhaseFailed,
			expectMessage:  false,
		},
		{
			name:           "allocated with no reservation pod stays allocated",
			initialPhase:   kaiv1alpha1.GPUGroupPhaseAllocated,
			reservationPod: nil,
			expectedPhase:  kaiv1alpha1.GPUGroupPhaseAllocated,
			expectMessage:  false,
		},
		{
			name:           "accepted phase is unchanged",
			initialPhase:   kaiv1alpha1.GPUGroupPhaseAccepted,
			reservationPod: nil,
			expectedPhase:  kaiv1alpha1.GPUGroupPhaseAccepted,
			expectMessage:  false,
		},
	}

	reconciler := &GPUGroupReconciler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gpuGroup := &kaiv1alpha1.GPUGroup{
				Status: kaiv1alpha1.GPUGroupStatus{Phase: tt.initialPhase},
			}
			reconciler.reconcilePhase(gpuGroup, tt.reservationPod)

			if gpuGroup.Status.Phase != tt.expectedPhase {
				t.Errorf("Phase = %v, expected %v", gpuGroup.Status.Phase, tt.expectedPhase)
			}
			if tt.expectMessage && gpuGroup.Status.PhaseMessage == "" {
				t.Errorf("expected PhaseMessage to be set")
			}
			if !tt.expectMessage && gpuGroup.Status.PhaseMessage != "" {
				t.Errorf("expected PhaseMessage to be empty, got %q", gpuGroup.Status.PhaseMessage)
			}
		})
	}
}

func TestUpdateAttachedPodsStatus(t *testing.T) {
	tests := []struct {
		name              string
		consumerPods      []v1.Pod
		expectedPodNames  []string
		expectedMemberIDs []string
	}{
		{
			name:              "no consumer pods",
			consumerPods:      nil,
			expectedPodNames:  []string{},
			expectedMemberIDs: []string{},
		},
		{
			name: "consumer pods without unique member IDs",
			consumerPods: []v1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-2"}},
			},
			expectedPodNames:  []string{"pod-1", "pod-2"},
			expectedMemberIDs: []string{},
		},
		{
			name: "consumer pods with unique member IDs",
			consumerPods: []v1.Pod{
				{ObjectMeta: metav1.ObjectMeta{
					Name:   "pod-1",
					Labels: map[string]string{"kai.scheduler/gpu-group-unique-member-id": "member-1"},
				}},
				{ObjectMeta: metav1.ObjectMeta{
					Name:   "pod-2",
					Labels: map[string]string{"kai.scheduler/gpu-group-unique-member-id": "member-2"},
				}},
			},
			expectedPodNames:  []string{"pod-1", "pod-2"},
			expectedMemberIDs: []string{"member-1", "member-2"},
		},
	}

	reconciler := &GPUGroupReconciler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gpuGroup := &kaiv1alpha1.GPUGroup{}
			reconciler.updateAttachedPodsStatus(gpuGroup, tt.consumerPods)

			if !stringSlicesEqual(gpuGroup.Status.AttachedPodsNames, tt.expectedPodNames) {
				t.Errorf("AttachedPodsNames = %v, expected %v",
					gpuGroup.Status.AttachedPodsNames, tt.expectedPodNames)
			}
			if !stringSlicesEqual(gpuGroup.Status.UniqueMemberIDs, tt.expectedMemberIDs) {
				t.Errorf("UniqueMemberIDs = %v, expected %v",
					gpuGroup.Status.UniqueMemberIDs, tt.expectedMemberIDs)
			}
		})
	}
}

func TestMapPodEventToGPUGroup(t *testing.T) {
	tests := []struct {
		name           string
		pod            *v1.Pod
		expectedLength int
		expectedName   string
	}{
		{
			name: "pod with GPUGroup label",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "consumer",
					Namespace: "test-ns",
					Labels:    map[string]string{gpuGroupLabel: "my-group"},
				},
			},
			expectedLength: 1,
			expectedName:   "my-group",
		},
		{
			name: "pod without GPUGroup label",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other-pod",
					Namespace: "test-ns",
				},
			},
			expectedLength: 0,
		},
		{
			name: "pod with nil labels",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other-pod",
					Namespace: "test-ns",
				},
			},
			expectedLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := mapPodEventToGPUGroup(nil, tt.pod)
			if len(requests) != tt.expectedLength {
				t.Errorf("expected %d requests, got %d", tt.expectedLength, len(requests))
				return
			}
			if tt.expectedLength > 0 {
				if requests[0].Name != tt.expectedName {
					t.Errorf("expected name %s, got %s", tt.expectedName, requests[0].Name)
				}
			}
		})
	}
}

func TestStatusEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        *kaiv1alpha1.GPUGroupStatus
		b        *kaiv1alpha1.GPUGroupStatus
		expected bool
	}{
		{
			name:     "both empty",
			a:        &kaiv1alpha1.GPUGroupStatus{},
			b:        &kaiv1alpha1.GPUGroupStatus{},
			expected: true,
		},
		{
			name:     "different phase",
			a:        &kaiv1alpha1.GPUGroupStatus{Phase: kaiv1alpha1.GPUGroupPhaseAccepted},
			b:        &kaiv1alpha1.GPUGroupStatus{Phase: kaiv1alpha1.GPUGroupPhaseAllocated},
			expected: false,
		},
		{
			name:     "different phase message",
			a:        &kaiv1alpha1.GPUGroupStatus{PhaseMessage: "msg-1"},
			b:        &kaiv1alpha1.GPUGroupStatus{PhaseMessage: "msg-2"},
			expected: false,
		},
		{
			name: "same status",
			a: &kaiv1alpha1.GPUGroupStatus{
				Phase:             kaiv1alpha1.GPUGroupPhaseAllocated,
				PhaseMessage:      "allocated",
				NodeName:          "node-1",
				GPUSUUIDs:         []string{"uuid-1"},
				AttachedPodsNames: []string{"pod-1"},
				UniqueMemberIDs:   []string{"member-1"},
			},
			b: &kaiv1alpha1.GPUGroupStatus{
				Phase:             kaiv1alpha1.GPUGroupPhaseAllocated,
				PhaseMessage:      "allocated",
				NodeName:          "node-1",
				GPUSUUIDs:         []string{"uuid-1"},
				AttachedPodsNames: []string{"pod-1"},
				UniqueMemberIDs:   []string{"member-1"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusEqual(tt.a, tt.b); got != tt.expected {
				t.Errorf("statusEqual() = %v, expected %v", got, tt.expected)
			}
		})
	}
}
