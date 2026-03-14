// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"flag"
)

type Options struct {
	ProbeAddr               string
	EnableLeaderElection    bool
	Qps                     int
	Burst                   int
	MaxConcurrentReconciles int
	LogLevel                int
	SchedulerName           string
	EnableGPUGroupWebhook   bool
}

func InitOptions(fs *flag.FlagSet) *Options {
	options := &Options{}

	if fs == nil {
		fs = flag.CommandLine
	}

	fs.StringVar(&options.ProbeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	fs.BoolVar(&options.EnableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager.")
	fs.IntVar(&options.Qps, "qps", 50,
		"Queries per second to the K8s API server")
	fs.IntVar(&options.Burst, "burst", 300,
		"Burst to the K8s API server")
	fs.IntVar(&options.MaxConcurrentReconciles, "max-concurrent-reconciles", 10,
		"Max concurrent reconciles")
	fs.IntVar(&options.LogLevel, "log-level", 3,
		"Log level")
	fs.StringVar(&options.SchedulerName, "scheduler-name", "kai-scheduler",
		"The name of the scheduler")
	fs.BoolVar(&options.EnableGPUGroupWebhook, "enable-gpugroup-webhook", true,
		"Enable GPUGroup validation webhook")

	return options
}
