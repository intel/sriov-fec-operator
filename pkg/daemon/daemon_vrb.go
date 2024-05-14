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

	vrbv1 "github.com/intel/sriov-fec-operator/api/sriovvrb/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var (
	VrbConfigPath            = "/sriov_config/config/accelerators_vrb.json"
	VrbgetSriovInventory     = VrbGetSriovInventory
	VrbsupportedAccelerators utils.AcceleratorDiscoveryConfig
)

type VrbNodeConfigReconciler struct {
	client.Client
	log                 *logrus.Logger
	nodeNameRef         types.NamespacedName
	drainerAndExecute   DrainAndExecute
	vrbconfigurer       VrbConfigurer
	restartDevicePlugin RestartDevicePluginFunction
}

type VrbConfigurer interface {
	VrbApplySpec(nodeConfig vrbv1.SriovVrbNodeConfigSpec) error
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::Reconcile
 * Description:
 *
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) Reconcile(_ context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Infof("VrbReconcile(...) triggered by %s", req.NamespacedName.String())

	vrbnc, err := r.readNodeConfig(req.NamespacedName)

	if err != nil {
		return requeueNowWithError(err)
	}

	vrbdetectedInventory, err := r.readExistingInventory()
	if err != nil {
		return requeueNowWithError(err)
	}

	if err := validateVrbNodeConfig(vrbnc.Spec); err != nil {
		return requeueNowWithError(r.updateStatus(vrbnc, metav1.ConditionFalse, ConfigurationFailed, err.Error()))
	}

	if VrbisConfigurationOfNonExistingInventoryRequested(vrbnc.Spec.PhysicalFunctions, vrbdetectedInventory) {
		r.log.Info("requested configuration refers to not existing accelerator(s)")
		return requeueLaterOrNowIfError(r.updateStatus(vrbnc, metav1.ConditionFalse, ConfigurationFailed, "requested configuration refers to not existing accelerator"))
	}

	if !r.isCardUpdateRequired(vrbnc, vrbdetectedInventory) {
		r.log.Info("SriovVrb: Nothing to do")
		return requeueLater()
	}

	if r.isCardUpdateRequired(vrbnc, vrbdetectedInventory) {

		if err := r.updateStatus(vrbnc, metav1.ConditionFalse, ConfigurationInProgress, "Configuration started"); err != nil {
			return requeueNowWithError(err)
		}

		if err := r.configureNode(vrbnc); err != nil {
			r.log.WithError(err).Error("error occurred during configuring node")
			return requeueNowWithError(r.updateStatus(vrbnc, metav1.ConditionFalse, ConfigurationFailed, err.Error()))
		} else {
			return requeueLaterOrNowIfError(r.updateStatus(vrbnc, metav1.ConditionTrue, ConfigurationSucceeded, "Configured successfully"))
		}

	}

	return requeueLater()
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::CreateEmptyNodeConfigIfNeeded
 * Description:
 *
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) CreateEmptyNodeConfigIfNeeded(c client.Client) error {
	VrbnodeConfig := &vrbv1.SriovVrbNodeConfig{}

	err := c.Get(context.Background(), client.ObjectKey{Name: r.nodeNameRef.Name, Namespace: r.nodeNameRef.Namespace}, VrbnodeConfig)
	if err == nil {
		r.log.Info("already exists")
		return nil
	}

	if !k8serrors.IsNotFound(err) {
		return err
	}

	r.log.Infof("VrbnodeConfig{%s} not found - creating", r.nodeNameRef)

	VrbnodeConfig = &vrbv1.SriovVrbNodeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.nodeNameRef.Name,
			Namespace: r.nodeNameRef.Namespace,
		},
		Spec: vrbv1.SriovVrbNodeConfigSpec{
			PhysicalFunctions: []vrbv1.PhysicalFunctionConfigExt{},
		},
	}

	if createErr := c.Create(context.Background(), VrbnodeConfig); createErr != nil {
		r.log.WithError(createErr).Error("failed to create")
		return createErr
	}

	meta.SetStatusCondition(&VrbnodeConfig.Status.Conditions, metav1.Condition{
		Type:               ConditionConfigured,
		Status:             metav1.ConditionFalse,
		Reason:             string(ConfigurationNotRequested),
		Message:            "",
		ObservedGeneration: VrbnodeConfig.GetGeneration(),
	})

	if inv, err := r.readExistingInventory(); err != nil {
		return err
	} else {
		VrbnodeConfig.Status.Inventory = *inv
	}

	VrbnodeConfig.Status.PfBbConfVersion = r.getVrbPfBbConfVersion()

	if updateErr := c.Status().Update(context.Background(), VrbnodeConfig); updateErr != nil {
		r.log.WithError(updateErr).Error("failed to update cr status")
		return updateErr
	}

	return nil
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::SetupWithManager
 * Description:
 *
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&vrbv1.SriovVrbNodeConfig{}).
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
 * Method: VrbNodeConfigReconciler::UpdateStatus
 * Description:
 *
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) updateStatus(nc *vrbv1.SriovVrbNodeConfig,
	status metav1.ConditionStatus,
	reason ConfigurationConditionReason, msg string) error {

	previousCondition := VrbfindOrCreateConfigurationStatusCondition(nc)

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
	if inv, err := VrbgetSriovInventory(r.log); err != nil {
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
 * Method: VrbNodeConfigReconciler::readExistingInventory
 * Description:
 *
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) readExistingInventory() (*vrbv1.NodeInventory, error) {
	inv, err := VrbgetSriovInventory(r.log)
	if err != nil {
		r.log.WithError(err).Error("failed to obtain sriov inventory for the node")
	}
	return inv, err
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::readNodeConfig
 * Description:
 *
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) readNodeConfig(nn types.NamespacedName) (nc *vrbv1.SriovVrbNodeConfig, err error) {
	getVrbNodeConfig := func() (*vrbv1.SriovVrbNodeConfig, error) {
		vrbnc := new(vrbv1.SriovVrbNodeConfig)
		if err := r.Client.Get(context.TODO(), nn, vrbnc); err != nil {
			return nil, err
		}
		return vrbnc, nil
	}

	if nc, err = getVrbNodeConfig(); err != nil {
		if !k8serrors.IsNotFound(err) {
			r.log.WithError(err).Error("Get() failed")
			return nil, err
		}

		r.log.Info("SriovVrbNodeConfig not found - creating")
		if err := r.CreateEmptyNodeConfigIfNeeded(r.Client); err != nil {
			r.log.WithError(err).Error("Couldn't create SriovVrbNodeConfig")
			return nil, err
		}

		if nc, err = getVrbNodeConfig(); err != nil {
			return nil, err
		}
	}

	return nc, nil
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::configureNode
 * Description:
 *
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) configureNode(nodeConfig *vrbv1.SriovVrbNodeConfig) error {
	var configurationError error

	drainFunc := func(ctx context.Context) bool {
		if err := r.vrbconfigurer.VrbApplySpec(nodeConfig.Spec); err != nil {
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
 * Method: VrbNodeConfigReconciler::isCardUpdateRequierd
 * Description:
 *
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) isCardUpdateRequired(nc *vrbv1.SriovVrbNodeConfig,
	detectedInventory *vrbv1.NodeInventory) bool {
	pciToVfsAmount := map[string]int{}
	for _, physicalFunction := range nc.Spec.PhysicalFunctions {
		pciToVfsAmount[physicalFunction.PCIAddress] = physicalFunction.VFAmount
	}
	isGenerationChanged := func() bool {
		observedGeneration := VrbfindOrCreateConfigurationStatusCondition(nc).ObservedGeneration
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
		r.log.Info("Empty VRB PF")
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

/*****************************************************************************
 * Function: VrbNewNodeConfigReconciler
 * Description:
 *
 ****************************************************************************/
func VrbNewNodeConfigReconciler(k8sClient client.Client, drainer DrainAndExecute, nodeNameRef types.NamespacedName, vrbconfigurer VrbConfigurer, restartDevicePluginFunction RestartDevicePluginFunction) (r *VrbNodeConfigReconciler, err error) {

	if VrbsupportedAccelerators, err = utils.LoadDiscoveryConfig(VrbConfigPath); err != nil {
		return nil, err
	}

	return &VrbNodeConfigReconciler{
		Client:              k8sClient,
		drainerAndExecute:   drainer,
		log:                 utils.NewLogger(),
		nodeNameRef:         nodeNameRef,
		vrbconfigurer:       vrbconfigurer,
		restartDevicePlugin: restartDevicePluginFunction,
	}, nil
}

/*****************************************************************************
 * Function: VrbfindOrCreateConfigurationStatusCondition
 * Description:
 *
 ****************************************************************************/
func VrbfindOrCreateConfigurationStatusCondition(nc *vrbv1.SriovVrbNodeConfig) metav1.Condition {
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

/*****************************************************************************
 * Function: VrbisConfigurationOfNonExistingInventoryRequested
 * Description:
 *
 ****************************************************************************/
func VrbisConfigurationOfNonExistingInventoryRequested(
	requestedConfiguration []vrbv1.PhysicalFunctionConfigExt,
	existingInventory *vrbv1.NodeInventory) bool {
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

/*****************************************************************************
 * Function: validateVrbNodeConfig
 * Description:
 *
 ****************************************************************************/
func validateVrbNodeConfig(nodeConfig vrbv1.SriovVrbNodeConfigSpec) error {
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
			cmdlineBytes, err = os.ReadFile(sysLockdownFilePath)
			if err != nil {
				return fmt.Errorf("failed to read file contents: path: %v, error - %v", sysLockdownFilePath, err)
			}
			cmdline = string(cmdlineBytes)
			if !strings.Contains(cmdline, "[none]") {
				return fmt.Errorf("Kernel lockdown is enabled, '%s' driver doesn't supports, use 'vfio-pci'", physFunc.PFDriver)
			}

		case utils.VFIO_PCI:
			err := moduleParameterIsEnabled(utils.VFIO_PCI_UNDERSCORE, "enable_sriov")
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown driver '%s'", physFunc.PFDriver)
		}
	}
	return nil
}

func (r *VrbNodeConfigReconciler) getVrbPfBbConfVersion() string {
	pfConfigAppFilepath = "/sriov_workdir/pf_bb_config"
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
