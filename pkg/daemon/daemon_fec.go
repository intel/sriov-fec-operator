// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2023 Intel Corporation

package daemon

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/intel/sriov-fec-operator/pkg/common/utils"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"

	fec "github.com/intel/sriov-fec-operator/api/sriovfec/v2"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var (
	FecConfigPath           = "/sriov_config/config/accelerators.json"
	getSriovInventory       = GetSriovInventory
	supportedAccelerators   utils.AcceleratorDiscoveryConfig
	fecPreviousConfig       = make(map[string]fec.PhysicalFunctionConfigExt)
	fecCurrentConfig        = make(map[string]fec.PhysicalFunctionConfigExt)
	fecDeviceUpdateRequired = make(map[string]bool)
)

type FecNodeConfigReconciler struct {
	client.Client
	log                 *logrus.Logger
	nodeNameRef         types.NamespacedName
	drainerAndExecute   DrainAndExecute
	sriovfecconfigurer  Configurer
	restartDevicePlugin RestartDevicePluginFunction
}

type Configurer interface {
	ApplySpec(nodeConfig fec.SriovFecNodeConfigSpec, fecDeviceUpdateRequired map[string]bool) error
}

/*****************************************************************************
 * Method: FecNodeConfigReconciler::Reconcile
 * Description:
 *
 ****************************************************************************/
func (r *FecNodeConfigReconciler) Reconcile(_ context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Debugf("Reconcile(...) triggered by %s", req.NamespacedName.String())

	sfnc, err := r.readNodeConfig(req.NamespacedName)
	if err != nil {
		return requeueNowWithError(err)
	}

	if err := validateNodeConfig(sfnc.Spec); err != nil {
		return requeueNowWithError(r.updateStatus(sfnc, metav1.ConditionFalse, ConfigurationFailed, err.Error()))
	}

	detectedInventory, err := r.readExistingInventory()
	if err != nil {
		return requeueNowWithError(err)
	}

	if isConfigurationOfNonExistingInventoryRequested(sfnc.Spec.PhysicalFunctions, detectedInventory) {
		r.log.Info("requested configuration refers to not existing accelerator(s)")
		return requeueLaterOrNowIfError(r.updateStatus(sfnc, metav1.ConditionFalse, ConfigurationFailed, "requested configuration refers to not existing accelerator"))
	}

	for _, accelerator := range detectedInventory.SriovAccelerators {
		fecDeviceUpdateRequired[accelerator.PCIAddress] = true
	}

	if !r.isCardUpdateRequired(sfnc, detectedInventory) {
		r.log.Debug("SriovFec: Nothing to do")
		return requeueLater()
	}

	if err := r.updateStatus(sfnc, metav1.ConditionFalse, ConfigurationInProgress, "Configuration started"); err != nil {
		return requeueNowWithError(err)
	}

	if err := r.configureNode(sfnc); err != nil {
		r.log.WithError(err).Error("error occurred during configuring node")
		return requeueNowWithError(r.updateStatus(sfnc, metav1.ConditionFalse, ConfigurationFailed, err.Error()))
	}

	return requeueLaterOrNowIfError(r.updateStatus(sfnc, metav1.ConditionTrue, ConfigurationSucceeded, "Configured successfully"))
}

/*****************************************************************************
 * Method: FecNodeConfigReconciler::
 * Description:
 * CreateEmptyNodeConfigIfNeeded creates empty CR to be Reconciled in
 * near future and filled with Status.
 * If invoked before manager's Start, it'll need a direct API client
 * (Manager's/Controller's client is cached and cache is not initialized yet).
 ****************************************************************************/
func (r *FecNodeConfigReconciler) CreateEmptyNodeConfigIfNeeded(c client.Client) error {
	SriovFecnodeConfig := &fec.SriovFecNodeConfig{}

	err := c.Get(context.Background(), client.ObjectKey{Name: r.nodeNameRef.Name, Namespace: r.nodeNameRef.Namespace}, SriovFecnodeConfig)
	if err == nil {
		r.log.Info("already exists")
		return nil
	}

	if !k8serrors.IsNotFound(err) {
		return err
	}

	r.log.Infof("SriovFecNodeConfig{%s} not found - creating", r.nodeNameRef)

	SriovFecnodeConfig = &fec.SriovFecNodeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.nodeNameRef.Name,
			Namespace: r.nodeNameRef.Namespace,
		},
		Spec: fec.SriovFecNodeConfigSpec{
			PhysicalFunctions: []fec.PhysicalFunctionConfigExt{},
		},
	}

	if createErr := c.Create(context.Background(), SriovFecnodeConfig); createErr != nil {
		r.log.WithError(createErr).Error("failed to create")
		return createErr
	}

	meta.SetStatusCondition(&SriovFecnodeConfig.Status.Conditions, metav1.Condition{
		Type:               ConditionConfigured,
		Status:             metav1.ConditionFalse,
		Reason:             string(ConfigurationNotRequested),
		Message:            "",
		ObservedGeneration: SriovFecnodeConfig.GetGeneration(),
	})

	if inv, err := r.readExistingInventory(); err != nil {
		return err
	} else {
		SriovFecnodeConfig.Status.Inventory = *inv
	}

	SriovFecnodeConfig.Status.PfBbConfVersion = r.getPfBbConfVersion()

	if updateErr := c.Status().Update(context.Background(), SriovFecnodeConfig); updateErr != nil {
		r.log.WithError(updateErr).Error("failed to update cr status")
		return updateErr
	}

	return nil
}

/*****************************************************************************
 * Method: FecNodeConfigReconciler::
 * Description:
 *
 ****************************************************************************/
func (r *FecNodeConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {

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

/*****************************************************************************
 * Method: FecNodeConfigReconciler::updateStatus
 * Description: Updates the status of the SriovFecNodeConfig resource.
 * Returns error if the status update fails
 ****************************************************************************/
func (r *FecNodeConfigReconciler) updateStatus(nc *fec.SriovFecNodeConfig, status metav1.ConditionStatus, reason ConfigurationConditionReason, msg string) error {
	previousCondition := findOrCreateConfigurationStatusCondition(nc)

	if reason == ConfigurationInProgress {
		// Clear the current configuration map
		for key := range fecCurrentConfig {
			delete(fecCurrentConfig, key)
		}
		// Update the current configuration map with the new configuration
		for _, pf := range nc.Spec.PhysicalFunctions {
			fecCurrentConfig[pf.PCIAddress] = pf
		}
		r.checkIfDeviceUpdateNeeded(fecPreviousConfig, fecCurrentConfig)
	} else if reason == ConfigurationSucceeded {
		// Clear the previous configuration map
		for key := range fecPreviousConfig {
			delete(fecPreviousConfig, key)
		}
		// Update the previous configuration with the current configuration
		for _, pf := range nc.Spec.PhysicalFunctions {
			fecPreviousConfig[pf.PCIAddress] = pf
		}
	}

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

/*****************************************************************************
 * Method: NodeConfigReconciler::
 * Description:
 *
 ****************************************************************************/
func (r *FecNodeConfigReconciler) readExistingInventory() (*fec.NodeInventory, error) {
	inv, err := getSriovInventory(r.log)
	if err != nil {
		r.log.WithError(err).Error("failed to obtain sriov inventory for the node")
	}
	return inv, err
}

/*****************************************************************************
 * Method: FecNodeConfigReconciler::readNodeConfig
 * Description:
 *
 ****************************************************************************/
func (r *FecNodeConfigReconciler) readNodeConfig(nn types.NamespacedName) (nc *fec.SriovFecNodeConfig, err error) {
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

/*****************************************************************************
 * Method: FecNodeConfigReconciler::
 * Description:
 *
 ****************************************************************************/
func (r *FecNodeConfigReconciler) configureNode(nodeConfig *fec.SriovFecNodeConfig) error {
	var configurationError error

	drainFunc := func(ctx context.Context) bool {
		if err := r.sriovfecconfigurer.ApplySpec(nodeConfig.Spec, fecDeviceUpdateRequired); err != nil {
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

/*****************************************************************************
 * Method: bbDevConfigDaemonIsDead:
 * Description:
 *
 ****************************************************************************/
func (r *FecNodeConfigReconciler) bbDevConfigDaemonIsDead(nc *fec.SriovFecNodeConfig) bool {

	for _, acc := range nc.Spec.PhysicalFunctions {
		if strings.EqualFold(acc.PFDriver, utils.VfioPci) {
			if pfBbConfigProcIsDead(r.log, acc.PCIAddress) {
				r.log.WithField("pciAddress", acc.PCIAddress).
					Info("pf-bb-config process for card is not running")
				return true
			}
		}
	}
	return false
}

/*****************************************************************************
 * Method: FecNodeConfigReconciler::
 * Description:
 *
 ****************************************************************************/
func (r *FecNodeConfigReconciler) isCardUpdateRequired(nc *fec.SriovFecNodeConfig, detectedInventory *fec.NodeInventory) bool {
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

	if len(nc.Spec.PhysicalFunctions) == 0 {
		if isGenerationChanged() {
			return true
		}
		r.log.Debug("Empty SriovFec PF")
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

	return isGenerationChanged() || exposedInventoryOutdated() || r.bbDevConfigDaemonIsDead(nc)
}

/*****************************************************************************
 * Function: FecNewNodeConfigReconciler
 * Description:
 *
 ****************************************************************************/
func FecNewNodeConfigReconciler(k8sClient client.Client, drainer DrainAndExecute,
	nodeNameRef types.NamespacedName, sriovfecconfigurer Configurer,
	restartDevicePluginFunction RestartDevicePluginFunction) (r *FecNodeConfigReconciler, err error) {

	if supportedAccelerators, err = utils.LoadDiscoveryConfig(FecConfigPath); err != nil {
		return nil, err
	}

	return &FecNodeConfigReconciler{
		Client:              k8sClient,
		drainerAndExecute:   drainer,
		log:                 utils.NewLogger(),
		nodeNameRef:         nodeNameRef,
		sriovfecconfigurer:  sriovfecconfigurer,
		restartDevicePlugin: restartDevicePluginFunction,
	}, nil
}

/******************************************************************************
 * Function: findOrCreateConfigurationStatusCondition
 * Description:
 *
 *****************************************************************************/
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

/******************************************************************************
 * Function: findOrCreateConfigurationStatusCondition
 * Description: Returns error if requested configuration refers to not existing inventory/accelerator
 *****************************************************************************/
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

func validateVfioDriverOptions(physFunc fec.PhysicalFunctionConfigExt) error {

	err := moduleParameterIsEnabled(utils.VfioPciUnderscore, "enable_sriov")
	if err != nil {
		return err
	}

	// Need to skip disable_idle_d3 check for VRB devices, only check
	// this parameter when configuring ACC100 and N3000 device
	if physFunc.BBDevConfig.ACC100 != nil || physFunc.BBDevConfig.N3000 != nil {
		err = moduleParameterIsEnabled(utils.VfioPciUnderscore, "disable_idle_d3")
	}
	return err
}

func validateNodeConfig(nodeConfig fec.SriovFecNodeConfigSpec) error {
	cmdlineBytes, err := os.ReadFile(procCmdlineFilePath)
	if err != nil {
		return fmt.Errorf("failed to read file contents: path: %v, error - %v", procCmdlineFilePath, err)
	}
	cmdline := string(cmdlineBytes)
	// Common attributes for SRIOV
	if err := validateOrdinalKernelParams(cmdline); err != nil {
		return err
	}

	for _, physFunc := range nodeConfig.PhysicalFunctions {
		switch physFunc.PFDriver {
		case utils.PciPfStubDash, utils.PciPfStubUnderscore, utils.IgbUio:
			cmdlineBytes, err = os.ReadFile(sysLockdownFilePath)
			if err != nil {
				return fmt.Errorf("failed to read file contents: path: %v, error - %v", sysLockdownFilePath, err)
			}
			cmdline = string(cmdlineBytes)
			if !strings.Contains(cmdline, "[none]") {
				return fmt.Errorf("kernel lockdown is enabled, '%s' driver doesn't supports, use 'vfio-pci'", physFunc.PFDriver)
			}
			return nil

		case utils.VfioPci:
			return validateVfioDriverOptions(physFunc)
		default:
			return fmt.Errorf("unknown driver '%s'", physFunc.PFDriver)
		}
	}
	return nil
}

func (r *FecNodeConfigReconciler) getPfBbConfVersion() string {
	cmdString := fmt.Sprintf("%s version 2>/dev/null | sed -n 's/.*Version \\(\\S*\\) .*/\\1/p' | tr -d '\\n'", pfConfigAppFilepath)
	cmd := exec.Command("bash", "-c", cmdString)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		r.log.WithError(err).Error("failed to execute command")
		return "null"
	} else {
		r.log.Info("pf_bb_config Version is:", out.String())
		return out.String()
	}
}

/*****************************************************************************
 * Method: FecNodeConfigReconciler::checkIfDeviceUpdateNeeded
 * Description: Determines if a device update is required based on the current
 *		and requested configurations
 ****************************************************************************/
func (r *FecNodeConfigReconciler) checkIfDeviceUpdateNeeded(previousConf, currentConf map[string]fec.PhysicalFunctionConfigExt) {
	// Check for updates in previous configuration
	for k, prevConfig := range previousConf {
		if currConfig, exists := currentConf[k]; !exists || !equality.Semantic.DeepEqual(prevConfig, currConfig) {
			fecDeviceUpdateRequired[k] = true
		} else {
			fecDeviceUpdateRequired[k] = false
		}
	}

	// Check for new entries in current configuration
	for k := range currentConf {
		if _, exists := previousConf[k]; !exists {
			fecDeviceUpdateRequired[k] = true
		}
	}
}
