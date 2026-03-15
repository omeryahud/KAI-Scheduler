// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	gpuGroupLabel = "kai.scheduler/gpu-group"

	gpuGroupReservationPodPrefix = "gpu-group-reservation"

	rateLimiterBaseDelay = time.Second
	rateLimiterMaxDelay  = time.Minute

	GPUGroupToPodIndexer = ".metadata.labels.kai.scheduler/gpu-group"

	gpuGroupFinalizer = "kai.scheduler/gpugroup-protection"
)

type Configs struct {
	MaxConcurrentReconciles int
}

// GPUGroupReconciler reconciles GPUGroup objects
type GPUGroupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	config Configs
}

// +kubebuilder:rbac:groups=kai.scheduler,resources=gpugroups,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=kai.scheduler,resources=gpugroups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kai.scheduler,resources=gpugroups/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

func (r *GPUGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(3).Info("Reconciling GPUGroup")

	gpuGroup, err := r.getGPUGroup(ctx, req)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("GPUGroup %v not found, it might have been deleted.", req))
			return ctrl.Result{}, nil
		}
		logger.Error(err, fmt.Sprintf("Failed to get GPUGroup for request %s/%s", req.Namespace, req.Name))
		return ctrl.Result{}, err
	}

	if err := r.ensureFinalizer(ctx, gpuGroup); err != nil {
		return ctrl.Result{}, err
	}

	if !gpuGroup.DeletionTimestamp.IsZero() {
		return r.reconcileDeletion(ctx, gpuGroup)
	}

	return r.reconcileGPUGroup(ctx, gpuGroup)
}

func (r *GPUGroupReconciler) reconcileGPUGroup(ctx context.Context, gpuGroup *kaiv1alpha1.GPUGroup) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	consumerPods, err := r.listConsumerPods(ctx, gpuGroup)
	if err != nil {
		logger.Error(err, "Failed to list consumer pods")
		return ctrl.Result{}, err
	}

	reservationPod, err := r.getReservationPod(ctx, gpuGroup)
	if err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "Failed to get reservation pod")
		return ctrl.Result{}, err
	}

	originalStatus := gpuGroup.Status.DeepCopy()

	r.reconcilePhase(gpuGroup, reservationPod)
	r.updateAttachedPodsStatus(gpuGroup, consumerPods)

	if !statusEqual(originalStatus, &gpuGroup.Status) {
		if err := r.Status().Update(ctx, gpuGroup); err != nil {
			logger.Error(err, "Failed to update GPUGroup status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *GPUGroupReconciler) ensureFinalizer(ctx context.Context, gpuGroup *kaiv1alpha1.GPUGroup) error {
	if controllerutil.ContainsFinalizer(gpuGroup, gpuGroupFinalizer) {
		return nil
	}
	controllerutil.AddFinalizer(gpuGroup, gpuGroupFinalizer)
	return r.Update(ctx, gpuGroup)
}

func (r *GPUGroupReconciler) reconcileDeletion(ctx context.Context, gpuGroup *kaiv1alpha1.GPUGroup) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(gpuGroup, gpuGroupFinalizer) {
		return ctrl.Result{}, nil
	}

	consumerPods, err := r.listConsumerPods(ctx, gpuGroup)
	if err != nil {
		logger.Error(err, "Failed to list consumer pods during deletion")
		return ctrl.Result{}, err
	}

	if len(consumerPods) > 0 {
		logger.Info("GPUGroup still has attached pods, cannot remove finalizer",
			"attachedPods", len(consumerPods))
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	controllerutil.RemoveFinalizer(gpuGroup, gpuGroupFinalizer)
	if err := r.Update(ctx, gpuGroup); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *GPUGroupReconciler) reconcilePhase(gpuGroup *kaiv1alpha1.GPUGroup, reservationPod *v1.Pod) {
	if gpuGroup.Status.Phase == kaiv1alpha1.GPUGroupPhaseAllocated && reservationPod != nil && !isPodHealthy(reservationPod) {
		gpuGroup.Status.Phase = kaiv1alpha1.GPUGroupPhaseFailed
		gpuGroup.Status.PhaseMessage = fmt.Sprintf("gpu-reservation pod %s is unhealthy (phase: %s)",
			reservationPod.Name, reservationPod.Status.Phase)
	}

	if gpuGroup.Status.Phase == kaiv1alpha1.GPUGroupPhaseFailed && reservationPod != nil && isPodHealthy(reservationPod) {
		gpuGroup.Status.Phase = kaiv1alpha1.GPUGroupPhaseAllocated
		gpuGroup.Status.PhaseMessage = fmt.Sprintf("gpu-reservation pod %s recovered", reservationPod.Name)
	}
}

func (r *GPUGroupReconciler) SetupWithManager(mgr ctrl.Manager, configs Configs) error {
	r.config = configs

	err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.Pod{}, GPUGroupToPodIndexer,
		func(obj client.Object) []string {
			pod, ok := obj.(*v1.Pod)
			if !ok {
				return nil
			}
			gpuGroupName, found := pod.Labels[gpuGroupLabel]
			if !found {
				return nil
			}
			return []string{gpuGroupName}
		})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kaiv1alpha1.GPUGroup{}).
		Watches(&v1.Pod{}, handler.EnqueueRequestsFromMapFunc(mapPodEventToGPUGroup)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.config.MaxConcurrentReconciles,
			RateLimiter: workqueue.NewTypedItemExponentialFailureRateLimiter[ctrl.Request](
				rateLimiterBaseDelay, rateLimiterMaxDelay),
		}).
		Complete(r)
}

func (r *GPUGroupReconciler) getGPUGroup(ctx context.Context, req ctrl.Request) (*kaiv1alpha1.GPUGroup, error) {
	gpuGroup := &kaiv1alpha1.GPUGroup{}
	err := r.Client.Get(ctx, req.NamespacedName, gpuGroup)
	if err != nil {
		return nil, err
	}
	return gpuGroup, nil
}

func (r *GPUGroupReconciler) listConsumerPods(ctx context.Context, gpuGroup *kaiv1alpha1.GPUGroup) ([]v1.Pod, error) {
	podList := &v1.PodList{}
	err := r.Client.List(ctx, podList,
		client.InNamespace(gpuGroup.Namespace),
		client.MatchingFields{GPUGroupToPodIndexer: gpuGroup.Name},
	)
	if err != nil {
		return nil, err
	}

	var consumerPods []v1.Pod
	for _, pod := range podList.Items {
		if !isReservationPod(&pod) && isPodScheduled(&pod) {
			consumerPods = append(consumerPods, pod)
		}
	}
	return consumerPods, nil
}

func (r *GPUGroupReconciler) getReservationPod(ctx context.Context, gpuGroup *kaiv1alpha1.GPUGroup) (*v1.Pod, error) {
	reservationPodName := fmt.Sprintf("%s-%s", gpuGroupReservationPodPrefix, gpuGroup.Name)
	pod := &v1.Pod{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: gpuGroup.Namespace,
		Name:      reservationPodName,
	}, pod)
	if err != nil {
		return nil, err
	}
	return pod, nil
}

func (r *GPUGroupReconciler) updateAttachedPodsStatus(gpuGroup *kaiv1alpha1.GPUGroup, consumerPods []v1.Pod) {
	attachedNames := make([]string, 0, len(consumerPods))
	uniqueMemberIDs := make([]string, 0)

	for _, pod := range consumerPods {
		attachedNames = append(attachedNames, pod.Name)
		if memberID, found := pod.Labels["kai.scheduler/gpu-group-unique-member-id"]; found {
			uniqueMemberIDs = append(uniqueMemberIDs, memberID)
		}
	}

	gpuGroup.Status.AttachedPodsNames = attachedNames
	gpuGroup.Status.UniqueMemberIDs = uniqueMemberIDs
}

func mapPodEventToGPUGroup(_ context.Context, p client.Object) []reconcile.Request {
	labels := p.GetLabels()
	if labels == nil {
		return nil
	}

	gpuGroupName, found := labels[gpuGroupLabel]
	if !found {
		return nil
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      gpuGroupName,
				Namespace: p.GetNamespace(),
			},
		},
	}
}

func isReservationPod(pod *v1.Pod) bool {
	return len(pod.Name) > len(gpuGroupReservationPodPrefix) &&
		pod.Name[:len(gpuGroupReservationPodPrefix)] == gpuGroupReservationPodPrefix
}

func isPodScheduled(pod *v1.Pod) bool {
	if pod.Spec.NodeName != "" {
		return true
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodScheduled && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func isPodHealthy(pod *v1.Pod) bool {
	if pod.Status.Phase != v1.PodRunning {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func statusEqual(a *kaiv1alpha1.GPUGroupStatus, b *kaiv1alpha1.GPUGroupStatus) bool {
	if a.Phase != b.Phase || a.NodeName != b.NodeName || a.PhaseMessage != b.PhaseMessage {
		return false
	}
	if !stringSlicesEqual(a.GPUSUUIDs, b.GPUSUUIDs) {
		return false
	}
	if !stringSlicesEqual(a.AttachedPodsNames, b.AttachedPodsNames) {
		return false
	}
	if !stringSlicesEqual(a.UniqueMemberIDs, b.UniqueMemberIDs) {
		return false
	}
	return true
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
