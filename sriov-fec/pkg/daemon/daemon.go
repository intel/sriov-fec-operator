// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"context"

	"github.com/go-logr/logr"
	dh "github.com/otcshare/openshift-operator/N3000/pkg/drainhelper"
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
)

const (
	conditionConfigured = "Configured"
)

type NodeConfigReconciler struct {
	client.Client
	log              logr.Logger
	nodeName         string
	namespace        string
	drainHelper      *dh.DrainHelper
	nodeConfigurator *NodeConfigurator
}

var (
	getSriovInventory = GetSriovInventory
)

func NewNodeConfigReconciler(c client.Client, clientSet *clientset.Clientset, log logr.Logger,
	nodename, namespace string) *NodeConfigReconciler {

	return &NodeConfigReconciler{
		Client:           c,
		log:              log,
		nodeName:         nodename,
		namespace:        namespace,
		drainHelper:      dh.NewDrainHelper(log, clientSet, nodename, namespace),
		nodeConfigurator: &NodeConfigurator{Log: log.WithName("NodeConfigurator")},
	}
}

func (r *NodeConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sriovv1.SriovFecNodeConfig{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				nodeConfig, ok := e.Object.(*sriovv1.SriovFecNodeConfig)
				if !ok {
					r.log.Info("Failed to convert e.Object to sriovv1.SriovFecNodeConfig", "e.Object", e.Object)
					return false
				}
				cond := meta.FindStatusCondition(nodeConfig.Status.Conditions, conditionConfigured)
				if cond != nil && cond.ObservedGeneration == e.Meta.GetGeneration() {
					r.log.Info("Created object was handled previously, ignoring")
					return false
				}
				return true

			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				if e.MetaOld.GetGeneration() == e.MetaNew.GetGeneration() {
					r.log.Info("Update ignored, generation unchanged")
					return false
				}
				return true
			},
		}).
		Complete(r)
}

func (r *NodeConfigReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.log.WithName("Reconcile").WithValues("namespace", req.Namespace, "name", req.Name)

	if req.Namespace != r.namespace {
		log.Info("unexpected namespace - ignoring", "expected namespace", r.namespace)
		return ctrl.Result{}, nil
	}

	if req.Name != r.nodeName {
		log.Info("CR intended for another node - ignoring", "expected name", r.nodeName)
		return ctrl.Result{}, nil
	}

	ctx := context.Background()

	nodeConfig := &sriovv1.SriovFecNodeConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, nodeConfig); err != nil {
		if k8serrors.IsNotFound(err) {
			log.Info("not found - creating")
			return ctrl.Result{}, r.CreateEmptyNodeConfigIfNeeded(r.Client)
		}
		log.Error(err, "Get() failed")
		return ctrl.Result{}, err
	}
	log.Info("obtained nodeConfig", "generation", nodeConfig.GetGeneration())

	configuredCondition := metav1.Condition{
		Type:               conditionConfigured,
		Status:             metav1.ConditionTrue,
		Message:            "Configured successfully",
		ObservedGeneration: nodeConfig.GetGeneration(),
		Reason:             "ConfigurationSucceeded",
	}

	skipStatusUpdate := false
	if len(nodeConfig.Spec.PhysicalFunctions) > 0 {
		var configurationErr error

		dhErr := r.drainHelper.Run(func(c context.Context) bool {
			missingParams, err := r.nodeConfigurator.isAnyKernelParamsMissing()
			if err != nil {
				log.Error(err, "failed to check for missing params")
				configurationErr = err
				return true
			}

			if missingParams {
				log.Info("missing kernel params")

				_, err := r.nodeConfigurator.addMissingKernelParams()
				if err != nil {
					log.Error(err, "failed to add missing params")
					configurationErr = err
					return true
				}

				log.Info("added kernel params - rebooting")
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

		if dhErr != nil {
			log.Error(dhErr, "drainhelper returned an error")
			configuredCondition.Status = metav1.ConditionFalse
			configuredCondition.Reason = "Unknown"
			configuredCondition.Message = dhErr.Error()
		}

		if configurationErr != nil {
			log.Error(configurationErr, "error during configuration")
			configuredCondition.Status = metav1.ConditionFalse
			configuredCondition.Reason = "ConfigurationFailed"
			configuredCondition.Message = configurationErr.Error()
		}
	}

	if skipStatusUpdate {
		log.Info("status update skipped - CR will be handled again after node reboot")
		return ctrl.Result{}, nil
	}

	meta.SetStatusCondition(&nodeConfig.Status.Conditions, configuredCondition)

	if err := r.updateInventory(nodeConfig); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Status().Update(context.Background(), nodeConfig); err != nil {
		log.Error(err, "failed to update NodeConfig status")
		return ctrl.Result{}, err
	}

	log.Info("Reconciled")

	return ctrl.Result{}, nil
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

	log.Info("obtained inventory", "inv", inv)
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
		log.Info("already exists")
		return nil
	}

	if !k8serrors.IsNotFound(err) {
		return err
	}

	log.Info("not found - creating")

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
