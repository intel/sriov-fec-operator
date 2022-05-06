// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package daemon

import (
	"context"
	"fmt"
	"github.com/otcshare/sriov-fec-operator/sriov-fec/pkg/common/utils"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"time"

	fec "github.com/otcshare/sriov-fec-operator/sriov-fec/api/v2"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
)

type NodeConfigReconciler struct {
	client.Client
	log         *logrus.Logger
	nodeNameRef types.NamespacedName
	configurer  NodeConfigurer
}

type Drainer func(configurer func(ctx context.Context) bool, drain bool) error

func NewNodeConfigReconciler(k8sClient client.Client, configurer NodeConfigurer, nodeNameRef types.NamespacedName) (r *NodeConfigReconciler, err error) {

	if supportedAccelerators, err = utils.LoadDiscoveryConfig(configPath); err != nil {
		return nil, err
	}

	return &NodeConfigReconciler{Client: k8sClient, log: utils.NewLogger(), nodeNameRef: nodeNameRef, configurer: configurer}, nil
}

func (r *NodeConfigReconciler) Reconcile(_ context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Infof("Reconcile(...) triggered by %s", req.NamespacedName.String())

	nc, err := r.readSriovFecNodeConfig(req.NamespacedName)
	if err != nil {
		return requeueNowWithError(err)
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

	if err := r.configurer.configureNode(nc); err != nil {
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
				ResourceNamePredicate{
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

type NodeConfigurer interface {
	configureNode(nodeConfig *fec.SriovFecNodeConfig) error
}

func NewNodeConfigurer(drainer Drainer, client client.Client, nodeNameRef types.NamespacedName) (NodeConfigurer, error) {
	log := utils.NewLogger()
	configurer := &NodeConfigurator{Log: log}
	return &nodeConfigurer{log: log, drainAndExecute: drainer, configurer: configurer, Client: client, nodeNameRef: nodeNameRef}, nil
}

type nodeConfigurer struct {
	client.Client
	drainAndExecute Drainer
	log             *logrus.Logger
	configurer      *NodeConfigurator
	nodeNameRef     types.NamespacedName
}

func (n *nodeConfigurer) configureNode(nodeConfig *fec.SriovFecNodeConfig) error {
	var configurationError error

	drainFunc := func(ctx context.Context) bool {
		missingParams, err := n.configurer.isAnyKernelParamsMissing()
		if err != nil {
			n.log.WithError(err).Error("failed to check for missing params")
			configurationError = err
			return true
		}

		if missingParams {
			configurationError = errors.New("missing kernel params")
			n.log.Error(configurationError)
			return true
		}

		if err := n.configurer.applyConfig(nodeConfig.Spec); err != nil {
			n.log.WithError(err).Error("failed applying new PF/VF configuration")
			configurationError = err
			return true
		}

		configurationError = n.restartDevicePlugin()
		return true
	}

	if err := n.drainAndExecute(drainFunc, !nodeConfig.Spec.DrainSkip); err != nil {
		return err
	}

	return configurationError
}

func (n *nodeConfigurer) restartDevicePlugin() error {
	pods := &corev1.PodList{}
	err := n.List(context.TODO(), pods,
		client.InNamespace(n.nodeNameRef.Namespace),
		&client.MatchingLabels{"app": "sriov-device-plugin-daemonset"})

	if err != nil {
		return errors.Wrap(err, "failed to get pods")
	}
	if len(pods.Items) == 0 {
		n.log.Info("there is no running instance of device plugin, nothing to restart")
	}

	for _, p := range pods.Items {
		if p.Spec.NodeName != n.nodeNameRef.Name {
			continue
		}
		d := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: p.Namespace,
				Name:      p.Name,
			},
		}
		if err := n.Delete(context.TODO(), d, &client.DeleteOptions{}); err != nil {
			return errors.Wrap(err, "failed to delete sriov-device-plugin-daemonset pod")
		}

		backoff := wait.Backoff{Steps: 300, Duration: 1 * time.Second, Factor: 1}
		err = wait.ExponentialBackoff(backoff, n.waitForDevicePluginRestart(p.Name))
		if err == wait.ErrWaitTimeout {
			return fmt.Errorf("failed to restart sriov-device-plugin within specified time")
		}
		return err
	}
	return nil
}

func (n *nodeConfigurer) waitForDevicePluginRestart(oldPodName string) func() (bool, error) {
	return func() (bool, error) {
		pods := &corev1.PodList{}

		err := n.List(context.TODO(), pods,
			client.InNamespace(n.nodeNameRef.Namespace),
			&client.MatchingLabels{"app": "sriov-device-plugin-daemonset"})
		if err != nil {
			n.log.WithError(err).Error("failed to list pods for sriov-device-plugin")
			return false, err
		}

		for _, pod := range pods.Items {
			if pod.Spec.NodeName == n.nodeNameRef.Name && pod.Name != oldPodName && isReady(pod) {
				n.log.Info("device-plugin is running")
				return true, nil
			}

		}
		return false, nil
	}
}

func isReady(p corev1.Pod) bool {
	for _, condition := range p.Status.Conditions {
		if condition.Type == corev1.PodReady && p.Status.Phase == corev1.PodRunning {
			return true
		}
	}
	return false
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

	return isGenerationChanged() || exposedInventoryOutdated()
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

//returns error if requested configuration refers to not existing inventory/accelerator
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

//returns result indicating necessity of re-queuing Reconcile after configured resyncPeriod
func requeueLater() (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: resyncPeriod}, nil
}

//returns result indicating necessity of re-queuing Reconcile(...) immediately; non-nil err will be logged by controller
func requeueNowWithError(e error) (reconcile.Result, error) {
	return reconcile.Result{Requeue: true}, e
}

//returns result indicating necessity of re-queuing Reconcile(...):
//immediately - in case when given err is non-nil;
//on configured schedule, when err is nil
func requeueLaterOrNowIfError(e error) (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: resyncPeriod}, e
}

type ResourceNamePredicate struct {
	predicate.Funcs
	requiredName string
	log          *logrus.Logger
}

func (r ResourceNamePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectNew.GetName() != r.requiredName {
		r.log.WithField("expected name", r.requiredName).Info("CR intended for another node - ignoring")
		return false
	}
	return true
}

func (r ResourceNamePredicate) Create(e event.CreateEvent) bool {
	if e.Object.GetName() != r.requiredName {
		r.log.WithField("expected name", r.requiredName).Info("CR intended for another node - ignoring")
		return false
	}
	return true
}

func CreateManager(config *rest.Config, namespace string, scheme *runtime.Scheme) (manager.Manager, error) {
	return ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
		LeaderElection:     false,
		Namespace:          namespace,
	})
}
