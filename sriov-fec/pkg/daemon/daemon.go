// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"context"

	"github.com/go-logr/logr"
	sriovv1 "github.com/otcshare/openshift-operator/sriov-fec/api/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NodeConfigReconciler struct {
	client.Client
	log       logr.Logger
	nodeName  string
	namespace string
}

func NewNodeConfigReconciler(c client.Client, log logr.Logger,
	nodename, namespace string) *NodeConfigReconciler {

	return &NodeConfigReconciler{
		Client:    c,
		log:       log,
		nodeName:  nodename,
		namespace: namespace,
	}
}

func (r *NodeConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sriovv1.SriovFecNodeConfig{}).
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

	log.Info("Reconciled")

	return ctrl.Result{}, nil
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

	if k8serrors.IsNotFound(err) {
		log.Info("not found - creating")

		nodeConfig = &sriovv1.SriovFecNodeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.nodeName,
				Namespace: r.namespace,
			},
			Spec: sriovv1.SriovFecNodeConfigSpec{
				OneCardConfigForAll: true,
				Cards:               []sriovv1.CardConfig{},
			},
			Status: sriovv1.SriovFecNodeConfigStatus{
				SyncStatus:    "sriovv1.InProgressSync",
				LastSyncError: "Initial, empty NodeConfig. Waiting for inventory refresh",
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

	return err

}
