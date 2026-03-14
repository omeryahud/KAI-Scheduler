// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (g *GPUGroup) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(g).
		WithValidator(&GPUGroup{}).
		Complete()
}

func (_ *GPUGroup) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	logger := log.FromContext(ctx)
	gpuGroup, ok := obj.(*GPUGroup)
	if !ok {
		return nil, fmt.Errorf("expected a GPUGroup but got a %T", obj)
	}
	logger.Info("validate create", "namespace", gpuGroup.Namespace, "name", gpuGroup.Name)

	return nil, validateGPUGroupSpec(&gpuGroup.Spec)
}

func (_ *GPUGroup) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	logger := log.FromContext(ctx)
	newGPUGroup, ok := newObj.(*GPUGroup)
	if !ok {
		return nil, fmt.Errorf("expected a GPUGroup but got a %T", newObj)
	}
	oldGPUGroup, ok := oldObj.(*GPUGroup)
	if !ok {
		return nil, fmt.Errorf("expected a GPUGroup but got a %T", oldObj)
	}
	logger.Info("validate update", "namespace", newGPUGroup.Namespace, "name", newGPUGroup.Name)

	if err := validateGPUGroupSpec(&newGPUGroup.Spec); err != nil {
		return nil, err
	}

	if newGPUGroup.Spec.GPUCount != oldGPUGroup.Spec.GPUCount {
		return nil, fmt.Errorf("spec.gpuCount is immutable")
	}

	if err := validateMaxAttachedPodsUpdate(oldGPUGroup.Spec.MaxAttachedPods, newGPUGroup.Spec.MaxAttachedPods); err != nil {
		return nil, err
	}

	return nil, nil
}

func (_ *GPUGroup) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	logger := log.FromContext(ctx)
	gpuGroup, ok := obj.(*GPUGroup)
	if !ok {
		return nil, fmt.Errorf("expected a GPUGroup but got a %T", obj)
	}
	logger.Info("validate delete", "namespace", gpuGroup.Namespace, "name", gpuGroup.Name)
	return nil, nil
}

func validateGPUGroupSpec(spec *GPUGroupSpec) error {
	if spec.GPUCount < 1 {
		return fmt.Errorf("spec.gpuCount must be at least 1")
	}
	if spec.MaxAttachedPods != nil && *spec.MaxAttachedPods < 1 {
		return fmt.Errorf("spec.maxAttachedPods must be at least 1 when set")
	}
	return nil
}

func validateMaxAttachedPodsUpdate(oldVal, newVal *int32) error {
	if oldVal == nil {
		return nil
	}
	if newVal == nil {
		return fmt.Errorf("spec.maxAttachedPods cannot be set to nil once set")
	}
	if *newVal < *oldVal {
		return fmt.Errorf("spec.maxAttachedPods can only be increased (old: %d, new: %d)", *oldVal, *newVal)
	}
	return nil
}
