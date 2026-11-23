// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2023 Intel Corporation

package daemon

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/intel/sriov-fec-operator/pkg/common/utils"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"

	fec "github.com/intel/sriov-fec-operator/api/sriovfec/v2"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var (
	FecConfigPath         = "/sriov_config/config/accelerators.json"
	getSriovInventory     = GetSriovInventory
	supportedAccelerators utils.AcceleratorDiscoveryConfig
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
	// ApplySpec reconfigures the accelerators for which fecDeviceUpdateRequired is true.
	ApplySpec(nodeConfig fec.SriovFecNodeConfigSpec, fecDeviceUpdateRequired map[string]bool) error
	// RestartPfBBConfig starts pf_bb_config again for a PF that is otherwise configured.
	RestartPfBBConfig(acc fec.SriovAccelerator, pf *fec.PhysicalFunctionConfigExt) error
}

/*****************************************************************************
 * Method: FecNodeConfigReconciler::Reconcile
 * Description:
 *
 ****************************************************************************/
func (r *FecNodeConfigReconciler) Reconcile(_ context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Debugf("Reconcile(...) triggered by %s", req.NamespacedName.String())

	r.setLogLevel()

	sfnc, err := r.readNodeConfig(req.NamespacedName)
	if err != nil {
		return requeueNowWithError(err)
	}

	// Update PfBbConfVersion if it has changed
	if err := r.updatePfBbConfVersionIfChanged(sfnc); err != nil {
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

	plan := r.planActions(sfnc, detectedInventory)
	if !plan.anyAction() && !r.isGenerationChanged(sfnc) {
		r.log.Debug("SriovFec: Nothing to do")
		if plan.adopted {
			if err := storeAppliedConfigs(context.Background(), r.Client, sfnc, fecAppliedConfigAnnotation, plan.applied); err != nil {
				return requeueNowWithError(err)
			}
		}
		return requeueLater()
	}

	if err := r.updateStatus(sfnc, metav1.ConditionFalse, ConfigurationInProgress, "Configuration started"); err != nil {
		return requeueNowWithError(err)
	}

	if err := r.applyPlan(sfnc, plan); err != nil {
		r.log.WithError(err).Error("error occurred during configuring node")
		reason, retryLater := configurationFailureReason(err)
		if retryLater {
			return requeueLaterOrNowIfError(r.updateStatus(sfnc, metav1.ConditionFalse, reason, err.Error()))
		}
		return requeueNowWithError(r.updateStatus(sfnc, metav1.ConditionFalse, reason, err.Error()))
	}

	return requeueLaterOrNowIfError(r.updateStatus(sfnc, metav1.ConditionTrue, ConfigurationSucceeded, "Configured successfully"))
}

// fecPlan is what has to be done with each accelerator of the node.
type fecPlan struct {
	actions   map[string]pfAction
	applied   appliedConfigs
	adopted   bool // applied was taken from the status, not from a record
	specByPCI map[string]fec.PhysicalFunctionConfigExt
	accByPCI  map[string]fec.SriovAccelerator
}

func (p fecPlan) anyAction() bool {
	for _, a := range p.actions {
		if a != pfNoAction {
			return true
		}
	}
	return false
}

func (r *FecNodeConfigReconciler) isGenerationChanged(nc *fec.SriovFecNodeConfig) bool {
	observedGeneration := findOrCreateConfigurationStatusCondition(nc).ObservedGeneration
	if nc.GetGeneration() != observedGeneration {
		r.log.WithField("observed", observedGeneration).
			WithField("requested", nc.GetGeneration()).
			Info("Observed generation doesn't reflect requested one")
		return true
	}
	return false
}

// planActions decides, for each accelerator, whether it is left alone, gets
// pf_bb_config started again, or is reconfigured. See planPFAction.
func (r *FecNodeConfigReconciler) planActions(nc *fec.SriovFecNodeConfig, inventory *fec.NodeInventory) fecPlan {
	applied, recordPresent := readAppliedConfigs(nc, fecAppliedConfigAnnotation)
	plan := fecPlan{
		actions:   map[string]pfAction{},
		applied:   applied,
		specByPCI: map[string]fec.PhysicalFunctionConfigExt{},
		accByPCI:  map[string]fec.SriovAccelerator{},
	}

	adopt := adoptCurrentConfiguration(recordPresent, findOrCreateConfigurationStatusCondition(nc), nc.GetGeneration())
	for _, pf := range nc.Spec.PhysicalFunctions {
		plan.specByPCI[pf.PCIAddress] = pf
		if adopt {
			applied[pf.PCIAddress] = configHash(pf)
		}
	}
	plan.adopted = adopt
	if adopt && len(nc.Spec.PhysicalFunctions) > 0 {
		r.log.WithField("generation", nc.GetGeneration()).
			Info("no record of the applied configuration; taking the configuration reported as applied by the status")
	}

	for _, acc := range inventory.SriovAccelerators {
		acc := acc
		plan.accByPCI[acc.PCIAddress] = acc

		state := pfDeviceState{pfDriver: acc.PFDriver}
		for _, vf := range acc.VFs {
			state.vfDrivers = append(state.vfDrivers, vf.Driver)
		}
		var req *pfRequest
		if pf, ok := plan.specByPCI[acc.PCIAddress]; ok {
			req = &pfRequest{pfDriver: pf.PFDriver, vfDriver: pf.VFDriver, vfAmount: pf.VFAmount, hash: configHash(pf)}
		}

		action, why := planPFAction(req, applied[acc.PCIAddress], state, staleSysfsWrites.has(acc.PCIAddress),
			func() procState { return pfBbConfigProcState(r.log, acc.PCIAddress) })
		plan.actions[acc.PCIAddress] = action
		if action != pfNoAction {
			r.log.WithField("pciAddress", acc.PCIAddress).WithField("action", action.String()).Info(why)
		}
	}
	return plan
}

// applyPlan carries out the plan and records, per PF, the configuration
// that was applied.
func (r *FecNodeConfigReconciler) applyPlan(nc *fec.SriovFecNodeConfig, plan fecPlan) error {
	var firstErr error
	keepErr := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	var reconfigure []string
	for pci, action := range plan.actions {
		if action == pfReconfigure {
			reconfigure = append(reconfigure, pci)
		}
	}
	sort.Strings(reconfigure)

	if len(reconfigure) > 0 {
		results, err := r.configureNode(nc, reconfigure)
		for pci, perr := range results {
			pf, inSpec := plan.specByPCI[pci]
			switch {
			case perr == nil && inSpec:
				plan.applied[pci] = configHash(pf)
			case perr == nil || !refusedBeforeTeardown(perr):
				// not in the spec any more, or the teardown may have started
				delete(plan.applied, pci)
			}
		}
		keepErr(err)
	}

	for pci, action := range plan.actions {
		if action != pfRestartPfBBConfig {
			continue
		}
		pf := plan.specByPCI[pci]
		keepErr(r.sriovfecconfigurer.RestartPfBBConfig(plan.accByPCI[pci], &pf))
	}

	// drop records of PFs that are gone from the node or from the spec
	for pci := range plan.applied {
		_, inSpec := plan.specByPCI[pci]
		_, onNode := plan.accByPCI[pci]
		if !inSpec || !onNode {
			delete(plan.applied, pci)
		}
	}
	if err := storeAppliedConfigs(context.Background(), r.Client, nc, fecAppliedConfigAnnotation, plan.applied); err != nil {
		r.log.WithError(err).Error("failed to record the applied configuration")
		keepErr(err)
	}
	return firstErr
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

	SriovFecnodeConfig.Status.PfBbConfVersion = r.getPfBbConfVersion(true)

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
func (r *FecNodeConfigReconciler) configureNode(nodeConfig *fec.SriovFecNodeConfig, pciAddresses []string) (map[string]error, error) {
	results := map[string]error{}
	var configurationError error

	// one PF at a time, so that the outcome is known for each of them
	drainFunc := func(ctx context.Context) bool {
		applied := false
		for _, pci := range pciAddresses {
			err := r.sriovfecconfigurer.ApplySpec(nodeConfig.Spec, map[string]bool{pci: true})
			results[pci] = err
			if err != nil {
				r.log.WithError(err).WithField("pciAddress", pci).Error("failed applying new PF/VF configuration")
				if configurationError == nil {
					configurationError = err
				}
				continue
			}
			applied = true
		}

		if applied {
			if err := r.restartDevicePlugin(); err != nil && configurationError == nil {
				configurationError = err
			}
		}
		return true
	}

	if err := r.drainerAndExecute(drainFunc, !nodeConfig.Spec.DrainSkip); err != nil {
		return results, err
	}

	return results, configurationError
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

	loadStaleSysfsWriteMarkers(log)

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

		case utils.VfioPci:
			// check every PF, not only the first one: disable_idle_d3 is
			// only required for some accelerators
			if err := validateVfioDriverOptions(physFunc); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown driver '%s'", physFunc.PFDriver)
		}
	}
	return nil
}

func (r *FecNodeConfigReconciler) getPfBbConfVersion(shouldLog bool) string {
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
 * Method: FecNodeConfigReconciler::updatePfBbConfVersionIfChanged
 * Description: Updates the PfBbConfVersion in the status of the SriovFecNodeConfig
 *		if it has changed.
 * Returns an error if the update failed.
 ****************************************************************************/
func (r *FecNodeConfigReconciler) updatePfBbConfVersionIfChanged(nodeConfig *fec.SriovFecNodeConfig) error {
	currentVersion := r.getPfBbConfVersion(false)
	if nodeConfig.Status.PfBbConfVersion != currentVersion {
		r.log.Infof("PfBbConfVersion changed from %s to %s", nodeConfig.Status.PfBbConfVersion, currentVersion)
		nodeConfig.Status.PfBbConfVersion = currentVersion
		if err := r.Client.Status().Update(context.Background(), nodeConfig); err != nil {
			r.log.WithError(err).Error("failed to update PfBbConfVersion in status")
			return err
		}
	}
	return nil
}

/*****************************************************************************
 * Method: FecNodeConfigReconciler::setLogLevel
 * Description: Sets the log level based on the contents of the log level file.
 * If the file is empty or does not exist, it does nothing.
 * If the log level is changed, it logs a message indicating the change.
 ****************************************************************************/
func (r *FecNodeConfigReconciler) setLogLevel() {
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
