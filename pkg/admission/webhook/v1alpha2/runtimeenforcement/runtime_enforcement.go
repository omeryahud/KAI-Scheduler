// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package runtimeenforcement

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NVIDIA/KAI-scheduler/pkg/binder/binding/resourcereservation"
	"github.com/NVIDIA/KAI-scheduler/pkg/binder/common"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/constants"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/k8s_utils"
	"github.com/NVIDIA/KAI-scheduler/pkg/common/resources"
)

type RuntimeEnforcement struct {
	kubeClient client.Client
}

func New(kubeClient client.Client) *RuntimeEnforcement {
	return &RuntimeEnforcement{
		kubeClient: kubeClient,
	}
}

func (p *RuntimeEnforcement) Name() string {
	return "runtimeenforcement"
}

func (p *RuntimeEnforcement) Validate(pod *v1.Pod) error {
	return nil
}

func (p *RuntimeEnforcement) Mutate(pod *v1.Pod) error {
	// nvidia runtimeClass is not supported on openshift
	isOpenshift, err := k8s_utils.IsOpenshift(context.Background(), p.kubeClient)
	if err != nil {
		return err
	} else if isOpenshift {
		return nil
	}

	// in order to no collide with custom reservation pods runtimeClass
	if resourcereservation.IsGPUReservationPod(pod) {
		return nil
	}

	if resources.RequestsGPU(pod) {
		exists, err := k8s_utils.RuntimeClassExists(context.Background(),
			p.kubeClient, constants.DefaultRuntimeClassName)
		if err != nil {
			return err
		} else if !exists {
			return nil
		}

		common.SetNVIDIARuntimeClass(pod)
	}

	return nil
}
