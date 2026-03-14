// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	kaiv1alpha1 "github.com/kai-scheduler/KAI-scheduler/pkg/apis/kai/v1alpha1"
	"github.com/kai-scheduler/KAI-scheduler/pkg/gpugrouptemplatecontroller/controllers"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kaiv1alpha1.AddToScheme(scheme))
}

func Run(options *Options, config *rest.Config, ctx context.Context) error {
	config.QPS = float32(options.Qps)
	config.Burst = options.Burst

	cacheOptions := cache.Options{}
	cacheOptions.ByObject = map[client.Object]cache.ByObject{
		&kaiv1alpha1.GPUGroupTemplate{}: {},
		&kaiv1alpha1.GPUGroup{}:         {},
	}

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		Cache:                  cacheOptions,
		HealthProbeBindAddress: options.ProbeAddr,
		LeaderElection:         options.EnableLeaderElection,
		LeaderElectionID:       "gpugrouptemplate.kai.scheduler",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}

	if options.EnableWebhook {
		if err = (&kaiv1alpha1.GPUGroupTemplate{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook for GPUGroupTemplate")
			return err
		}
	}

	configs := controllers.Configs{
		MaxConcurrentReconciles: options.MaxConcurrentReconciles,
	}
	if err = (&controllers.GPUGroupTemplateReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr, configs); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GPUGroupTemplate")
		return err
	}

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		return err
	}
	if err = mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		return err
	}

	setupLog.Info("starting manager")
	if err = mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}

	return nil
}
