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

func (r *N3000NodeReconciler) createStatus(n *fpgav1.N3000Node) (*fpgav1.N3000NodeStatus, error) {
	log := r.log.WithName("createStatus")

	fortvilleStatus, err := r.fortville.getInventory()
	if err != nil {
		log.Error(err, "Failed to get Fortville inventory")
		return nil, err
	}

	fpgaStatus, err := getFPGAInventory(r.log)
	if err != nil {
		log.Error(err, "Failed to get FPGA inventory")
		return nil, err
	}

	return &fpgav1.N3000NodeStatus{
		Fortville: fortvilleStatus,
		FPGA:      fpgaStatus,
	}, nil
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

	lastCondition := meta.FindStatusCondition(n3000node.Status.Conditions, "Flashed")
	if lastCondition != nil {
		if lastCondition.ObservedGeneration == n3000node.GetGeneration() {
			log.Info("CR already handled")
			return ctrl.Result{}, nil
		}
	}

	flashCondition := metav1.Condition{
		Type:               "Flashed",
		Status:             metav1.ConditionTrue,
		Message:            "Inventory up to date",
		ObservedGeneration: n3000node.GetGeneration(),
		Reason:             "FlashNotRequested",
	}

	err := r.verifySpec(n3000node)
	if err != nil {
		log.Error(err, "verifySpec error")
		flashCondition.Status = metav1.ConditionFalse
		flashCondition.Message = err.Error()
		flashCondition.Reason = "FlashFailed"
	} else {
		if n3000node.Spec.FPGA != nil || n3000node.Spec.Fortville.MACs != nil {
			if n3000node.Spec.FPGA != nil {
				err := r.fpga.verifyPreconditions(n3000node)
				if err != nil {
					return ctrl.Result{}, err
				}
			}

			if n3000node.Spec.Fortville.MACs != nil {
				err = r.fortville.verifyPreconditions(n3000node)
				if err != nil {
					return ctrl.Result{}, err
				}
			}

			var flashErr error
			var result ctrl.Result

			err := r.drainHelper.Run(func(c context.Context) {
				if n3000node.Spec.FPGA != nil {
					err := r.fpga.ProgramFPGAs(n3000node)
					if err != nil {
						log.Error(err, "Unable to flash FPGA")
						flashErr = err
						return
					}
				}

				if n3000node.Spec.Fortville.MACs != nil {
					err = r.fortville.flash(n3000node)
					if err != nil {
						log.Error(err, "Unable to flash Fortville")
						flashErr = err
						return
					}
				}
			})

			if err != nil {
				// some kind of error around leader election / node (un)cordon / node drain
				return result, err
			}

			if flashErr != nil {
				flashCondition.Status = metav1.ConditionFalse
				flashCondition.Message = flashErr.Error()
				flashCondition.Reason = "FlashFailed"
			} else {
				flashCondition.Status = metav1.ConditionTrue
				flashCondition.Message = "Flashed successfully"
				flashCondition.Reason = "FlashSucceeded"
			}
		}
	}

	s, err := r.createStatus(n3000node)
	if err != nil {
		log.Error(err, "failed to get status")
		return ctrl.Result{}, nil
	}

	meta.SetStatusCondition(&s.Conditions, flashCondition)

	n3000node.Status = *s
	if err := r.Status().Update(context.Background(), n3000node); err != nil {
		log.Error(err, "failed to update N3000Node status")
		return ctrl.Result{}, nil
	}

	log.Info("Reconciled")

	return ctrl.Result{}, nil
}
