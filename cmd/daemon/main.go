// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2024 Intel Corporation

package main

import (
	"flag"
	"os"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/intel/sriov-fec-operator/pkg/common/drainhelper"
	"github.com/intel/sriov-fec-operator/pkg/common/utils"

	"k8s.io/apimachinery/pkg/types"

	sriovv2 "github.com/intel/sriov-fec-operator/api/sriovfec/v2"
	vrbv1 "github.com/intel/sriov-fec-operator/api/sriovvrb/v1"
	"github.com/intel/sriov-fec-operator/pkg/daemon"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = utils.NewLogger()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(sriovv2.AddToScheme(scheme))
	utilruntime.Must(vrbv1.AddToScheme(scheme))
}

func initFecReconciler(mgr manager.Manager, drainHelper *drainhelper.DrainHelper, nodeNameRef types.NamespacedName,
	nodeConfigurer *daemon.NodeConfigurator, devicePluginController *daemon.DevicePluginController, directClient client.Client) error {

	isFecDevice, _, err := utils.FindAccelerator(daemon.FecConfigPath)
	if err != nil {
		return err
	}
	if !isFecDevice {
		setupLog.WithField("Reconciler", "FEC").Info("Not started, no device found")
		return nil
	}

	reconciler, err := daemon.FecNewNodeConfigReconciler(mgr.GetClient(), drainHelper.Run, nodeNameRef, nodeConfigurer, devicePluginController.RestartDevicePlugin)
	if err != nil {
		return err
	}

	if err := reconciler.SetupWithManager(mgr); err != nil {
		return err
	}

	if err := reconciler.CreateEmptyNodeConfigIfNeeded(directClient); err != nil {
		return err
	}

	return nil
}

func initVrbReconciler(mgr manager.Manager, drainHelper *drainhelper.DrainHelper, nodeNameRef types.NamespacedName,
	nodeConfigurer *daemon.NodeConfigurator, devicePluginController *daemon.DevicePluginController, directClient client.Client) error {

	isVrbDevice, _, err := utils.FindAccelerator(daemon.VrbConfigPath)
	if err != nil {
		return err
	}
	if !isVrbDevice {
		setupLog.WithField("Reconciler", "VRB").Info("Not started, no device found")
		return nil
	}

	reconciler, err := daemon.VrbNewNodeConfigReconciler(mgr.GetClient(), drainHelper.Run, nodeNameRef, nodeConfigurer, devicePluginController.RestartDevicePlugin)
	if err != nil {
		return err
	}

	if err := reconciler.SetupWithManager(mgr); err != nil {
		return err
	}

	if err := reconciler.CreateEmptyNodeConfigIfNeeded(directClient); err != nil {
		return err
	}

	return nil
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

	pfBbConfigCliCmd := flag.String("C", "", "CLI command string")
	flag.Usage = func() {
		daemon.ShowHelp()
	}
	flag.Parse()
	if *pfBbConfigCliCmd != "" {
		// Get the additional arguments after CLI command
		args := flag.Args()
		daemon.StartPfBbConfigCli(nodeName, ns, directClient, *pfBbConfigCliCmd, args, setupLog)
		return
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

	vfioTokenBytes, err := os.ReadFile("/sriov_config/vfiotoken")
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

	if err := initFecReconciler(mgr, drainHelper, nodeNameRef, nodeConfigurer, devicePluginController, directClient); err != nil {
		setupLog.WithError(err).Error("Fail to start FEC Reconciler")
		os.Exit(1)
	}

	if err := initVrbReconciler(mgr, drainHelper, nodeNameRef, nodeConfigurer, devicePluginController, directClient); err != nil {
		setupLog.WithError(err).Error("Fail to start VRB Reconciler")
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
