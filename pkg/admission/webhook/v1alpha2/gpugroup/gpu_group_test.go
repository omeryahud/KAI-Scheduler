// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package gpugroup

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kai-scheduler/KAI-scheduler/pkg/common/constants"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		pod       *v1.Pod
		expectErr bool
	}{
		{
			name: "pod without GPUGroup reference",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
			},
			expectErr: false,
		},
		{
			name: "pod with GPUGroup label and queue label",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						gpuGroupLabel:              "my-gpu-group",
						constants.DefaultQueueLabel: "my-queue",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "pod with GPUGroup label but missing queue label",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						gpuGroupLabel: "my-gpu-group",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "pod with GPUGroupTemplate label and queue label",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						gpuGroupTemplateLabel:       "my-template",
						constants.DefaultQueueLabel: "my-queue",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "pod with GPUGroupTemplate label but missing queue label",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						gpuGroupTemplateLabel: "my-template",
					},
				},
			},
			expectErr: true,
		},
	}

	plugin := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := plugin.Validate(tt.pod)
			if (err != nil) != tt.expectErr {
				t.Errorf("Validate() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestMutate(t *testing.T) {
	tests := []struct {
		name                    string
		pod                     *v1.Pod
		expectErr               bool
		expectEnvVarsOnContainer []string
	}{
		{
			name: "pod without GPUGroup reference is not mutated",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Name: "main"},
					},
				},
			},
			expectErr:               false,
			expectEnvVarsOnContainer: nil,
		},
		{
			name: "pod with GPUGroup label mutates first container by default",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						gpuGroupLabel: "my-gpu-group",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Name: "main"},
						{Name: "sidecar"},
					},
				},
			},
			expectErr:               false,
			expectEnvVarsOnContainer: []string{"main"},
		},
		{
			name: "pod with GPUGroupTemplate label mutates first container by default",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						gpuGroupTemplateLabel: "my-template",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Name: "main"},
					},
				},
			},
			expectErr:               false,
			expectEnvVarsOnContainer: []string{"main"},
		},
		{
			name: "pod with attached-container-names annotation mutates specified containers",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						gpuGroupLabel: "my-gpu-group",
					},
					Annotations: map[string]string{
						gpuGroupAttachedContainersAnnotation: "primary,secondary",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Name: "primary"},
						{Name: "secondary"},
						{Name: "sidecar"},
					},
				},
			},
			expectErr:               false,
			expectEnvVarsOnContainer: []string{"primary", "secondary"},
		},
		{
			name: "pod with non-existent container in annotation fails",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						gpuGroupLabel: "my-gpu-group",
					},
					Annotations: map[string]string{
						gpuGroupAttachedContainersAnnotation: "nonexistent",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Name: "main"},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "pod with no containers is not mutated",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						gpuGroupLabel: "my-gpu-group",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{},
				},
			},
			expectErr: false,
		},
		{
			name: "pod with init container in annotation",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
					Labels: map[string]string{
						gpuGroupLabel: "my-gpu-group",
					},
					Annotations: map[string]string{
						gpuGroupAttachedContainersAnnotation: "init-gpu",
					},
				},
				Spec: v1.PodSpec{
					InitContainers: []v1.Container{
						{Name: "init-gpu"},
					},
					Containers: []v1.Container{
						{Name: "main"},
					},
				},
			},
			expectErr:               false,
			expectEnvVarsOnContainer: []string{"init-gpu"},
		},
	}

	plugin := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := plugin.Mutate(tt.pod)
			if (err != nil) != tt.expectErr {
				t.Errorf("Mutate() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if tt.expectErr {
				return
			}

			for _, containerName := range tt.expectEnvVarsOnContainer {
				container := findContainer(tt.pod, containerName)
				if container == nil {
					t.Errorf("container %q not found in pod", containerName)
					continue
				}
				if !hasEnvVar(container, constants.NvidiaVisibleDevices) {
					t.Errorf("container %q missing %s env var", containerName, constants.NvidiaVisibleDevices)
				}
				if len(container.EnvFrom) == 0 {
					t.Errorf("container %q missing envFrom entries", containerName)
				}
			}
		})
	}
}

func findContainer(pod *v1.Pod, name string) *v1.Container {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == name {
			return &pod.Spec.Containers[i]
		}
	}
	for i := range pod.Spec.InitContainers {
		if pod.Spec.InitContainers[i].Name == name {
			return &pod.Spec.InitContainers[i]
		}
	}
	return nil
}

func hasEnvVar(container *v1.Container, name string) bool {
	for _, env := range container.Env {
		if env.Name == name {
			return true
		}
	}
	return false
}
