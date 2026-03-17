// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package gpudeviceaffinity

import (
	"fmt"
	"strings"

	commonconstants "github.com/kai-scheduler/KAI-scheduler/pkg/common/constants"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/framework"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/log"
)

const pluginName = "gpudeviceaffinity"

type gpuDeviceAffinityPlugin struct{}

func New(_ framework.PluginArguments) framework.Plugin {
	return &gpuDeviceAffinityPlugin{}
}

func (p *gpuDeviceAffinityPlugin) Name() string {
	return pluginName
}

func (p *gpuDeviceAffinityPlugin) OnSessionOpen(ssn *framework.Session) {
	ssn.AddGPUOrderFn(gpuOrderFn)
}

func (p *gpuDeviceAffinityPlugin) OnSessionClose(_ *framework.Session) {}

func gpuOrderFn(task *pod_info.PodInfo, node *node_info.NodeInfo, gpuIdx string) (float64, error) {
	labels := task.Pod.Labels
	if labels == nil {
		return 0, nil
	}

	reqAff := parseCSV(labels[commonconstants.GPUSharingGroupRequiredAffinity])
	prefAff := parseCSV(labels[commonconstants.GPUSharingGroupPreferredAffinity])
	reqAntiAff := parseCSV(labels[commonconstants.GPUSharingGroupRequiredAntiAffinity])
	prefAntiAff := parseCSV(labels[commonconstants.GPUSharingGroupPreferredAntiAffinity])

	hasConstraints := len(reqAff) > 0 || len(prefAff) > 0 || len(reqAntiAff) > 0 || len(prefAntiAff) > 0
	if !hasConstraints {
		return 0, nil
	}

	allowFreeStr, exists := labels[commonconstants.GPUSharingGroupAllowFreeGPUAllocation]
	allowFree := !exists || allowFreeStr != "false"

	if gpuIdx == pod_info.WholeGpuIndicator {
		if !allowFree {
			return 0, fmt.Errorf("pod <%s/%s> disallows free GPU allocation", task.Namespace, task.Name)
		}
		return 0, nil
	}

	gpuIdentifiers := gpuGroupIdentifiers(node)
	identifiers := gpuIdentifiers[gpuIdx]

	for _, id := range reqAff {
		if !identifiers[id] {
			return 0, fmt.Errorf(
				"required affinity not satisfied: GPU %s missing identifier %q for pod <%s/%s>",
				gpuIdx, id, task.Namespace, task.Name)
		}
	}

	for _, id := range reqAntiAff {
		if identifiers[id] {
			return 0, fmt.Errorf(
				"required anti-affinity violated: GPU %s has identifier %q for pod <%s/%s>",
				gpuIdx, id, task.Namespace, task.Name)
		}
	}

	var score float64
	for _, id := range prefAff {
		if identifiers[id] {
			score += 1.0
		}
	}
	for _, id := range prefAntiAff {
		if identifiers[id] {
			score -= 1.0
		}
	}

	log.InfraLogger.V(7).Infof(
		"GPU device affinity: Task <%s/%s> gpuIdx <%s> node <%s> score %f",
		task.Namespace, task.Name, gpuIdx, node.Name, score)
	return score, nil
}

func gpuGroupIdentifiers(node *node_info.NodeInfo) map[string]map[string]bool {
	result := map[string]map[string]bool{}
	for _, podInfo := range node.PodInfos {
		if podInfo.Pod == nil || podInfo.Pod.Labels == nil {
			continue
		}
		identifier := podInfo.Pod.Labels[commonconstants.GPUSharingIdentifier]
		if identifier == "" {
			continue
		}
		for _, gpuGroup := range podInfo.GPUGroups {
			if result[gpuGroup] == nil {
				result[gpuGroup] = map[string]bool{}
			}
			result[gpuGroup][identifier] = true
		}
	}
	return result
}

func parseCSV(val string) []string {
	if val == "" {
		return nil
	}
	parts := strings.Split(val, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
