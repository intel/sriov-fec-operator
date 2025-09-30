// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2023 Intel Corporation

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/intel/sriov-fec-operator/pkg/common/utils"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	vrbv1 "github.com/intel/sriov-fec-operator/api/sriovvrb/v1"
	"k8s.io/apimachinery/pkg/api/equality"
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
	configMapResource        sync.Map
	vrbPreviousConfig        = make(map[string]vrbv1.PhysicalFunctionConfigExt)
	vrbCurrentConfig         = make(map[string]vrbv1.PhysicalFunctionConfigExt)
	vrbDeviceUpdateRequired  = make(map[string]bool)
)

type VrbNodeConfigReconciler struct {
	client.Client
	log                 *logrus.Logger
	nodeNameRef         types.NamespacedName
	drainerAndExecute   DrainAndExecute
	vrbconfigurer       VrbConfigurer
	restartDevicePlugin RestartDevicePluginFunction
	cmRetrieveTime      time.Time
	cmRetrieveMutex     sync.Mutex
}

type VrbConfigurer interface {
	VrbApplySpec(nodeConfig vrbv1.SriovVrbNodeConfigSpec, vrbDeviceUpdateRequired map[string]bool) error
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::Reconcile
 * Description: This function is the main reconciliation loop for the VrbNodeConfig
 *              custom resource. It is triggered by changes to the resource and
 *              performs the following steps:
 *              1. Reads the VrbNodeConfig resource.
 *              2. Reads the existing inventory of the node.
 *              3. Validates the VrbNodeConfig specification.
 *              4. Checks if the requested configuration refers to non-existing
 *                 accelerators and updates the status accordingly.
 *              5. Determines if a card update is required based on the current
 *                 and requested configurations.
 *              6. If an update is required, it updates the status to indicate
 *                 that the configuration is in progress.
 *              7. Configures the node based on the VrbNodeConfig specification.
 *              8. Updates the status to indicate whether the configuration
 *                 succeeded or failed.
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) Reconcile(_ context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Debugf("VrbReconcile(...) triggered by %s", req.NamespacedName.String())

	r.setLogLevel()

	vrbnc, err := r.readNodeConfig(req.NamespacedName)

	if err != nil {
		return requeueNowWithError(err)
	}

	// Update PfBbConfVersion if it has changed
	if err := r.updatePfBbConfVersionIfChanged(vrbnc); err != nil {
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

	for _, accelerator := range vrbdetectedInventory.SriovAccelerators {
		vrbDeviceUpdateRequired[accelerator.PCIAddress] = true
	}

	if !r.isCardUpdateRequired(vrbnc, vrbdetectedInventory) {
		r.log.Debug("SriovVrb: Nothing to do")
		return requeueLater()
	}

	if err := r.updateStatus(vrbnc, metav1.ConditionFalse, ConfigurationInProgress, "Configuration started"); err != nil {
		return requeueNowWithError(err)
	}

	if err := r.configureNode(vrbnc); err != nil {
		r.log.WithError(err).Error("error occurred during configuring node")
		return requeueNowWithError(r.updateStatus(vrbnc, metav1.ConditionFalse, ConfigurationFailed, err.Error()))
	}

	return requeueLaterOrNowIfError(r.updateStatus(vrbnc, metav1.ConditionTrue, ConfigurationSucceeded, "Configured successfully"))
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::handleSriovDevicePluginConfigMap
 * Description: Handles the SR-IOV device plugin ConfigMap configuration,
 *		including loading, modifying and updating it if needed.
 * Returns an error if the device plugin configuration could not be handled.
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) handleSriovDevicePluginConfigMap(vrbnc *vrbv1.SriovVrbNodeConfig) error {
	// Process each physical function
	for _, acc := range vrbnc.Spec.PhysicalFunctions {
		if acc.VrbResourceName == "" {
			continue
		}
		if err := r.loadAndModifyDevicePluginConfig(vrbnc, acc); err != nil {
			return err
		}
	}

	return nil
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

	VrbnodeConfig.Status.PfBbConfVersion = r.getVrbPfBbConfVersion(true)

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
 * Description: Updates the status of the VrbNodeConfig resource.
 * Returns an error if the status could not be updated.
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) updateStatus(nc *vrbv1.SriovVrbNodeConfig,
	status metav1.ConditionStatus,
	reason ConfigurationConditionReason, msg string) error {

	previousCondition := VrbfindOrCreateConfigurationStatusCondition(nc)

	if reason == ConfigurationInProgress {
		// Clear the current configuration map
		for key := range vrbCurrentConfig {
			delete(vrbCurrentConfig, key)
		}
		// Update the current configuration map with the new configuration
		for _, pf := range nc.Spec.PhysicalFunctions {
			vrbCurrentConfig[pf.PCIAddress] = pf
		}
		r.checkIfDeviceUpdateNeeded(vrbPreviousConfig, vrbCurrentConfig)
	} else if reason == ConfigurationSucceeded {
		// Clear the previous configuration
		for key := range vrbPreviousConfig {
			delete(vrbPreviousConfig, key)
		}
		// Update the previous configuration with the current configuration
		for _, pf := range nc.Spec.PhysicalFunctions {
			vrbPreviousConfig[pf.PCIAddress] = pf
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
 * Description: Configures the node based on the VrbNodeConfig specification.
 * 		Applies the new PF/VF configuration and updates the sriov device plugin ConfigMap.
 * Returns an error if the configuration failed.
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) configureNode(nodeConfig *vrbv1.SriovVrbNodeConfig) error {
	var configurationError error

	drainFunc := func(ctx context.Context) bool {
		if err := r.vrbconfigurer.VrbApplySpec(nodeConfig.Spec, vrbDeviceUpdateRequired); err != nil {
			r.log.WithError(err).Error("failed applying new PF/VF configuration")
			configurationError = err
			return true
		}
		if err := r.handleSriovDevicePluginConfigMap(nodeConfig); err != nil {
			r.log.WithError(err).Error("failed updating the sriov device plugin ConfigMap")
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
 * Method: bbDevConfigDaemonIsDead
 * Description:
 *
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) bbDevConfigDaemonIsDead(nc *vrbv1.SriovVrbNodeConfig) bool {
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
		r.log.Debug("Empty VRB PF")
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

		case utils.VfioPci:
			err := moduleParameterIsEnabled(utils.VfioPciUnderscore, "enable_sriov")
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown driver '%s'", physFunc.PFDriver)
		}
	}
	return nil
}

func (r *VrbNodeConfigReconciler) getVrbPfBbConfVersion(shouldLog bool) string {
	cmdString := fmt.Sprintf("%s version 2>/dev/null | sed -n 's/.*Version \\(\\S*\\) .*/\\1/p' | tr -d '\\n'", pfConfigAppFilepath)
	cmd := exec.Command("bash", "-c", cmdString)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		r.log.WithError(err).Error("failed to execute command")
		return "null"
	} else if shouldLog {
		r.log.Info("pf_bb_config Version is:", out.String())
	}
	return out.String()
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::loadAndModifyDevicePluginConfig
 * Description: Loads the sriovdp-config ConfigMap and modifies the resourceName
 * 	    	and PF_PCI_ADDR for the requested pfPciAddress
 * Returns an error if the ConfigMap could not be loaded or modified
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) loadAndModifyDevicePluginConfig(vrbnc *vrbv1.SriovVrbNodeConfig, acc vrbv1.PhysicalFunctionConfigExt) error {
	// Load the current sriovdp-config ConfigMap
	currentConfig, err := r.loadCurrentDevicePluginConfig()
	if err != nil {
		return r.updateStatus(vrbnc, metav1.ConditionFalse, ConfigurationFailed, err.Error())
	}

	// Check if currentConfig["resourceList"] is a non-empty slice
	resourceList, ok := currentConfig["resourceList"].([]interface{})
	if !ok || len(resourceList) == 0 {
		r.log.Info("currentConfig does not contain a valid resourceList")
		return r.updateStatus(vrbnc, metav1.ConditionFalse, ConfigurationFailed, "currentConfig does not contain a valid resourceList")
	}

	// Get the VF device ID
	vfDeviceID, err := utils.GetVFDeviceID(acc.PCIAddress)
	if err != nil {
		r.log.WithError(err).Error("failed to get VF device ID")
	}
	r.log.Infof("VF device ID: %s for PCIAddress: %s", vfDeviceID, acc.PCIAddress)

	// Find all VFs configured for the PF
	vfAddresses, err := utils.FindVFs(acc.PCIAddress)
	r.log.WithField("pfPciAddress", acc.PCIAddress).Infof("Found VFs: %v", vfAddresses)
	if err != nil {
		r.log.WithError(err).Error("failed to find VFs")
	}

	// Check if the ConfigMap resource exists and needs to be updated
	if modified, err := r.resourceFoundAndUpdated(currentConfig, resourceList, vfDeviceID, acc, vfAddresses); err != nil {
		r.log.WithError(err).WithField("pfPciAddress", acc.PCIAddress).WithField("resourceName", acc.VrbResourceName).Error("failed to update ConfigMap")
		return r.updateStatus(vrbnc, metav1.ConditionFalse, ConfigurationFailed, err.Error())
	} else if modified {
		r.log.WithField("pfPciAddress", acc.PCIAddress).WithField("resourceName", acc.VrbResourceName).Info("ConfigMap resource updated successfully")
		return r.updateStatus(vrbnc, metav1.ConditionTrue, ConfigurationSucceeded, "ConfigMap resource updated successfully")
	}

	// Handle the case where a resource matching vfDeviceID and pfPciAddress was not found in the ConfigMap
	return r.handleResourceNotFound(currentConfig, resourceList, vfDeviceID, acc, vfAddresses)
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::resourceFoundAndUpdated
 * Description: Checks if the ConfigMap resource needs to be updated
 * Returns true if the resource was modified or
 *		if the number of pciAddresses in the resourceMap match the VFs found and the resourceName is the same as the newResourceName
 * 	   false otherwise
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) resourceFoundAndUpdated(currentConfig map[string]interface{}, resourceList []interface{}, vfDeviceID string, acc vrbv1.PhysicalFunctionConfigExt, vfAddresses []string) (bool, error) {
	for i := 0; i < len(resourceList); i++ {
		resourceMap, ok := resourceList[i].(map[string]interface{})
		if !ok {
			continue
		}
		if !r.matchVFDeviceID(resourceMap, vfDeviceID) {
			continue
		}
		if !r.matchPFPCIAddress(resourceMap, acc.PCIAddress) {
			continue
		}
		if r.matchVFsAndResourceName(resourceMap, acc.VrbResourceName, vfAddresses) {
			// VFs and resourceName match, no need to update the resource
			r.log.WithField("pfPciAddress", acc.PCIAddress).WithField("resourceName", acc.VrbResourceName).Info("ConfigMap resource already exists and is up-to-date")
			return true, nil
		}
		if r.modifyResource(resourceMap, acc.VrbResourceName, acc.PCIAddress, vfAddresses) {
			if err := r.updateConfigMap(currentConfig, acc.VrbResourceName); err != nil {
				r.log.WithError(err).WithField("pfPciAddress", acc.PCIAddress).WithField("resourceName", acc.VrbResourceName).Error("failed to update ConfigMap")
				return false, err
			} else {
				r.log.WithField("pfPciAddress", acc.PCIAddress).WithField("resourceName", acc.VrbResourceName).Info("ConfigMap resource modified successfully")
				return true, nil
			}
		}
	}
	return false, nil
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::matchVFsAndResourceName
 * Description: Checks if the amount of pciAddresses in the resourceMap match the VFs found and
 * 		if the resourceName is the same as the newResourceName
 * Returns true if number of pciAddresses in the resourceMap match the VFs found and
 * 		the resourceName is the same as the newResourceName, false otherwise
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) matchVFsAndResourceName(resourceMap map[string]interface{}, newResourceName string, vfAddresses []string) bool {
	resourceName, ok := resourceMap["resourceName"].(string)
	if !ok {
		return false
	}
	selectors, ok := resourceMap["selectors"].(map[string]interface{})
	if !ok {
		return false
	}
	pciAddresses, ok := selectors["pciAddresses"].([]interface{})
	if !ok {
		r.log.WithField("selectors", selectors).Info("pciAddresses not found in resourceMap")
		return false
	}
	r.log.WithField("pciAddresses", pciAddresses).WithField("vfAddresses", vfAddresses).Infof("current vfAddresses amount: %d requested vfAddresses: %d", len(pciAddresses), len(vfAddresses))
	r.log.WithField("resourceName", resourceName).WithField("newResourceName", newResourceName).Info("resourceName")
	return resourceName == newResourceName && len(pciAddresses) == len(vfAddresses)
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::matchPFPCIAddress
 * Description: Checks the additional information in the resource map to see if
 *              PF_PCI_ADDR matches the requested PF PCI address.
 * Returns: true if the PF_PCI_ADDR stored in additional information matches or if not set
 * 	    false otherwise.
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) matchPFPCIAddress(resourceMap map[string]interface{}, pfPciAddress string) bool {
	additionalInfoMap, ok := resourceMap["additionalInfo"].(map[string]interface{})
	if !ok {
		r.log.Infof("additionalInfo not found in resourceMap")
		return false
	}

	starInfo, ok := additionalInfoMap["*"].(map[string]interface{})
	if !ok {
		r.log.Infof("additionalInfo[*] not found in resourceMap")
		return false
	}

	pfPciAddr, ok := starInfo["PF_PCI_ADDR"].(string)
	if ok {
		// If the PF_PCI_ADDR is already set and it is different from the new PF_PCI_ADDR, return false
		if strings.TrimSpace(pfPciAddr) != pfPciAddress {
			r.log.Infof("PF_PCI_ADDR=%s in resourceMap does not match requested pfPciAddress=%s", pfPciAddr, pfPciAddress)
			return false
		}
	}
	return true
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::matchVFDeviceID
 * Description: Checks the selectors["devices"] in the resource map to see if
 * 		they match the VF device ID. If they do, it saves the original
 *		ConfigMap resource if it hasn't been saved already.
 * Returns true if selectors["devices"] equals vfDeviceID, false otherwise.
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) matchVFDeviceID(resourceMap map[string]interface{}, vfDeviceID string) bool {
	selectors, ok := resourceMap["selectors"].(map[string]interface{})
	if !ok {
		return false
	}
	devices, ok := selectors["devices"].([]interface{})
	if !ok || len(devices) == 0 {
		return false
	}
	if deviceID, ok := devices[0].(string); ok && deviceID == vfDeviceID {
		// Save original ConfigMap resource
		if _, loaded := configMapResource.LoadOrStore(vfDeviceID, copyResource(resourceMap)); !loaded {
			r.log.WithFields(logrus.Fields{
				"vfDeviceID":       vfDeviceID,
				"originalResource": resourceMap,
			}).Info("original resource saved")
		}
		return true
	}
	return false
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::handleResourceNotFound
 * Description: Handles the case where a resource matching vfDeviceID and
 * 		pfPciAddress was not found in the ConfigMap
 * Returns an error if the resource could not be modified
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) handleResourceNotFound(config map[string]interface{}, resourceList []interface{}, vfDeviceID string, acc vrbv1.PhysicalFunctionConfigExt, vfAddresses []string) error {
	// Check if the configMapResource map contains the original resource for the vfDeviceID
	if originalResource, exists := configMapResource.Load(vfDeviceID); exists {
		r.log.WithFields(logrus.Fields{
			"vfDeviceID":       vfDeviceID,
			"pfPciAddress":     acc.PCIAddress,
			"newResourceName":  acc.VrbResourceName,
			"originalResource": originalResource,
		}).Info("modifying original resource")

		// Create a new resource based on the original resource
		newResource := copyResource(originalResource)
		newResourceMap, ok := newResource.(map[string]interface{})
		if !ok {
			r.log.Infof("resourceMap not found in new resource")
			return nil
		}

		// Modify the new resource
		if r.modifyResource(newResourceMap, acc.VrbResourceName, acc.PCIAddress, vfAddresses) {
			// Append the new resource to the resourceList
			r.log.WithField("originalConfig", config).Info("original sriov-device-plugin config")
			resourceList = append(resourceList, newResource)
			config["resourceList"] = resourceList
			r.log.WithField("newConfig", config).Info("updated sriov-device-plugin config")
			return r.updateConfigMap(config, acc.VrbResourceName)
		} else {
			r.log.WithField("pfPciAddress", acc.PCIAddress).WithField("resourceName", acc.VrbResourceName).Info("ConfigMap resource not modified")
			return nil
		}
	} else {
		r.log.WithField("vfDeviceID", vfDeviceID).WithField("pfPciAddress", acc.PCIAddress).Info("resource not found in ConfigMap")
		return nil
	}
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::updateConfigMap
 * Description: Updates the sriovdp-config ConfigMap with the modified data
 * Returns an error if the ConfigMap could not be updated
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) updateConfigMap(newConfig map[string]interface{}, newResourceName string) error {
	modifiedData, err := json.MarshalIndent(newConfig, "", "  ")
	if err != nil {
		r.log.WithError(err).Error("failed to marshal modified config.json")
		return err
	}
	ctx := context.TODO()
	configMapName := "sriovdp-config"

	// Extract the node name to construct a key for node-specific configuration in the ConfigMap.
	// This key is used to check if a node-specific entry (config_$nodeName.json) exists and to update it if necessary
	nodeName := r.nodeNameRef.Name
	r.log.WithField("configMapName", configMapName).Debug("loading current sriovdp-config")

	// Load the ConfigMap
	configMap := &v1.ConfigMap{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: r.nodeNameRef.Namespace}, configMap); err != nil {
		r.log.WithError(err).WithField("ConfigMap name", configMapName).Error("failed to load ConfigMap")
		return err
	}

	// Check for config_$nodeName.json entry
	nodeConfigKey := fmt.Sprintf("config_%s.json", nodeName)
	_, ok := configMap.Data[nodeConfigKey]
	if ok {
		configMap.Data[nodeConfigKey] = string(modifiedData)
	} else {
		nodeConfigKey = "config.json"
		configMap.Data["config.json"] = string(modifiedData)
	}
	// Attempt to update the ConfigMap in the cluster
	if err := r.Client.Update(ctx, configMap); err != nil {
		r.log.WithError(err).WithField("ConfigMap name", configMapName).Error("failed to update ConfigMap")
		return err
	}

	r.log.WithFields(logrus.Fields{
		"ConfigMap name": configMapName,
		"nodeConfigKey":  nodeConfigKey,
		"resourceName":   newResourceName,
	}).Info("sriovdp-config modified successfully")

	return nil
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::modifyResource
 * Description: Modifies the resource with the new resourceName and pfPciAddress
 * Returns true if the resource was modified, false otherwise
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) modifyResource(resourceMap map[string]interface{}, newResourceName string, pfPciAddress string, vfAddresses []string) bool {
	selectors, ok := resourceMap["selectors"].(map[string]interface{})
	if !ok {
		return false
	}

	additionalInfoMap, ok := resourceMap["additionalInfo"].(map[string]interface{})
	if !ok {
		r.log.Infof("additionalInfo not found in resourceMap")
		return false
	}

	starInfo, ok := additionalInfoMap["*"].(map[string]interface{})
	if !ok {
		r.log.Infof("additionalInfo[*] not found in resourceMap")
		return false
	}

	r.log.WithFields(logrus.Fields{
		"selectors":    selectors,
		"resourceName": resourceMap["resourceName"],
	}).Info("original resource map selectors and resourceName")

	// Modify selectors["pciAddresses"], resourceName and additionalInfo["*"]["PF_PCI_ADDR"]
	selectors["pciAddresses"] = vfAddresses
	resourceMap["resourceName"] = newResourceName
	starInfo["PF_PCI_ADDR"] = pfPciAddress

	r.log.WithFields(logrus.Fields{
		"selectors":    selectors,
		"resourceName": resourceMap["resourceName"],
	}).Info("new resource map selectors and resourceName")

	return true
}

// copyResource creates a deep copy of the given resource
func copyResource(resource interface{}) interface{} {
	resourceBytes, err := json.Marshal(resource)
	if err != nil {
		log.Fatalf("failed to marshal resource: %v", err)
	}
	var newResource interface{}
	if err := json.Unmarshal(resourceBytes, &newResource); err != nil {
		log.Fatalf("failed to unmarshal resource: %v", err)
	}
	return newResource
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::loadCurrentDevicePluginConfig
 * Description: Loads the current sriovdp-config ConfigMap
 * Returns the config.json data as a map[string]interface{} or an error
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) loadCurrentDevicePluginConfig() (map[string]interface{}, error) {
	r.cmRetrieveMutex.Lock()
	defer r.cmRetrieveMutex.Unlock()

	// Enforce a delay of at least 1 second between executions
	timeSinceLastExecution := time.Since(r.cmRetrieveTime)
	if timeSinceLastExecution < time.Second {
		time.Sleep(time.Second - timeSinceLastExecution)
	}

	// Update the last execution time
	r.cmRetrieveTime = time.Now()

	configMapName := "sriovdp-config"
	nodeName := r.nodeNameRef.Name
	r.log.WithField("configMapName", configMapName).Info("loading current sriovdp-config")

	// Load the ConfigMap
	configMap := &v1.ConfigMap{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: r.nodeNameRef.Namespace}, configMap); err != nil {
		r.log.WithError(err).WithField("ConfigMap name", configMapName).Error("failed to load ConfigMap")
		return nil, err
	}

	// Check for config_$nodeName.json entry
	nodeConfigKey := fmt.Sprintf("config_%s.json", nodeName)
	data, ok := configMap.Data[nodeConfigKey]
	if !ok {
		// If config_$nodeName.json entry does not exist, create it from config.json
		// This is a fallback mechanism to ensure that the node-specific configuration is created when vrbCustomResourceName is set
		r.log.WithField("nodeConfigKey", nodeConfigKey).Infof("%s not found, creating %s entry from config.json", nodeConfigKey, nodeConfigKey)

		// Attempt to copy config.json if it exists
		defaultConfigData, defaultConfigExists := configMap.Data["config.json"]
		if !defaultConfigExists {
			err := fmt.Errorf("config.json not found in ConfigMap %s", configMapName)
			r.log.WithError(err).Error("failed to find config.json in ConfigMap")
			return nil, err
		}

		// Copy config.json to config_$nodeName.json
		configMap.Data[nodeConfigKey] = defaultConfigData
		if err := r.Client.Update(context.TODO(), configMap); err != nil {
			r.log.WithError(err).Errorf("failed to update ConfigMap with %s", nodeConfigKey)
			return nil, err
		}

		data = defaultConfigData
	}

	// Parse the config_$nodeName.json data
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		r.log.WithError(err).Errorf("failed to unmarshal %s", nodeConfigKey)
		return nil, err
	}

	return config, nil
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::checkIfDeviceUpdateNeeded
 * Description: Determines if a device update is required based on the current
 *		and requested configurations
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) checkIfDeviceUpdateNeeded(previousConf, currentConf map[string]vrbv1.PhysicalFunctionConfigExt) {
	// Check for updates in previous configuration
	for k, prevConfig := range previousConf {
		if currConfig, exists := currentConf[k]; !exists || !equality.Semantic.DeepEqual(prevConfig, currConfig) {
			vrbDeviceUpdateRequired[k] = true
		} else {
			vrbDeviceUpdateRequired[k] = false
		}
	}

	// Check for new entries in current configuration
	for k := range currentConf {
		if _, exists := previousConf[k]; !exists {
			vrbDeviceUpdateRequired[k] = true
		}
	}
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::updatePfBbConfVersionIfChanged
 * Description: Updates the PfBbConfVersion in the status of the VrbNodeConfig
 *		if it has changed.
 * Returns an error if the update failed.
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) updatePfBbConfVersionIfChanged(vrbnc *vrbv1.SriovVrbNodeConfig) error {
	currentVersion := r.getVrbPfBbConfVersion(false)
	if vrbnc.Status.PfBbConfVersion != currentVersion {
		r.log.Infof("PfBbConfVersion changed from %s to %s", vrbnc.Status.PfBbConfVersion, currentVersion)
		vrbnc.Status.PfBbConfVersion = currentVersion
		if err := r.Client.Status().Update(context.Background(), vrbnc); err != nil {
			r.log.WithError(err).Error("failed to update PfBbConfVersion in status")
			return err
		}
	}
	return nil
}

/*****************************************************************************
 * Method: VrbNodeConfigReconciler::setLogLevel
 * Description: Sets the log level based on the contents of the log level file.
 * If the file is empty or does not exist, it does nothing.
 * If the log level is changed, it logs a message indicating the change.
 ****************************************************************************/
func (r *VrbNodeConfigReconciler) setLogLevel() {
	logLevelFilePath := "/var/log/level.dat"
	logLevelContent, err := os.ReadFile(logLevelFilePath)
	if err != nil {
		return
	}

	// If the file is empty, do nothing
	if len(logLevelContent) == 0 {
		r.log.Debugf("Log level file %s is empty", logLevelFilePath)
		return
	}

	logLevelContent = bytes.TrimSpace(logLevelContent)
	logLevelContent = bytes.ToLower(logLevelContent)
	logLevelString := string(logLevelContent)

	// Get the current log level
	currentLogLevel := r.log.GetLevel()

	switch logLevelString {
	case "warn":
		if currentLogLevel != logrus.WarnLevel {
			r.log.SetLevel(logrus.WarnLevel)
			r.log.Warn("Log level changed to Warn")
		}
	case "error":
		if currentLogLevel != logrus.ErrorLevel {
			r.log.SetLevel(logrus.ErrorLevel)
			r.log.Error("Log level changed to Error")
		}
	case "info":
		if currentLogLevel != logrus.InfoLevel {
			r.log.SetLevel(logrus.InfoLevel)
			r.log.Info("Log level changed to Info")
		}
	case "debug":
		if currentLogLevel != logrus.DebugLevel {
			r.log.SetLevel(logrus.DebugLevel)
			r.log.Debug("Log level changed to Debug")
		}
	default:
		r.log.Debugf("Unknown log level: %s", logLevelString)
	}
}
