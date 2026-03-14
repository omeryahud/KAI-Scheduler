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

func (g *GPUGroupTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(g).
		WithValidator(&GPUGroupTemplate{}).
		Complete()
}

func (_ *GPUGroupTemplate) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	logger := log.FromContext(ctx)
	template, ok := obj.(*GPUGroupTemplate)
	if !ok {
		return nil, fmt.Errorf("expected a GPUGroupTemplate but got a %T", obj)
	}
	logger.Info("validate create", "namespace", template.Namespace, "name", template.Name)

	return nil, validateGPUGroupSpec(&template.Spec.Template.Spec)
}

func (_ *GPUGroupTemplate) ValidateUpdate(ctx context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	logger := log.FromContext(ctx)
	template, ok := newObj.(*GPUGroupTemplate)
	if !ok {
		return nil, fmt.Errorf("expected a GPUGroupTemplate but got a %T", newObj)
	}
	logger.Info("validate update", "namespace", template.Namespace, "name", template.Name)

	return nil, validateGPUGroupSpec(&template.Spec.Template.Spec)
}

func (_ *GPUGroupTemplate) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	logger := log.FromContext(ctx)
	template, ok := obj.(*GPUGroupTemplate)
	if !ok {
		return nil, fmt.Errorf("expected a GPUGroupTemplate but got a %T", obj)
	}
	logger.Info("validate delete", "namespace", template.Namespace, "name", template.Name)
	return nil, nil
}
