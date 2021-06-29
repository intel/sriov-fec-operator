// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package daemon

import (
	"context"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	dh "github.com/otcshare/openshift-operator/common/pkg/drainhelper"
	"github.com/otcshare/openshift-operator/common/pkg/utils"
	sriovv1 "github.com/otcshare/openshift-operator/sriov-fec/api/v1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	resyncPeriod = time.Minute
)

type ConfigurationConditionReason string

const (
	ConfigurationCondition    string                       = "Configured"
	ConfigurationUnknown      ConfigurationConditionReason = "Unknown"
	ConfigurationInProgress   ConfigurationConditionReason = "InProgress"
	ConfigurationFailed       ConfigurationConditionReason = "Failed"
	ConfigurationNotRequested ConfigurationConditionReason = "NotRequested"
	ConfigurationSucceeded    ConfigurationConditionReason = "Succeeded"
)

func (r *NodeConfigReconciler) updateCondition(nc *sriovv1.SriovFecNodeConfig, status metav1.ConditionStatus,
	reason ConfigurationConditionReason, msg string) {
	log := r.log.WithName("updateCondition")
	c := metav1.Condition{
		Type:               ConfigurationCondition,
		Status:             status,
		Reason:             string(reason),
		Message:            msg,
		ObservedGeneration: nc.GetGeneration(),
	}
	if err := r.updateStatus(nc, []metav1.Condition{c}); err != nil {
		log.Error(err, "failed to update SriovFecNodeConfig condition")
	}
}

func (r *NodeConfigReconciler) updateStatus(nc *sriovv1.SriovFecNodeConfig, c []metav1.Condition) error {
	log := r.log.WithName("updateStatus")

	inv, err := getSriovInventory(log)
	if err != nil {
		log.Error(err, "failed to obtain sriov inventory for the node")
		return err
	}
	nodeStatus := sriovv1.SriovFecNodeConfigStatus{Inventory: *inv}

	for _, condition := range c {
		meta.SetStatusCondition(&nodeStatus.Conditions, condition)
	}

	nc.Status = nodeStatus
	if err := r.Status().Update(context.Background(), nc); err != nil {
		log.Error(err, "failed to update SriovFecNode status")
		return err
	}

	return nil
}

type NodeConfigReconciler struct {
	client.Client
	log              logr.Logger
	nodeName         string
	namespace        string
	drainHelper      *dh.DrainHelper
	nodeConfigurator *NodeConfigurator
}

var (
	configPath            = "/sriov_config/config/accelerators.json"
	getSriovInventory     = GetSriovInventory
	supportedAccelerators utils.AcceleratorDiscoveryConfig
)

func NewNodeConfigReconciler(c client.Client, clientSet *clientset.Clientset, log logr.Logger,
	nodeName, ns string) (*NodeConfigReconciler, error) {

	var err error
	supportedAccelerators, err = utils.LoadDiscoveryConfig(configPath)
	if err != nil {
		return nil, err
	}

	kk, err := createKernelController(log.WithName("KernelController"))
	if err != nil {
		return nil, err
	}

	nc := &NodeConfigurator{Log: log.WithName("NodeConfigurator"), kernelController: kk}

	return &NodeConfigReconciler{
		Client:           c,
		log:              log,
		nodeName:         nodeName,
		namespace:        ns,
		drainHelper:      dh.NewDrainHelper(log, clientSet, nodeName, ns),
		nodeConfigurator: nc,
	}, nil
}

func (r *NodeConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sriovv1.SriovFecNodeConfig{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				_, ok := e.Object.(*sriovv1.SriovFecNodeConfig)
				if !ok {
					r.log.V(2).Info("Failed to convert e.Object to sriovv1.SriovFecNodeConfig", "e.Object", e.Object)
					return false
				}
				return true

			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				if e.ObjectOld.GetGeneration() == e.ObjectNew.GetGeneration() {
					r.log.V(4).Info("Update ignored, generation unchanged")
					return false
				}
				return true
			},
		}).
		Complete(r)
}

func (r *NodeConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.log.WithName("Reconcile").WithValues("namespace", req.Namespace, "name", req.Name)

	if req.Namespace != r.namespace {
		log.V(4).Info("unexpected namespace - ignoring", "expected namespace", r.namespace)
		return reconcile.Result{}, nil
	}

	if req.Name != r.nodeName {
		log.V(4).Info("CR intended for another node - ignoring", "expected name", r.nodeName)
		return reconcile.Result{}, nil
	}

	nodeConfig := &sriovv1.SriovFecNodeConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, nodeConfig); err != nil {
		if k8serrors.IsNotFound(err) {
			log.V(4).Info("not found - creating")
			return reconcile.Result{}, r.CreateEmptyNodeConfigIfNeeded(r.Client)
		}
		log.Error(err, "Get() failed")
		return reconcile.Result{}, err
	}
	skipStatusUpdate := false

	inv, err := getSriovInventory(log)
	if err != nil {
		log.Error(err, "failed to obtain sriov inventory for the node")
		r.updateCondition(nodeConfig, metav1.ConditionFalse, ConfigurationFailed, err.Error())
		return reconcile.Result{}, err
	}

	currentCondition := meta.FindStatusCondition(nodeConfig.Status.Conditions, ConfigurationCondition)
	if currentCondition != nil {
		if !reflect.DeepEqual(*inv, nodeConfig.Status.Inventory) {
			log.V(4).Info("updating inventory")
			r.updateCondition(nodeConfig, metav1.ConditionTrue, ConfigurationConditionReason(currentCondition.Reason), currentCondition.Message)
			return reconcile.Result{RequeueAfter: resyncPeriod}, nil
		}

		if currentCondition.ObservedGeneration == nodeConfig.GetGeneration() {
			return reconcile.Result{RequeueAfter: resyncPeriod}, nil
		}

		currentCondition.Status = metav1.ConditionFalse
		currentCondition.Reason = string(ConfigurationInProgress)
		currentCondition.Message = "Configuration started"
		if err := r.updateStatus(nodeConfig, []metav1.Condition{*currentCondition}); err != nil {
			log.Error(err, "failed to update current SriovFecNode configuration condition")
			return reconcile.Result{}, err
		}
	}

	if len(nodeConfig.Spec.PhysicalFunctions) == 0 {
		log.V(4).Info("Nothing to do")
		r.updateCondition(nodeConfig, metav1.ConditionFalse, ConfigurationNotRequested, "Inventory up to date")
		return reconcile.Result{RequeueAfter: resyncPeriod}, nil
	}

	var configurationErr, dhErr error

	dhErr = r.drainHelper.Run(func(c context.Context) bool {
		missingParams, err := r.nodeConfigurator.isAnyKernelParamsMissing()
		if err != nil {
			log.Error(err, "failed to check for missing params")
			configurationErr = err
			return true
		}

		if missingParams {
			log.V(2).Info("missing kernel params")

			err := r.nodeConfigurator.addMissingKernelParams()
			if err != nil {
				log.Error(err, "failed to add missing params")
				configurationErr = err
				return true
			}

			log.V(2).Info("added kernel params - rebooting")
			if err := r.nodeConfigurator.rebootNode(); err != nil {
				log.Error(err, "failed to request a node reboot")
				configurationErr = err
				return true
			}
			skipStatusUpdate = true
			return false // leave node cordoned & keep the leadership
		}
		if err := r.nodeConfigurator.applyConfig(nodeConfig.Spec); err != nil {
			log.Error(err, "failed applying new PF/VF configuration")
			configurationErr = err
			return true
		}

		configurationErr = r.restartDevicePlugin()
		return true
	}, !nodeConfig.Spec.DrainSkip)

	if skipStatusUpdate {
		log.V(4).Info("status update skipped - CR will be handled again after node reboot")
		return reconcile.Result{}, nil
	}

	if dhErr != nil {
		log.Error(dhErr, "drainhelper returned an error")
		r.updateCondition(nodeConfig, metav1.ConditionFalse, ConfigurationUnknown, dhErr.Error())
		return reconcile.Result{}, err
	}

	if configurationErr != nil {
		log.Error(configurationErr, "error during configuration")
		r.updateCondition(nodeConfig, metav1.ConditionFalse, ConfigurationFailed, configurationErr.Error())
		return reconcile.Result{}, err
	}

	if err := r.updateInventory(nodeConfig); err != nil {
		log.Error(err, "error during updateInventory")
		r.updateCondition(nodeConfig, metav1.ConditionFalse, ConfigurationFailed, err.Error())
		return reconcile.Result{}, err
	}

	r.updateCondition(nodeConfig, metav1.ConditionTrue, ConfigurationSucceeded, "Configured successfully")
	log.V(2).Info("Reconciled")

	return reconcile.Result{RequeueAfter: resyncPeriod}, nil
}

func (r *NodeConfigReconciler) restartDevicePlugin() error {
	pods := &corev1.PodList{}
	err := r.Client.List(context.TODO(), pods,
		client.InNamespace(r.namespace),
		&client.MatchingLabels{"app": "sriov-device-plugin-daemonset"})

	if err != nil {
		return errors.Wrap(err, "failed to get pods")
	}
	if len(pods.Items) == 0 {
		return errors.New("restartDevicePlugin: No pods found")
	}

	for _, p := range pods.Items {
		if p.Spec.NodeName != r.nodeName {
			continue
		}
		d := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: p.Namespace,
				Name:      p.Name,
			},
		}
		if err := r.Delete(context.TODO(), d, &client.DeleteOptions{}); err != nil {
			return errors.Wrap(err, "failed to delete sriov-device-plugin-daemonset pod")
		}

	}

	return nil
}

func (r *NodeConfigReconciler) updateInventory(nc *sriovv1.SriovFecNodeConfig) error {
	log := r.log.WithName("updateInventory")

	inv, err := getSriovInventory(log)
	if err != nil {
		log.Error(err, "failed to obtain sriov inventory")
		return err
	}

	log.V(4).Info("obtained inventory", "inv", inv)
	if inv != nil {
		nc.Status.Inventory = *inv
	}

	return nil
}

// CreateEmptyNodeConfigIfNeeded creates empty CR to be Reconciled in near future and filled with Status.
// If invoked before manager's Start, it'll need a direct API client
// (Manager's/Controller's client is cached and cache is not initialized yet).
func (r *NodeConfigReconciler) CreateEmptyNodeConfigIfNeeded(c client.Client) error {
	log := r.log.WithName("CreateEmptyNodeConfigIfNeeded").WithValues("name", r.nodeName, "namespace", r.namespace)

	nodeConfig := &sriovv1.SriovFecNodeConfig{}
	err := c.Get(context.Background(),
		client.ObjectKey{
			Name:      r.nodeName,
			Namespace: r.namespace,
		},
		nodeConfig)

	if err == nil {
		log.V(4).Info("already exists")
		return nil
	}

	if !k8serrors.IsNotFound(err) {
		return err
	}

	log.V(2).Info("not found - creating")

	nodeConfig = &sriovv1.SriovFecNodeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.nodeName,
			Namespace: r.namespace,
		},
		Spec: sriovv1.SriovFecNodeConfigSpec{
			PhysicalFunctions: []sriovv1.PhysicalFunctionConfig{},
		},
	}

	if createErr := c.Create(context.Background(), nodeConfig); createErr != nil {
		log.Error(createErr, "failed to create")
		return createErr
	}

	updateErr := c.Status().Update(context.Background(), nodeConfig)
	if updateErr != nil {
		log.Error(updateErr, "failed to update status")
	}
	return updateErr

}
