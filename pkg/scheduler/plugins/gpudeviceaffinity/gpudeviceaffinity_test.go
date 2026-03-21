// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package gpudeviceaffinity

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	commonconstants "github.com/kai-scheduler/KAI-scheduler/pkg/common/constants"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/kai-scheduler/KAI-scheduler/pkg/scheduler/api/resource_info"
)

func TestGpuDeviceAffinity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GpuDeviceAffinity Suite")
}

func createTask(labels map[string]string) *pod_info.PodInfo {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels:    labels,
		},
	}
	return pod_info.NewTaskInfo(pod, nil, resource_info.NewResourceVectorMap())
}

func createNodeWithPods(pods map[common_info.PodID]*pod_info.PodInfo) *node_info.NodeInfo {
	return &node_info.NodeInfo{
		Name:     "test-node",
		PodInfos: pods,
		GpuSharingNodeInfo: node_info.GpuSharingNodeInfo{
			UsedSharedGPUsMemory: map[string]int64{"0": 512},
		},
		MemoryOfEveryGpuOnNode: 1024,
	}
}

func createExistingPod(name, identifier string, gpuGroups []string) *pod_info.PodInfo {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				commonconstants.GPUSharingIdentifier: identifier,
			},
		},
	}
	pi := pod_info.NewTaskInfo(pod, nil, resource_info.NewResourceVectorMap())
	pi.GPUGroups = gpuGroups
	return pi
}

var _ = Describe("GPU device affinity scoring", func() {
	It("No affinity labels", func() {
		task := createTask(nil)
		nodeInfo := createNodeWithPods(map[common_info.PodID]*pod_info.PodInfo{})
		score, err := gpuOrderFn(task, nodeInfo, "0")
		Expect(err).NotTo(HaveOccurred())
		Expect(score).To(Equal(0.0))
	})

	It("Required affinity satisfied", func() {
		existing := createExistingPod("existing", "B", []string{"0"})
		nodeInfo := createNodeWithPods(map[common_info.PodID]*pod_info.PodInfo{
			"existing": existing,
		})
		task := createTask(map[string]string{
			commonconstants.GPUSharingGroupRequiredAffinity: "B",
		})
		score, err := gpuOrderFn(task, nodeInfo, "0")
		Expect(err).NotTo(HaveOccurred())
		Expect(score).To(Equal(0.0))
	})

	It("Required affinity violated", func() {
		existing := createExistingPod("existing", "C", []string{"0"})
		nodeInfo := createNodeWithPods(map[common_info.PodID]*pod_info.PodInfo{
			"existing": existing,
		})
		task := createTask(map[string]string{
			commonconstants.GPUSharingGroupRequiredAffinity: "B",
		})
		_, err := gpuOrderFn(task, nodeInfo, "0")
		Expect(err).To(HaveOccurred())
	})

	It("Required anti-affinity satisfied", func() {
		existing := createExistingPod("existing", "B", []string{"0"})
		nodeInfo := createNodeWithPods(map[common_info.PodID]*pod_info.PodInfo{
			"existing": existing,
		})
		task := createTask(map[string]string{
			commonconstants.GPUSharingGroupRequiredAntiAffinity: "A",
		})
		score, err := gpuOrderFn(task, nodeInfo, "0")
		Expect(err).NotTo(HaveOccurred())
		Expect(score).To(Equal(0.0))
	})

	It("Required anti-affinity violated", func() {
		existing := createExistingPod("existing", "A", []string{"0"})
		nodeInfo := createNodeWithPods(map[common_info.PodID]*pod_info.PodInfo{
			"existing": existing,
		})
		task := createTask(map[string]string{
			commonconstants.GPUSharingGroupRequiredAntiAffinity: "A",
		})
		_, err := gpuOrderFn(task, nodeInfo, "0")
		Expect(err).To(HaveOccurred())
	})

	It("Preferred affinity match", func() {
		existing := createExistingPod("existing", "B", []string{"0"})
		nodeInfo := createNodeWithPods(map[common_info.PodID]*pod_info.PodInfo{
			"existing": existing,
		})
		task := createTask(map[string]string{
			commonconstants.GPUSharingGroupPreferredAffinity: "B,C",
		})
		score, err := gpuOrderFn(task, nodeInfo, "0")
		Expect(err).NotTo(HaveOccurred())
		Expect(score).To(Equal(1.0))
	})

	It("Preferred anti-affinity match", func() {
		existing := createExistingPod("existing", "A", []string{"0"})
		nodeInfo := createNodeWithPods(map[common_info.PodID]*pod_info.PodInfo{
			"existing": existing,
		})
		task := createTask(map[string]string{
			commonconstants.GPUSharingGroupPreferredAntiAffinity: "A",
		})
		score, err := gpuOrderFn(task, nodeInfo, "0")
		Expect(err).NotTo(HaveOccurred())
		Expect(score).To(Equal(-1.0))
	})

	It("WholeGpuIndicator allowed for affinity-aware pod", func() {
		nodeInfo := createNodeWithPods(map[common_info.PodID]*pod_info.PodInfo{})
		task := createTask(map[string]string{
			commonconstants.GPUSharingGroupRequiredAffinity: "B",
		})
		score, err := gpuOrderFn(task, nodeInfo, pod_info.WholeGpuIndicator)
		Expect(err).NotTo(HaveOccurred())
		Expect(score).To(Equal(0.0))
	})

	It("Multiple identifiers on GPU", func() {
		pod1 := createExistingPod("pod1", "A", []string{"0"})
		pod2 := createExistingPod("pod2", "B", []string{"0"})
		nodeInfo := createNodeWithPods(map[common_info.PodID]*pod_info.PodInfo{
			"pod1": pod1,
			"pod2": pod2,
		})
		task := createTask(map[string]string{
			commonconstants.GPUSharingGroupPreferredAffinity: "A,B",
		})
		score, err := gpuOrderFn(task, nodeInfo, "0")
		Expect(err).NotTo(HaveOccurred())
		Expect(score).To(Equal(2.0))
	})

	It("Empty CSV label", func() {
		nodeInfo := createNodeWithPods(map[common_info.PodID]*pod_info.PodInfo{})
		task := createTask(map[string]string{
			commonconstants.GPUSharingGroupRequiredAffinity: "",
		})
		score, err := gpuOrderFn(task, nodeInfo, "0")
		Expect(err).NotTo(HaveOccurred())
		Expect(score).To(Equal(0.0))
	})

	It("Unaware pod excluded from GPU with identifiers", func() {
		existing := createExistingPod("existing", "A", []string{"0"})
		nodeInfo := createNodeWithPods(map[common_info.PodID]*pod_info.PodInfo{
			"existing": existing,
		})
		task := createTask(nil)
		_, err := gpuOrderFn(task, nodeInfo, "0")
		Expect(err).To(HaveOccurred())
	})

	It("Unaware pod allowed on GPU without identifiers", func() {
		nodeInfo := createNodeWithPods(map[common_info.PodID]*pod_info.PodInfo{})
		task := createTask(nil)
		score, err := gpuOrderFn(task, nodeInfo, "0")
		Expect(err).NotTo(HaveOccurred())
		Expect(score).To(Equal(0.0))
	})

	It("Unaware pod allowed on WholeGpuIndicator", func() {
		existing := createExistingPod("existing", "A", []string{"0"})
		nodeInfo := createNodeWithPods(map[common_info.PodID]*pod_info.PodInfo{
			"existing": existing,
		})
		task := createTask(nil)
		score, err := gpuOrderFn(task, nodeInfo, pod_info.WholeGpuIndicator)
		Expect(err).NotTo(HaveOccurred())
		Expect(score).To(Equal(0.0))
	})
})
