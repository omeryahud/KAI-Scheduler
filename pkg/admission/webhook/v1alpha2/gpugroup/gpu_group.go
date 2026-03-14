// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package gpugroup

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	"github.com/kai-scheduler/KAI-scheduler/pkg/binder/common"
	"github.com/kai-scheduler/KAI-scheduler/pkg/binder/common/gpusharingconfigmap"
	"github.com/kai-scheduler/KAI-scheduler/pkg/common/constants"
)

const (
	gpuGroupLabel                        = "kai.scheduler/gpu-group"
	gpuGroupTemplateLabel                = "kai.scheduler/gpu-group-template"
	gpuGroupAttachedContainersAnnotation = "kai.scheduler/gpu-group-attached-container-names"
)

type GPUGroupPlugin struct{}

func New() *GPUGroupPlugin {
	return &GPUGroupPlugin{}
}

func (p *GPUGroupPlugin) Name() string {
	return "gpugroup"
}

func (p *GPUGroupPlugin) Validate(pod *v1.Pod) error {
	if !referencesGPUGroup(pod) {
		return nil
	}

	if _, hasQueue := pod.Labels[constants.DefaultQueueLabel]; !hasQueue {
		return fmt.Errorf("pod %s/%s references a GPUGroup but is missing the %s label",
			pod.Namespace, pod.Name, constants.DefaultQueueLabel)
	}

	return nil
}

func (p *GPUGroupPlugin) Mutate(pod *v1.Pod) error {
	if !referencesGPUGroup(pod) {
		return nil
	}

	containerRefs, err := getTargetContainerRefs(pod)
	if err != nil {
		return err
	}

	for _, containerRef := range containerRefs {
		capabilitiesConfigMapName := gpusharingconfigmap.SetGpuCapabilitiesConfigMapName(pod, containerRef)
		directEnvVarsMapName, err := gpusharingconfigmap.ExtractDirectEnvVarsConfigMapName(pod, containerRef)
		if err != nil {
			return err
		}

		common.AddGPUSharingEnvVars(containerRef.Container, capabilitiesConfigMapName)
		common.SetConfigMapVolume(pod, capabilitiesConfigMapName)
		common.AddDirectEnvVarsConfigMapSource(containerRef.Container, directEnvVarsMapName)
	}

	return nil
}

func referencesGPUGroup(pod *v1.Pod) bool {
	if pod.Labels == nil {
		return false
	}
	_, hasGPUGroup := pod.Labels[gpuGroupLabel]
	_, hasTemplate := pod.Labels[gpuGroupTemplateLabel]
	return hasGPUGroup || hasTemplate
}

func getTargetContainerRefs(pod *v1.Pod) ([]*gpusharingconfigmap.PodContainerRef, error) {
	if len(pod.Spec.Containers) == 0 {
		return nil, nil
	}

	annotation, found := pod.Annotations[gpuGroupAttachedContainersAnnotation]
	if !found {
		return []*gpusharingconfigmap.PodContainerRef{
			{
				Container: &pod.Spec.Containers[0],
				Index:     0,
				Type:      gpusharingconfigmap.RegularContainer,
			},
		}, nil
	}

	containerNames := strings.Split(annotation, ",")
	var refs []*gpusharingconfigmap.PodContainerRef
	for _, name := range containerNames {
		name = strings.TrimSpace(name)
		ref, err := findContainerRef(pod, name)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}

	return refs, nil
}

func findContainerRef(pod *v1.Pod, name string) (*gpusharingconfigmap.PodContainerRef, error) {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == name {
			return &gpusharingconfigmap.PodContainerRef{
				Container: &pod.Spec.Containers[i],
				Index:     i,
				Type:      gpusharingconfigmap.RegularContainer,
			}, nil
		}
	}
	for i := range pod.Spec.InitContainers {
		if pod.Spec.InitContainers[i].Name == name {
			return &gpusharingconfigmap.PodContainerRef{
				Container: &pod.Spec.InitContainers[i],
				Index:     i,
				Type:      gpusharingconfigmap.InitContainer,
			}, nil
		}
	}
	return nil, fmt.Errorf("container %q specified in %s not found in pod %s/%s",
		name, gpuGroupAttachedContainersAnnotation, pod.Namespace, pod.Name)
}
