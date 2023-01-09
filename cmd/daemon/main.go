// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2023 Intel Corporation

package main

import (
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/intel-collab/applications.orchestration.operators.sriov-fec-operator/pkg/common/drainhelper"
	"github.com/intel-collab/applications.orchestration.operators.sriov-fec-operator/pkg/common/utils"
	"io/ioutil"
	"os"
	"syscall"

	"k8s.io/apimachinery/pkg/types"

	sriovv2 "github.com/intel-collab/applications.orchestration.operators.sriov-fec-operator/api/v2"
	"github.com/intel-collab/applications.orchestration.operators.sriov-fec-operator/pkg/daemon"

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
	syscall.Umask(0077)

	ctrl.SetLogger(logr.New(utils.NewLogWrapper()))

	nodeName := getNodeNameFromEnvOrDie()
	ns := getSriovFecNameSpaceFromEnvOrDie()

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
	mgr, err := daemon.CreateManager(config, scheme, ns, 8080, 8081, setupLog)
	if err != nil {
		setupLog.WithError(err).Error("unable to start manager")
		os.Exit(1)
	}

	daemon.StartTelemetryDaemon(mgr, nodeName, ns, directClient, setupLog)

	vfioTokenBytes, err := ioutil.ReadFile("/sriov_config/vfiotoken")
	if err != nil {
		setupLog.Error(err)
		os.Exit(1)
	}

	vfioToken, err := uuid.ParseBytes(vfioTokenBytes)
	if err != nil {
		setupLog.Errorf("provided vfioToken(%s) is not in UUID format: %s", vfioTokenBytes, err)
		os.Exit(1)
	}

	isSingleNodeCluster, err := utils.IsSingleNodeCluster(directClient)
	if err != nil {
		setupLog.WithError(err).Errorf("failed to determine cluster type")
		os.Exit(1)
	}

	nodeNameRef := types.NamespacedName{Namespace: ns, Name: nodeName}
	drainHelper := drainhelper.NewDrainHelper(utils.NewLogger(), cset, nodeName, ns, isSingleNodeCluster)
	pfBBConfigController := daemon.NewPfBBConfigController(utils.NewLogger(), vfioToken.String())
	nodeConfigurer := daemon.NewNodeConfigurator(utils.NewLogger(), pfBBConfigController, mgr.GetClient(), nodeNameRef)
	devicePluginController := daemon.NewDevicePluginController(mgr.GetClient(), utils.NewLogger(), nodeNameRef)

	reconciler, err := daemon.NewNodeConfigReconciler(mgr.GetClient(), drainHelper.Run, nodeNameRef, nodeConfigurer, devicePluginController.RestartDevicePlugin)
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

func getSriovFecNameSpaceFromEnvOrDie() string {
	ns := os.Getenv("SRIOV_FEC_NAMESPACE")
	if ns == "" {
		setupLog.Error("SRIOV_FEC_NAMESPACE environment variable is empty")
		os.Exit(1)
	}
	return ns
}

func getNodeNameFromEnvOrDie() string {
	nodeName := os.Getenv("NODENAME")
	if nodeName == "" {
		setupLog.Error("NODENAME environment variable is empty")
		os.Exit(1)
	}
	return nodeName
}
