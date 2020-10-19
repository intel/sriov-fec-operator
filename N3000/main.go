// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package main

import (
	"context"
	"flag"
	"os"
	"time"

	secv1 "github.com/openshift/api/security/v1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	fpgav1 "github.com/otcshare/openshift-operator/N3000/api/v1"
	"github.com/otcshare/openshift-operator/N3000/controllers"
	"github.com/otcshare/openshift-operator/N3000/pkg/assets"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(secv1.AddToScheme(scheme))
	utilruntime.Must(promv1.AddToScheme(scheme))
	utilruntime.Must(fpgav1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	config := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "f3417634.intel.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.N3000ClusterReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("N3000Cluster"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "N3000Cluster")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	client, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "failed to create client")
		os.Exit(1)
	}
	if err := (&assets.Manager{
		Client:    client,
		Log:       ctrl.Log.WithName("asset_manager").WithName("n3000"),
		EnvPrefix: "N3000_",
		Assets: []assets.Asset{
			{
				Path:              "assets/100-driver-container.yaml",
				BlockingReadiness: assets.ReadinessPollConfig{Retries: 30, Delay: 20 * time.Second},
			},
			{
				Path: "assets/200-monitoring.yaml",
			},
		},
	}).LoadAndDeploy(context.Background()); err != nil {
		setupLog.Error(err, "failed to deploy the assets")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
