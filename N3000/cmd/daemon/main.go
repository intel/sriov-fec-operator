// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package main

import (
	"flag"
	"os"
	"os/exec"

	fpgav1 "github.com/open-ness/openshift-operator/N3000/api/v1"
	"github.com/open-ness/openshift-operator/N3000/pkg/daemon"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(fpgav1.AddToScheme(scheme))
}

func main() {
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	nodeName := os.Getenv("NODENAME")
	if nodeName == "" {
		setupLog.Error(nil, "NODENAME environment variable is empty")
		os.Exit(1)
	}

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		setupLog.Error(nil, "NAMESPACE environment variable is empty")
		os.Exit(1)
	}

	config := ctrl.GetConfigOrDie()

	cset, err := clientset.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "failed to create clientset")
		os.Exit(1)
	}

	directClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "failed to create direct client")
		os.Exit(1)
	}

	// Check if we are able to iterate the cards
	_, err = exec.Command("fpgainfo", "bmc").CombinedOutput()
	if err != nil {
		setupLog.Error(err, "fpgainfo bmc failed to run")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
		LeaderElection:     false,
		Namespace:          namespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	daemon := daemon.NewN3000NodeReconciler(mgr.GetClient(), cset, ctrl.Log.WithName("daemon"), nodeName, namespace)
	if err := daemon.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "N3000Cluster")
		os.Exit(1)
	}

	if err := daemon.CreateEmptyN3000NodeIfNeeded(directClient); err != nil {
		setupLog.Error(err, "failed to create initial n3000node CR")
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
