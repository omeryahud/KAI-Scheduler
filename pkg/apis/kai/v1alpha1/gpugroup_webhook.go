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

	return nil, nil
}

func (_ *GPUGroup) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	logger := log.FromContext(ctx)
	gpuGroup, ok := obj.(*GPUGroup)
	if !ok {
		return nil, fmt.Errorf("expected a GPUGroup but got a %T", obj)
	}
	logger.Info("validate delete", "namespace", gpuGroup.Namespace, "name", gpuGroup.Name)

	if len(gpuGroup.Status.AttachedPodsNames) > 0 {
		return nil, fmt.Errorf("cannot delete GPUGroup %s/%s: %d pod(s) are still attached",
			gpuGroup.Namespace, gpuGroup.Name, len(gpuGroup.Status.AttachedPodsNames))
	}
	return nil, nil
}

func validateGPUGroupSpec(spec *GPUGroupSpec) error {
	if spec.GPUCount < 1 {
		return fmt.Errorf("spec.gpuCount must be at least 1")
	}
	return nil
}
