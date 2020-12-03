// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"context"
	"errors"
	"fmt"

	dh "github.com/otcshare/openshift-operator/N3000/pkg/drainhelper"

	"github.com/go-logr/logr"
	fpgav1 "github.com/otcshare/openshift-operator/N3000/api/v1"
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
	crNameTemplate = "n3000node-%s"
)

type FlashConditionReason string

const (
	// FlashCondition flash condition name
	FlashCondition string = "Flashed"

	// Failed indicates that the flashing is in an unknown state
	FlashUnknown FlashConditionReason = "Unknown"
	// FlashInProgress indicates that the flashing process is in progress
	FlashInProgress FlashConditionReason = "InProgress"
	// FlashFailed indicates that the flashing process failed
	FlashFailed FlashConditionReason = "Failed"
	// FlashNotRequested indicates that the flashing process was not requested
	FlashNotRequested FlashConditionReason = "NotRequested"
	// FlashSucceeded indicates that the flashing process succeeded
	FlashSucceeded FlashConditionReason = "Succeeded"
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
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				n3000node, ok := e.Object.(*fpgav1.N3000Node)
				if !ok {
					r.log.Info("Failed to convert e.Object to fpgav1.N3000Node", "e.Object", e.Object)
					return false
				}
				cond := meta.FindStatusCondition(n3000node.Status.Conditions, FlashCondition)
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
		}

		return c.Create(context.Background(), n3000node)
	}

	return err
}

func (r *N3000NodeReconciler) getNodeStatus(n *fpgav1.N3000Node) (fpgav1.N3000NodeStatus, error) {
	log := r.log.WithName("getNodeStatus")

	fortvilleStatus, err := r.fortville.getInventory()
	if err != nil {
		log.Error(err, "Failed to get Fortville inventory")
		return fpgav1.N3000NodeStatus{}, err
	}

	fpgaStatus, err := getFPGAInventory(r.log)
	if err != nil {
		log.Error(err, "Failed to get FPGA inventory")
		return fpgav1.N3000NodeStatus{}, err
	}

	return fpgav1.N3000NodeStatus{
		Fortville: fortvilleStatus,
		FPGA:      fpgaStatus,
	}, nil
}

func (r *N3000NodeReconciler) updateStatus(n *fpgav1.N3000Node, c []metav1.Condition) error {
	log := r.log.WithName("updateStatus")

	nodeStatus, err := r.getNodeStatus(n)
	if err != nil {
		log.Error(err, "failed to get N3000Node status")
		return err
	}

	for _, condition := range c {
		meta.SetStatusCondition(&nodeStatus.Conditions, condition)
	}
	n.Status = nodeStatus
	if err := r.Status().Update(context.Background(), n); err != nil {
		log.Error(err, "failed to update N3000Node status")
		return err
	}

	return nil
}

func (r *N3000NodeReconciler) updateFlashCondition(n *fpgav1.N3000Node, status metav1.ConditionStatus,
	reason FlashConditionReason, msg string) {
	log := r.log.WithName("updateFlashCondition")
	fc := metav1.Condition{
		Type:               FlashCondition,
		Status:             status,
		Reason:             string(reason),
		Message:            msg,
		ObservedGeneration: n.GetGeneration(),
	}
	if err := r.updateStatus(n, []metav1.Condition{fc}); err != nil {
		log.Error(err, "failed to update N3000Node flash condition")
	}
}

func (r *N3000NodeReconciler) verifySpec(n *fpgav1.N3000Node) error {
	for _, f := range n.Spec.FPGA {
		if f.UserImageURL == "" {
			return errors.New("Missing UserImageURL for PCI: " + f.PCIAddr)
		}
	}

	if len(n.Spec.Fortville.MACs) > 0 {
		if n.Spec.Fortville.FirmwareURL == "" {
			return errors.New("Missing Fortville FirmwareURL")
		}
	}
	return nil
}

func (r *N3000NodeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.log.WithName("Reconcile").WithValues("namespace", req.Namespace, "name", req.Name)

	// Lack of update on namespace/name mismatch (like in N3000ClusterReconciler):
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

	err := r.verifySpec(n3000node)
	if err != nil {
		log.Error(err, "verifySpec error")
		r.updateFlashCondition(n3000node, metav1.ConditionFalse, FlashFailed, err.Error())
		return ctrl.Result{}, nil
	}

	if n3000node.Spec.FPGA == nil && n3000node.Spec.Fortville.MACs == nil {
		log.Info("Nothing to do")
		r.updateFlashCondition(n3000node, metav1.ConditionFalse, FlashNotRequested, "Inventory up to date")
		return ctrl.Result{}, nil
	}

	// Update current condition to reflect that the flash started
	currentCondition := meta.FindStatusCondition(n3000node.Status.Conditions, FlashCondition)
	if currentCondition != nil {
		currentCondition.Status = metav1.ConditionFalse
		currentCondition.Reason = string(FlashInProgress)
		currentCondition.Message = "Flash started"
		if err := r.updateStatus(n3000node, []metav1.Condition{*currentCondition}); err != nil {
			log.Error(err, "failed to update current N3000Node flash condition")
			return ctrl.Result{}, err
		}
	}

	if n3000node.Spec.FPGA != nil {
		err := r.fpga.verifyPreconditions(n3000node)
		if err != nil {
			r.updateFlashCondition(n3000node, metav1.ConditionFalse, FlashFailed, err.Error())
			return ctrl.Result{}, nil
		}
	}

	if n3000node.Spec.Fortville.MACs != nil {
		err = r.fortville.verifyPreconditions(n3000node)
		if err != nil {
			r.updateFlashCondition(n3000node, metav1.ConditionFalse, FlashFailed, err.Error())
			return ctrl.Result{}, nil
		}
	}

	var flashErr error
	err = r.drainHelper.Run(func(c context.Context) bool {
		if n3000node.Spec.FPGA != nil {
			err := r.fpga.ProgramFPGAs(n3000node)
			if err != nil {
				log.Error(err, "Unable to flash FPGA")
				flashErr = err
				return true
			}
		}

		if n3000node.Spec.Fortville.MACs != nil {
			err = r.fortville.flash(n3000node)
			if err != nil {
				log.Error(err, "Unable to flash Fortville")
				flashErr = err
				return true
			}
		}
		return true
	}, !n3000node.Spec.DrainSkip)

	if err != nil {
		// some kind of error around leader election / node (un)cordon / node drain
		r.updateFlashCondition(n3000node, metav1.ConditionUnknown, FlashUnknown, err.Error())
		return ctrl.Result{}, nil
	}

	if flashErr != nil {
		r.updateFlashCondition(n3000node, metav1.ConditionFalse, FlashFailed, flashErr.Error())
	} else {
		r.updateFlashCondition(n3000node, metav1.ConditionTrue, FlashSucceeded, "Flashed successfully")
	}

	log.Info("Reconciled")
	return ctrl.Result{}, nil
}
