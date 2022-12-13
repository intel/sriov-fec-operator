// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2022 Intel Corporation

package daemon

import (
	"context"
	"errors"
	"fmt"
	"github.com/intel-collab/applications.orchestration.operators.sriov-fec-operator/pkg/common/utils"
	"github.com/sirupsen/logrus"
	"io/fs"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"os"
	"os/exec"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"strconv"
	"strings"
	"time"

	fec "github.com/intel-collab/applications.orchestration.operators.sriov-fec-operator/api/v2"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type ConfigurationConditionReason string

const (
	ConditionConfigured       string                       = "Configured"
	ConfigurationInProgress   ConfigurationConditionReason = "InProgress"
	ConfigurationFailed       ConfigurationConditionReason = "Failed"
	ConfigurationNotRequested ConfigurationConditionReason = "NotRequested"
	ConfigurationSucceeded    ConfigurationConditionReason = "Succeeded"
)

var (
	resyncPeriod          = time.Minute
	configPath            = "/sriov_config/config/accelerators.json"
	getSriovInventory     = GetSriovInventory
	supportedAccelerators utils.AcceleratorDiscoveryConfig
	procCmdlineFilePath   = "/host/proc/cmdline"
	kernelParams          = []string{"intel_iommu=on", "iommu=pt"}
)

type NodeConfigReconciler struct {
	client.Client
	log                 *logrus.Logger
	nodeNameRef         types.NamespacedName
	drainerAndExecute   DrainAndExecute
	configurer          Configurer
	restartDevicePlugin RestartDevicePluginFunction
}

type DrainAndExecute func(configurer func(ctx context.Context) bool, drain bool) error

type Configurer interface {
	ApplySpec(nodeConfig fec.SriovFecNodeConfigSpec) error
}

type RestartDevicePluginFunction func() error

func NewNodeConfigReconciler(k8sClient client.Client, drainer DrainAndExecute,
	nodeNameRef types.NamespacedName, configurer Configurer,
	restartDevicePluginFunction RestartDevicePluginFunction) (r *NodeConfigReconciler, err error) {

	if supportedAccelerators, err = utils.LoadDiscoveryConfig(configPath); err != nil {
		return nil, err
	}

	return &NodeConfigReconciler{Client: k8sClient, drainerAndExecute: drainer,
		log: utils.NewLogger(), nodeNameRef: nodeNameRef, configurer: configurer, restartDevicePlugin: restartDevicePluginFunction}, nil
}

func (r *NodeConfigReconciler) Reconcile(_ context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Infof("Reconcile(...) triggered by %s", req.NamespacedName.String())

	nc, err := r.readSriovFecNodeConfig(req.NamespacedName)
	if err != nil {
		return requeueNowWithError(err)
	}

	if err := validateNodeConfig(nc.Spec); err != nil {
		return requeueNowWithError(r.updateStatus(nc, metav1.ConditionFalse, ConfigurationFailed, err.Error()))
	}

	detectedInventory, err := r.readExistingInventory()
	if err != nil {
		return requeueNowWithError(err)
	}

	if isConfigurationOfNonExistingInventoryRequested(nc.Spec.PhysicalFunctions, detectedInventory) {
		r.log.Info("requested configuration refers to not existing accelerator(s)")
		return requeueLaterOrNowIfError(r.updateStatus(nc, metav1.ConditionFalse, ConfigurationFailed, "requested configuration refers to not existing accelerator"))
	}

	if !r.isCardUpdateRequired(nc, detectedInventory) {
		r.log.Info("Nothing to do")
		return requeueLater()
	}

	if err := r.updateStatus(nc, metav1.ConditionFalse, ConfigurationInProgress, "Configuration started"); err != nil {
		return requeueNowWithError(err)
	}

	if err := r.configureNode(nc); err != nil {
		r.log.WithError(err).Error("error occurred during configuring node")
		return requeueNowWithError(r.updateStatus(nc, metav1.ConditionFalse, ConfigurationFailed, err.Error()))
	} else {
		return requeueLaterOrNowIfError(r.updateStatus(nc, metav1.ConditionTrue, ConfigurationSucceeded, "Configured successfully"))
	}
}

// CreateEmptyNodeConfigIfNeeded creates empty CR to be Reconciled in near future and filled with Status.
// If invoked before manager's Start, it'll need a direct API client
// (Manager's/Controller's client is cached and cache is not initialized yet).
func (r *NodeConfigReconciler) CreateEmptyNodeConfigIfNeeded(c client.Client) error {
	nodeConfig := &fec.SriovFecNodeConfig{}

	err := c.Get(context.Background(), client.ObjectKey{Name: r.nodeNameRef.Name, Namespace: r.nodeNameRef.Namespace}, nodeConfig)
	if err == nil {
		r.log.Info("already exists")
		return nil
	}

	if !k8serrors.IsNotFound(err) {
		return err
	}

	r.log.Infof("SriovFecNodeConfig{%s} not found - creating", r.nodeNameRef)

	nodeConfig = &fec.SriovFecNodeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.nodeNameRef.Name,
			Namespace: r.nodeNameRef.Namespace,
		},
		Spec: fec.SriovFecNodeConfigSpec{
			PhysicalFunctions: []fec.PhysicalFunctionConfigExt{},
		},
	}

	if createErr := c.Create(context.Background(), nodeConfig); createErr != nil {
		r.log.WithError(createErr).Error("failed to create")
		return createErr
	}

	meta.SetStatusCondition(&nodeConfig.Status.Conditions, metav1.Condition{
		Type:               ConditionConfigured,
		Status:             metav1.ConditionFalse,
		Reason:             string(ConfigurationNotRequested),
		Message:            "",
		ObservedGeneration: nodeConfig.GetGeneration(),
	})

	if inv, err := r.readExistingInventory(); err != nil {
		return err
	} else {
		nodeConfig.Status.Inventory = *inv
	}

	if updateErr := c.Status().Update(context.Background(), nodeConfig); updateErr != nil {
		r.log.WithError(updateErr).Error("failed to update cr status")
		return updateErr
	}

	return nil
}

func (r *NodeConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&fec.SriovFecNodeConfig{}).
		WithEventFilter(
			predicate.And(
				resourceNamePredicate{
					requiredName: r.nodeNameRef.Name,
					log:          r.log,
				},
				predicate.GenerationChangedPredicate{},
			),
		).Complete(r)
}

func (r *NodeConfigReconciler) updateStatus(nc *fec.SriovFecNodeConfig, status metav1.ConditionStatus, reason ConfigurationConditionReason, msg string) error {
	previousCondition := findOrCreateConfigurationStatusCondition(nc)

	// SriovFecNodeConfig.generation is under K8S management
	// metav1.Condition.observedGeneration is under this reconciler management.
	// observedGeneration would be incremented then and only then when spec which comes with updated generation would be processed without any error.
	determineGeneration := func() int64 {
		if reason == ConfigurationSucceeded {
			return nc.GetGeneration()
		} else {
			return previousCondition.ObservedGeneration
		}
	}

	condition := metav1.Condition{
		Type:               ConditionConfigured,
		Status:             status,
		Reason:             string(reason),
		Message:            msg,
		ObservedGeneration: determineGeneration(),
	}

	meta.SetStatusCondition(&nc.Status.Conditions, condition)
	if inv, err := getSriovInventory(r.log); err != nil {
		r.log.WithError(err).
			WithField("reason", condition.Reason).
			WithField("message", condition.Message).
			Error("failed to obtain sriov inventory for the node")
	} else {
		nc.Status.Inventory = *inv
	}

	if err := r.Status().Update(context.Background(), nc); err != nil {
		return err
	}

	r.log.WithField("previous", previousCondition).
		WithField("current", condition).
		Infof("%s condition transition", ConditionConfigured)

	return nil
}

func (r *NodeConfigReconciler) readExistingInventory() (*fec.NodeInventory, error) {
	inv, err := getSriovInventory(r.log)
	if err != nil {
		r.log.WithError(err).Error("failed to obtain sriov inventory for the node")
	}
	return inv, err
}

func (r *NodeConfigReconciler) readSriovFecNodeConfig(nn types.NamespacedName) (nc *fec.SriovFecNodeConfig, err error) {
	getSriovFecNodeConfig := func() (*fec.SriovFecNodeConfig, error) {
		sfnc := new(fec.SriovFecNodeConfig)
		if err := r.Client.Get(context.TODO(), nn, sfnc); err != nil {
			return nil, err
		}
		return sfnc, nil
	}

	if nc, err = getSriovFecNodeConfig(); err != nil {
		if !k8serrors.IsNotFound(err) {
			r.log.WithError(err).Error("Get() failed")
			return nil, err
		}

		r.log.Info("SriovFecNodeConfig not found - creating")
		if err := r.CreateEmptyNodeConfigIfNeeded(r.Client); err != nil {
			r.log.WithError(err).Error("Couldn't create SriovFecNodeConfig")
			return nil, err
		}

		if nc, err = getSriovFecNodeConfig(); err != nil {
			return nil, err
		}
	}

	return nc, nil
}

func (r *NodeConfigReconciler) configureNode(nodeConfig *fec.SriovFecNodeConfig) error {
	var configurationError error

	drainFunc := func(ctx context.Context) bool {
		if err := r.configurer.ApplySpec(nodeConfig.Spec); err != nil {
			r.log.WithError(err).Error("failed applying new PF/VF configuration")
			configurationError = err
			return true
		}

		configurationError = r.restartDevicePlugin()
		return true
	}

	if err := r.drainerAndExecute(drainFunc, !nodeConfig.Spec.DrainSkip); err != nil {
		return err
	}

	return configurationError
}

func (r *NodeConfigReconciler) isCardUpdateRequired(nc *fec.SriovFecNodeConfig, detectedInventory *fec.NodeInventory) bool {
	pciToVfsAmount := map[string]int{}
	for _, physicalFunction := range nc.Spec.PhysicalFunctions {
		pciToVfsAmount[physicalFunction.PCIAddress] = physicalFunction.VFAmount
	}

	isGenerationChanged := func() bool {
		observedGeneration := findOrCreateConfigurationStatusCondition(nc).ObservedGeneration
		if nc.GetGeneration() != observedGeneration {
			r.log.WithField("observed", observedGeneration).
				WithField("requested", nc.GetGeneration()).
				Info("Observed generation doesn't reflect requested one")
			return true
		}
		return false
	}

	exposedInventoryOutdated := func() bool {
		for _, accelerator := range detectedInventory.SriovAccelerators {
			if len(accelerator.VFs) != pciToVfsAmount[accelerator.PCIAddress] {
				r.log.WithField("pciAddress", accelerator.PCIAddress).
					WithField("exposedVfs", len(accelerator.VFs)).
					WithField("requestedVfs", pciToVfsAmount[accelerator.PCIAddress]).
					Info("Exposed inventory doesn't match requested one")
				return true
			}
		}
		return false
	}

	bbDevConfigDaemonIsDead := func() bool {
		for _, acc := range nc.Spec.PhysicalFunctions {
			if strings.EqualFold(acc.PFDriver, utils.VFIO_PCI) {
				if pfBbConfigProcIsDead(r.log, acc.PCIAddress) {
					r.log.WithField("pciAddress", acc.PCIAddress).
						Info("pf-bb-config process for card is not running")
					return true
				}
			}
		}
		return false
	}

	return isGenerationChanged() || exposedInventoryOutdated() || bbDevConfigDaemonIsDead()
}

func pfBbConfigProcIsDead(log *logrus.Logger, pciAddr string) bool {
	stdout, err := execCmd([]string{
		"chroot",
		"/host/",
		"pgrep",
		"--count",
		"--full",
		fmt.Sprintf("pf_bb_config.*%s", pciAddr),
	}, log)
	if err != nil {
		log.WithError(err).Error("failed to determine status of pf-bb-config daemon")
		return true
	}
	matchingProcCount, err := strconv.Atoi(stdout[0:1]) //stdout contains characters like '\n', so we are removing them
	if err != nil {
		log.WithError(err).Error("failed to convert 'pgrep' output to int")
		return true
	}
	return matchingProcCount == 0
}

func isReady(p corev1.Pod) bool {
	for _, condition := range p.Status.Conditions {
		if condition.Type == corev1.PodReady && p.Status.Phase == corev1.PodRunning {
			return true
		}
	}
	return false
}

func findOrCreateConfigurationStatusCondition(nc *fec.SriovFecNodeConfig) metav1.Condition {
	configurationStatusCondition := nc.FindCondition(ConditionConfigured)
	if configurationStatusCondition == nil {
		return metav1.Condition{
			Type:               ConditionConfigured,
			Status:             metav1.ConditionTrue,
			Reason:             string(ConfigurationNotRequested),
			ObservedGeneration: 0,
		}
	}

	return *configurationStatusCondition
}

// returns error if requested configuration refers to not existing inventory/accelerator
func isConfigurationOfNonExistingInventoryRequested(requestedConfiguration []fec.PhysicalFunctionConfigExt, existingInventory *fec.NodeInventory) bool {
OUTER:
	for _, pf := range requestedConfiguration {
		for _, acc := range existingInventory.SriovAccelerators {
			if acc.PCIAddress == pf.PCIAddress {
				continue OUTER
			}
		}

		return true
	}
	return false
}

func CreateManager(config *rest.Config, scheme *runtime.Scheme, namespace string, metricsPort int, HealthProbePort int, log *logrus.Logger) (manager.Manager, error) {
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     ":" + strconv.Itoa(metricsPort),
		LeaderElection:         false,
		Namespace:              namespace,
		HealthProbeBindAddress: ":" + strconv.Itoa(HealthProbePort),
	})
	if err != nil {
		return nil, err
	}

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		log.WithError(err).Error("unable to set up health check")
		return nil, err
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		log.WithError(err).Error("unable to set up ready check")
		return nil, err
	}
	return mgr, nil
}

func validateNodeConfig(nodeConfig fec.SriovFecNodeConfigSpec) error {
	cmdlineBytes, err := os.ReadFile(procCmdlineFilePath)
	if err != nil {
		return fmt.Errorf("failed to read file contents: path: %v, error - %v", procCmdlineFilePath, err)
	}
	cmdline := string(cmdlineBytes)
	//common attributes for SRIOV
	if err := validateOrdinalKernelParams(cmdline); err != nil {
		return err
	}

	for _, physFunc := range nodeConfig.PhysicalFunctions {
		switch physFunc.PFDriver {
		case utils.PCI_PF_STUB_DASH, utils.PCI_PF_STUB_UNDERSCORE, utils.IGB_UIO:
			dmesgBytes, err := exec.Command("dmesg").Output()
			if err != nil {
				return fmt.Errorf("failed to run 'dmesg' - %v", err.Error())
			}
			dmesgOut := string(dmesgBytes)
			if strings.Contains(dmesgOut, "Secure boot enabled") {
				return fmt.Errorf("'%s' driver doesn't supports SecureBoot. It is supported only by 'vfio-pci'", physFunc.PFDriver)
			}
		case utils.VFIO_PCI:
			err := moduleParameterIsEnabled(utils.VFIO_PCI_UNDERSCORE, "enable_sriov")
			if err != nil {
				return err
			}
			err = moduleParameterIsEnabled(utils.VFIO_PCI_UNDERSCORE, "disable_idle_d3")
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown driver '%s'", physFunc.PFDriver)
		}
	}
	return nil
}

func moduleParameterIsEnabled(moduleName, parameter string) error {
	value, err := os.ReadFile("/sys/module/" + moduleName + "/parameters/" + parameter)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// module is not loaded - we will automatically append required parameter during modprobe
			return nil
		} else {
			return fmt.Errorf("failed to check parameter %v for %v module - %v", parameter, moduleName, err)
		}
	}

	if strings.Contains(strings.ToLower(string(value)), "n") {
		return fmt.Errorf(moduleName + " is loaded and doesn't has " + parameter + " set")
	}
	return nil
}

func validateOrdinalKernelParams(cmdline string) error {
	for _, param := range kernelParams {
		if !strings.Contains(cmdline, param) {
			return fmt.Errorf("missing kernel param(%s)", param)
		}
	}
	return nil
}
