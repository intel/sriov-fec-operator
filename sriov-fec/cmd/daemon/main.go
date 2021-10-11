// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package main

import (
	dh "github.com/otcshare/openshift-operator/common/pkg/drainhelper"
	"github.com/otcshare/openshift-operator/common/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
	"os"

	sriovv2 "github.com/otcshare/openshift-operator/sriov-fec/api/v2"
	"github.com/otcshare/openshift-operator/sriov-fec/pkg/daemon"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = utils.NewLogger()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(sriovv2.AddToScheme(scheme))
}

func main() {
	ctrl.SetLogger(utils.NewLogWrapper())

	nodeName := os.Getenv("NODENAME")
	if nodeName == "" {
		setupLog.Error("NODENAME environment variable is empty")
		os.Exit(1)
	}

	ns := os.Getenv("SRIOV_FEC_NAMESPACE")
	if ns == "" {
		setupLog.Error("SRIOV_FEC_NAMESPACE environment variable is empty")
		os.Exit(1)
	}

	config := ctrl.GetConfigOrDie()
	directClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		setupLog.WithError(err).Error("failed to create direct client")
		os.Exit(1)
	}

	cset, err := clientset.NewForConfig(config)
	if err != nil {
		setupLog.WithError(err).Error("failed to create clientset")
		os.Exit(1)
	}

	mgr, err := daemon.CreateManager(config, ns, scheme)
	if err != nil {
		setupLog.WithError(err).Error("unable to start manager")
		os.Exit(1)
	}

	nodeNameRef := types.NamespacedName{Namespace: ns, Name: nodeName}
	drainHelper := dh.NewDrainHelper(utils.NewLogger(), cset, nodeName, ns)
	configurer, err := daemon.NewNodeConfigurer(drainHelper.Run, mgr.GetClient(), nodeNameRef)
	if err != nil {
		setupLog.WithError(err).Error("unable to create node configurer")
		os.Exit(1)
	}
	reconciler, err := daemon.NewNodeConfigReconciler(mgr.GetClient(), configurer, nodeNameRef)
	if err != nil {
		setupLog.WithError(err).Error("unable to create reconciler")
		os.Exit(1)
	}

	if err := reconciler.SetupWithManager(mgr); err != nil {
		setupLog.WithError(err).Error("unable to create controller", "controller", "NodeConfig")
		os.Exit(1)
	}

	if err := reconciler.CreateEmptyNodeConfigIfNeeded(directClient); err != nil {
		setupLog.WithError(err).Error("failed to create initial NodeConfig CR")
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.WithError(err).Error("problem running manager")
		os.Exit(1)
	}
}
