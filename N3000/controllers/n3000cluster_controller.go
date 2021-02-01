// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fpgav1 "github.com/open-ness/openshift-operator/N3000/api/v1"
)

const (
	DEFAULT_N3000_CONFIG_NAME = "n3000"
)

var log = ctrl.Log.WithName("N3000ClusterController")
var namespace = os.Getenv("NAMESPACE")

func (r *N3000ClusterReconciler) updateStatus(n3000cluster *fpgav1.N3000Cluster,
	status fpgav1.SyncStatus, reason string) {
	n3000cluster.Status.SyncStatus = status
	n3000cluster.Status.LastSyncError = reason
	if err := r.Status().Update(context.Background(), n3000cluster, &client.UpdateOptions{}); err != nil {
		log.Error(err, "failed to update cluster config's status")
	}
}

// N3000ClusterReconciler reconciles a N3000Cluster object
type N3000ClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=fpga.intel.com,resources=n3000clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=fpga.intel.com,resources=n3000clusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=fpga.intel.com,resources=n3000nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=*
// +kubebuilder:rbac:groups="",resources=services;serviceaccounts,verbs=*
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=serviceaccounts;roles;rolebindings;clusterroles;clusterrolebindings,verbs=*
// +kubebuilder:rbac:groups=apps,resources=daemonsets;deployments;deployments/finalizers,verbs=*
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;create;update
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=get;create;update

func (r *N3000ClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	log.Info("Reconciling N3000ClusterReconciler", "name", req.Name, "namespace", req.Namespace)

	clusterConfig := &fpgav1.N3000Cluster{}
	err := r.Client.Get(ctx, req.NamespacedName, clusterConfig)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Info("N3000Cluster config not found", "namespacedName", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// To simplify things, only specific CR is honored (Name: DEFAULT_N3000_CONFIG_NAME, Namespace: namespace)
	// Any other N3000Cluster config is ignored
	if req.Namespace != namespace || req.Name != DEFAULT_N3000_CONFIG_NAME {
		log.Info("received N3000Cluster, but it not an expected one - it'll be ignored",
			"expectedNamespace", namespace, "expectedName", DEFAULT_N3000_CONFIG_NAME)

		r.updateStatus(clusterConfig, fpgav1.IgnoredSync, fmt.Sprintf(
			"Only N3000Cluster with name '%s' and namespace '%s' are handled",
			DEFAULT_N3000_CONFIG_NAME, namespace))

		return ctrl.Result{}, nil
	}

	n3000nodes, err := r.splitClusterIntoNodes(ctx, clusterConfig)
	if err != nil {
		log.Error(err, "cluster into nodes split failed")
		return ctrl.Result{RequeueAfter: time.Second * 5}, err
	}

	if err = r.removeOldNodes(n3000nodes); err != nil {
		log.Error(err, "removing old nodes failed")
		return ctrl.Result{RequeueAfter: time.Second * 5}, err
	}

	for _, node := range n3000nodes {
		err := r.updateOrCreateNodeConfig(node)
		if err != nil {
			log.Error(err, "create or update failed")
			return reconcile.Result{}, err
		}
	}

	r.updateStatus(clusterConfig, fpgav1.SucceededSync, "")
	return ctrl.Result{}, nil
}

func (r *N3000ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fpgav1.N3000Cluster{}).
		Complete(r)
}
func (r *N3000ClusterReconciler) updateOrCreateNodeConfig(nodeCfg *fpgav1.N3000Node) error {
	log := r.Log.WithName("updateOrCreateNodeConfig")
	log.Info("syncing node config", "name", nodeCfg.Name)

	prev := &fpgav1.N3000Node{}

	// try to get previous NodeConfig, if it does not exist - create, if exists - update
	if err := r.Get(context.TODO(),
		types.NamespacedName{Namespace: nodeCfg.Namespace, Name: nodeCfg.Name}, prev); err != nil {

		if errors.IsNotFound(err) {
			log.Info("old NodeConfig not found - creating", "name", nodeCfg.Name)
			if err := r.Create(context.TODO(), nodeCfg); err != nil {
				log.Error(err, "failed to create NodeConfig", "name", nodeCfg.Name)
				return err
			}
		} else {
			log.Error(err, "previous NodeConfig Get failed", "name", nodeCfg.Name)
			return err
		}
	} else {
		log.Info("previous NodeConfig found - updating", "name", nodeCfg.Name)

		prev.Spec = nodeCfg.Spec
		if err := r.Update(context.TODO(), prev); err != nil {
			log.Error(err, "failed to update NodeConfig", "name", nodeCfg.Name)
			return err
		}
	}

	return nil
}
func (r *N3000ClusterReconciler) splitClusterIntoNodes(ctx context.Context,
	n3000cluster *fpgav1.N3000Cluster) ([]*fpgav1.N3000Node, error) {

	nodes := &corev1.NodeList{}
	err := r.Client.List(ctx, nodes, &client.MatchingLabels{"fpga.intel.com/intel-accelerator-present": ""})
	if err != nil {
		log.Error(err, "Unable to list the nodes")
		return nil, err
	}

	var n3000Nodes []*fpgav1.N3000Node

	for _, res := range n3000cluster.Spec.Nodes {
		for _, node := range nodes.Items {
			if res.NodeName == node.Name {
				nodeRes := &fpgav1.N3000Node{}
				nodeRes.ObjectMeta = metav1.ObjectMeta{
					Name:      "n3000node-" + res.NodeName,
					Namespace: namespace,
				}
				nodeRes.Spec.FPGA = res.FPGA
				nodeRes.Spec.Fortville = res.Fortville
				nodeRes.Spec.DryRun = n3000cluster.Spec.DryRun
				nodeRes.Spec.DrainSkip = n3000cluster.Spec.DrainSkip
				n3000Nodes = append(n3000Nodes, nodeRes)
				break
			}
		}
	}

	return n3000Nodes, nil
}

func (r *N3000ClusterReconciler) removeOldNodes(newNodeCfgs []*fpgav1.N3000Node) error {
	log := r.Log.WithName("removeOldNodes")

	// existing NodeConfigs which are not part of the new ClusterConfig are removed
	// daemons will reiterate the devices and recreate NodeConfigs with empty spec and filled status

	nodes := &fpgav1.N3000NodeList{}
	if err := r.List(context.TODO(), nodes, &client.ListOptions{}); err != nil && !errors.IsNotFound(err) {
		log.Error(err, "failed to get N3000NodeList")
		return err
	}

	for _, node := range nodes.Items {
		del := true
		for _, newNode := range newNodeCfgs {
			if node.GetName() == newNode.GetName() {
				del = false
				break
			}
		}

		if del {
			log.Info("deleting existing N3000Node", "name", node.GetName())
			if err := r.Delete(context.TODO(), &node, &client.DeleteOptions{}); err != nil {
				log.Error(err, "failed to delete existing N3000Node", "name", node.GetName())
				return err
			}
		}
	}

	return nil
}
