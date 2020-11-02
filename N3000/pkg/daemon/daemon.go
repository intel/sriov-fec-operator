// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"context"
	"fmt"

	dh "github.com/otcshare/openshift-operator/N3000/pkg/drainhelper"

	"github.com/go-logr/logr"
	fpgav1 "github.com/otcshare/openshift-operator/N3000/api/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	crNameTemplate = "n3000node-%s"
)

type N3000NodeReconciler struct {
	client.Client
	log       logr.Logger
	nodeName  string
	namespace string

	fortville FortvilleManager
	fpga      FPGAManager

	drainHelper *dh.DrainHelper
}

func NewN3000NodeReconciler(c client.Client, clientSet *clientset.Clientset, log logr.Logger,
	nodename, namespace string) *N3000NodeReconciler {

	return &N3000NodeReconciler{
		Client:    c,
		log:       log,
		nodeName:  nodename,
		namespace: namespace,
		fortville: FortvilleManager{
			Log: log.WithName("fortvilleManager"),
		},
		fpga: FPGAManager{
			Log: log.WithName("fpgaManager"),
		},
		drainHelper: dh.NewDrainHelper(log, clientSet, nodename, namespace),
	}
}

func (r *N3000NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fpgav1.N3000Node{}).
		Complete(r)
}

// CreateEmptyN3000NodeIfNeeded creates empty CR to be Reconciled in near future and filled with Status.
// If invoked before manager's Start, it'll need a direct API client
// (Manager's/Controller's client is cached and cache is not initialized yet).
func (r *N3000NodeReconciler) CreateEmptyN3000NodeIfNeeded(c client.Client) error {
	name := fmt.Sprintf(crNameTemplate, r.nodeName)
	log := r.log.WithName("CreateEmptyN3000NodeIfNeeded").WithValues("name", name, "namespace", r.namespace)

	n3000node := &fpgav1.N3000Node{}
	err := c.Get(context.Background(),
		client.ObjectKey{
			Name:      name,
			Namespace: r.namespace,
		},
		n3000node)

	if err == nil {
		log.Info("already exists")
		return nil
	}

	if k8serrors.IsNotFound(err) {
		log.Info("not found - creating")

		n3000node = &fpgav1.N3000Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: r.namespace,
			},
			Status: fpgav1.N3000NodeStatus{
				SyncStatus:    fpgav1.InProgressSync,
				LastSyncError: "Initial, empty N3000Node. Waiting for inventory refresh",
			},
		}

		if createErr := c.Create(context.Background(), n3000node); createErr != nil {
			log.Error(createErr, "failed to create")
			return createErr
		}

		updateErr := c.Status().Update(context.Background(), n3000node)
		if updateErr != nil {
			log.Error(updateErr, "failed to update status")
		}
		return updateErr
	}

	return err
}

func (r *N3000NodeReconciler) flash(n *fpgav1.N3000Node) error {
	log := r.log.WithName("flash")
	err := r.fpga.processFPGA(n)
	if err != nil {
		log.Error(err, "Unable to processFPGA")
		return err
	}

	err = r.fortville.flash(n)
	if err != nil {
		log.Error(err, "Unable to flash fortville")
		return err
	}
	return nil
}

func (r *N3000NodeReconciler) createStatus(n *fpgav1.N3000Node) (*fpgav1.N3000NodeStatus, error) {
	log := r.log.WithName("createStatus")

	status, err := r.createBasicNodeStatus()
	if err != nil {
		log.Error(err, "failed to create basic node status")
		return nil, err
	}

	log.V(2).Info("Updating fortville status with nvmupdate inventory data")
	i, err := r.fortville.getInventory(n)
	if err != nil {
		log.Error(err, "Unable to get inventory...using basic status only")
		return nil, err
	} else {
		r.fortville.processInventory(&i, status) // fill ns with data from inventory
	}
	return status, nil
}

func (r *N3000NodeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.log.WithName("Reconcile").WithValues("namespace", req.Namespace, "name", req.Name)

	// Lack of Status.SyncStatus update on namespace/name mismatch (like in N3000ClusterReconciler):
	// N3000Node is between Operator & Daemon so we're in control of this communication channel.

	if req.Namespace != r.namespace {
		log.Info("unexpected namespace - ignoring", "expected namespace", r.namespace)
		return ctrl.Result{}, nil
	}

	if req.Name != fmt.Sprintf(crNameTemplate, r.nodeName) {
		log.Info("CR intended for another node - ignoring", "expected name", fmt.Sprintf(crNameTemplate, r.nodeName))
		return ctrl.Result{}, nil
	}

	ctx := context.Background()

	n3000node := &fpgav1.N3000Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, n3000node); err != nil {
		if k8serrors.IsNotFound(err) {
			log.Info("reconciled n3000node not found")
			return ctrl.Result{}, r.CreateEmptyN3000NodeIfNeeded(r.Client)
		}
		log.Error(err, "Get(n3000node) failed")
		return ctrl.Result{}, err
	}

	// here we should decide if we need to put node into maintenance mode
	// at least some basic checks like CR's Generation, to avoid reconciling twice the same CR
	drainNeeded := true

	if drainNeeded {
		var result ctrl.Result
		var actionErr error

		err := r.drainHelper.Run(func(c context.Context) {
			err := r.flash(n3000node)
			if err != nil {
				log.Error(err, "failed to flash")
				actionErr = err
			}
		})

		if err != nil {
			// some kind of error around leader election / node (un)cordon / node drain
			return result, err
		}

		if actionErr != nil {
			// flashing/programming logic failure
			return result, actionErr
		}
	}

	s, err := r.createStatus(n3000node)
	if err != nil {
		log.Error(err, "failed to get status")
		return ctrl.Result{}, err
	}
	n3000node.Status = *s
	if err := r.Status().Update(context.Background(), n3000node); err != nil {
		log.Error(err, "failed to update N3000Node status")
		return ctrl.Result{}, err
	}

	log.Info("Reconciled")

	return ctrl.Result{}, nil
}

func (r *N3000NodeReconciler) createBasicNodeStatus() (*fpgav1.N3000NodeStatus, error) {
	fortvilleStatus, err := r.fortville.getNetworkDevices()
	if err != nil {
		return nil, err
	}

	fpgaStatus, err := getFPGAInventory(r.log)
	if err != nil {
		return nil, err
	}

	return &fpgav1.N3000NodeStatus{
		Fortville: fortvilleStatus,
		FPGA:      fpgaStatus,
	}, nil
}
