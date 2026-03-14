// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kaiv1alpha1 "github.com/kai-scheduler/KAI-scheduler/pkg/apis/kai/v1alpha1"
)

const (
	rateLimiterBaseDelay = time.Second
	rateLimiterMaxDelay  = time.Minute

	GPUGroupTemplateOwnerIndexer = ".metadata.ownerReferences.gpugrouptemplate"
)

type Configs struct {
	MaxConcurrentReconciles int
}

// GPUGroupTemplateReconciler reconciles GPUGroupTemplate objects
type GPUGroupTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	config Configs
}

// +kubebuilder:rbac:groups=kai.scheduler,resources=gpugrouptemplates,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=kai.scheduler,resources=gpugrouptemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kai.scheduler,resources=gpugroups,verbs=get;list;watch

func (r *GPUGroupTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(3).Info("Reconciling GPUGroupTemplate")

	template, err := r.getGPUGroupTemplate(ctx, req)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("GPUGroupTemplate %v not found, it might have been deleted.", req))
			return ctrl.Result{}, nil
		}
		logger.Error(err, fmt.Sprintf("Failed to get GPUGroupTemplate for request %s/%s", req.Namespace, req.Name))
		return ctrl.Result{}, err
	}

	return r.reconcileGPUGroupTemplate(ctx, template)
}

func (r *GPUGroupTemplateReconciler) reconcileGPUGroupTemplate(
	ctx context.Context, template *kaiv1alpha1.GPUGroupTemplate,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	ownedGPUGroups, err := r.listOwnedGPUGroups(ctx, template)
	if err != nil {
		logger.Error(err, "Failed to list owned GPUGroups")
		return ctrl.Result{}, err
	}

	gpuGroupNames := make([]string, 0, len(ownedGPUGroups))
	for _, gpuGroup := range ownedGPUGroups {
		gpuGroupNames = append(gpuGroupNames, gpuGroup.Name)
	}

	if stringSlicesEqual(template.Status.TemplatedGPUGroupsNames, gpuGroupNames) {
		return ctrl.Result{}, nil
	}

	template.Status.TemplatedGPUGroupsNames = gpuGroupNames
	if err := r.Status().Update(ctx, template); err != nil {
		logger.Error(err, "Failed to update GPUGroupTemplate status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *GPUGroupTemplateReconciler) SetupWithManager(mgr ctrl.Manager, configs Configs) error {
	r.config = configs

	err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &kaiv1alpha1.GPUGroup{}, GPUGroupTemplateOwnerIndexer,
		func(obj client.Object) []string {
			gpuGroup, ok := obj.(*kaiv1alpha1.GPUGroup)
			if !ok {
				return nil
			}
			for _, ownerRef := range gpuGroup.OwnerReferences {
				if ownerRef.Kind == "GPUGroupTemplate" {
					return []string{ownerRef.Name}
				}
			}
			return nil
		})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kaiv1alpha1.GPUGroupTemplate{}).
		Watches(&kaiv1alpha1.GPUGroup{}, handler.EnqueueRequestsFromMapFunc(mapGPUGroupEventToTemplate)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.config.MaxConcurrentReconciles,
			RateLimiter: workqueue.NewTypedItemExponentialFailureRateLimiter[ctrl.Request](
				rateLimiterBaseDelay, rateLimiterMaxDelay),
		}).
		Complete(r)
}

func (r *GPUGroupTemplateReconciler) getGPUGroupTemplate(ctx context.Context, req ctrl.Request) (*kaiv1alpha1.GPUGroupTemplate, error) {
	template := &kaiv1alpha1.GPUGroupTemplate{}
	err := r.Client.Get(ctx, req.NamespacedName, template)
	if err != nil {
		return nil, err
	}
	return template, nil
}

func (r *GPUGroupTemplateReconciler) listOwnedGPUGroups(ctx context.Context, template *kaiv1alpha1.GPUGroupTemplate) ([]kaiv1alpha1.GPUGroup, error) {
	gpuGroupList := &kaiv1alpha1.GPUGroupList{}
	err := r.Client.List(ctx, gpuGroupList,
		client.InNamespace(template.Namespace),
		client.MatchingFields{GPUGroupTemplateOwnerIndexer: template.Name},
	)
	if err != nil {
		return nil, err
	}
	return gpuGroupList.Items, nil
}

func mapGPUGroupEventToTemplate(_ context.Context, obj client.Object) []reconcile.Request {
	gpuGroup, ok := obj.(*kaiv1alpha1.GPUGroup)
	if !ok {
		return nil
	}

	for _, ownerRef := range gpuGroup.OwnerReferences {
		if ownerRef.Kind == "GPUGroupTemplate" {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      ownerRef.Name,
						Namespace: gpuGroup.Namespace,
					},
				},
			}
		}
	}

	return nil
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
