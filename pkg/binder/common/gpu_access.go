// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NVIDIA/KAI-scheduler/pkg/binder/common/gpusharingconfigmap"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
)

const (
	visibleDevicesBC  = "RUNAI-VISIBLE-DEVICES" // Deprecated, this value was replaced with NVIDIA_VISIBLE_DEVICES
	NumOfGpusEnvVarBC = "RUNAI_NUM_OF_GPUS"     // Deprecated, please use GPU_PORTION env var instead
)

func AddGPUSharingEnvVars(container *v1.Container, sharedGpuConfigMapName string) {
	AddEnvVarToContainer(container, v1.EnvVar{
		Name: NvidiaVisibleDevices,
		ValueFrom: &v1.EnvVarSource{
			ConfigMapKeyRef: &v1.ConfigMapKeySelector{
				Key: NvidiaVisibleDevices,
				LocalObjectReference: v1.LocalObjectReference{
					Name: sharedGpuConfigMapName,
				},
			},
		},
	})

	AddEnvVarToContainer(container, v1.EnvVar{
		Name: NumOfGpusEnvVarBC,
		ValueFrom: &v1.EnvVarSource{
			ConfigMapKeyRef: &v1.ConfigMapKeySelector{
				Key: NumOfGpusEnvVarBC,
				LocalObjectReference: v1.LocalObjectReference{
					Name: sharedGpuConfigMapName,
				},
			},
		},
	})

	AddEnvVarToContainer(container, v1.EnvVar{
		Name: GPUPortion,
		ValueFrom: &v1.EnvVarSource{
			ConfigMapKeyRef: &v1.ConfigMapKeySelector{
				Key: GPUPortion,
				LocalObjectReference: v1.LocalObjectReference{
					Name: sharedGpuConfigMapName,
				},
			},
		},
	})
}

func SetNvidiaVisibleDevices(
	ctx context.Context, kubeClient client.Client, pod *v1.Pod, containerRef *gpusharingconfigmap.PodContainerRef,
	visibleDevicesValue string,
) error {
	nvidiaVisibleDevicesDefinedInSpec := false
	for _, envVar := range containerRef.Container.Env {
		if envVar.Name == NvidiaVisibleDevices && envVar.ValueFrom != nil &&
			envVar.ValueFrom.ConfigMapKeyRef != nil {
			nvidiaVisibleDevicesDefinedInSpec = true
		}
	}

	var configMapName string
	var err error
	var updateFunc func(data map[string]string) error
	if nvidiaVisibleDevicesDefinedInSpec {
		configMapName, err = gpusharingconfigmap.ExtractCapabilitiesConfigMapName(pod, containerRef)
	} else {
		configMapName, err = gpusharingconfigmap.ExtractDirectEnvVarsConfigMapName(pod, containerRef)
	}
	if err != nil {
		return err
	}
	updateFunc = func(data map[string]string) error {
		// BC for pods that were created with NVIDIA_VISIBLE_DEVICES env var
		// with value from RUNAI-VISIBLE-DEVICES entry in GPU sharing configmap
		if _, found := data[visibleDevicesBC]; found {
			data[visibleDevicesBC] = visibleDevicesValue
		}
		data[NvidiaVisibleDevices] = visibleDevicesValue
		return nil
	}
	err = UpdateConfigMapEnvironmentVariable(ctx, kubeClient, pod, configMapName, updateFunc)
	if err != nil {
		return fmt.Errorf("failed to update gpu sharing configmap %s for pod <%s/%s>: %v",
			configMapName, pod.Namespace, pod.Name, err)
	}
	return nil
}

func SetGPUPortion(
	ctx context.Context, kubeClient client.Client, pod *v1.Pod, containerRef *gpusharingconfigmap.PodContainerRef,
	gpuPortionStr string,
) error {
	updateFunc := func(data map[string]string) error {
		data[NumOfGpusEnvVarBC] = gpuPortionStr
		data[GPUPortion] = gpuPortionStr
		return nil
	}
	capabilitiesMapName, err := gpusharingconfigmap.ExtractCapabilitiesConfigMapName(pod, containerRef)
	if err != nil {
		return err
	}

	err = UpdateConfigMapEnvironmentVariable(ctx, kubeClient, pod, capabilitiesMapName, updateFunc)
	if err != nil {
		return fmt.Errorf("failed to update GPU_PORTION value in gpu sharing configmap for pod <%s/%s>: %v",
			pod.Namespace, pod.Name, err)
	}
	return nil
}

func UpdateConfigMapEnvironmentVariable(
	ctx context.Context, kubeclient client.Client, task *v1.Pod,
	configMapName string, changesFunc func(map[string]string) error,
) error {
	logger := log.FromContext(ctx)
	logger.Info("Updating config map for job", "namespace", task.Namespace, "name", task.Name,
		"configMapName", configMapName)

	configMap := &v1.ConfigMap{}
	err := kubeclient.Get(ctx, types.NamespacedName{
		Name:      configMapName,
		Namespace: task.Namespace,
	}, configMap)
	if err != nil {
		logger.Error(err, "failed to get configMap", "configMapName", configMapName)
		return err
	}
	if configMap.Data == nil {
		configMap.Data = map[string]string{}
	}

	origConfigMap := configMap.DeepCopy()
	if err = changesFunc(configMap.Data); err != nil {
		return err
	}

	err = kubeclient.Patch(ctx, configMap, client.MergeFrom(origConfigMap))
	if err != nil {
		logger.Error(err, "failed to update config map", "configMapName",
			configMapName, "error", err.Error())
		return err
	}

	return nil
}

func SetNVIDIARuntimeClass(pod *v1.Pod) {
	pod.Spec.RuntimeClassName = ptr.To(constants.DefaultRuntimeClassName)
}
