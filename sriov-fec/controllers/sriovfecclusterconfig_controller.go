// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sriovfecv1 "github.com/otcshare/openshift-operator/sriov-fec/api/v1"
)

const (
	DEFAULT_CLUSTER_CONFIG_NAME = "config"
)

var NAMESPACE = os.Getenv("SRIOV_FEC_NAMESPACE")

// SriovFecClusterConfigReconciler reconciles a SriovFecClusterConfig object
type SriovFecClusterConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=sriovfec.intel.com,resources=sriovfecclusterconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sriovfec.intel.com,resources=sriovfecclusterconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sriovfec.intel.com,resources=sriovfecnodeconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sriovfec.intel.com,resources=sriovfecnodeconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=list;watch
// +kubebuilder:rbac:groups="",resources=namespaces;serviceaccounts;configmaps,verbs=*
// +kubebuilder:rbac:groups=apps,resources=daemonsets;deployments;deployments/finalizers,verbs=*
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=*
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=*

func (r *SriovFecClusterConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("sriovfecclusterconfig", req.NamespacedName)
	log.Info("Reconciling SriovFecClusterConfig")

	clusterConfig := &sriovfecv1.SriovFecClusterConfig{}
	if err := r.Get(context.TODO(), req.NamespacedName, clusterConfig); err != nil {
		if errors.IsNotFound(err) {
			log.Info("SriovFecClusterConfig not found", "namespacedName", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	updateStatus := func(status sriovfecv1.SyncStatus, reason string) {
		clusterConfig.Status.SyncStatus = status
		clusterConfig.Status.LastSyncError = reason
		if err := r.Status().Update(context.TODO(), clusterConfig, &client.UpdateOptions{}); err != nil {
			log.Error(err, "failed to update cluster config's status")
		}
	}

	// To simplify things, only specific CR is honored (Name: DEFAULT_CLUSTER_CONFIG_NAME, Namespace: NAMESPACE)
	// Any other SriovFecClusterConfig is ignored
	if req.Namespace != NAMESPACE || req.Name != DEFAULT_CLUSTER_CONFIG_NAME {
		log.Info("received ClusterConfig, but it not an expected one - it'll be ignored",
			"expectedNamespace", NAMESPACE, "expectedName", DEFAULT_CLUSTER_CONFIG_NAME)

		updateStatus(sriovfecv1.IgnoredSync, fmt.Sprintf(
			"Only SriovFecClusterConfig with name '%s' and namespace '%s' are handled",
			DEFAULT_CLUSTER_CONFIG_NAME, NAMESPACE))

		return reconcile.Result{}, nil
	}

	nodeList, err := r.getNodesWithIntelAccelerator()
	if err != nil {
		log.Error(err, "failed to obtain nodes with Intel accelerator")
		updateStatus(sriovfecv1.FailedSync, "nfd error: failed to obtain nodes with Intel accelerator - check logs")
		return reconcile.Result{}, err
	}

	log.Info("nodes with intel accelerator", "nodes", func() []string {
		names := []string{}
		for _, n := range nodeList.Items {
			names = append(names, n.Name)
		}
		return names
	}())

	nodeConfigs := r.renderNodeConfigs(clusterConfig, nodeList)
	if err := r.syncNodeConfigs(nodeConfigs); err != nil {
		log.Error(err, "syncNodeConfigs failed")
		updateStatus(sriovfecv1.FailedSync, "failed to create NodeConfigs - check logs")
		return reconcile.Result{}, err
	}

	updateStatus(sriovfecv1.SucceededSync, "")

	return reconcile.Result{}, nil
}

func (r *SriovFecClusterConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Add NodeConfigs & DaemonSet
	return ctrl.NewControllerManagedBy(mgr).
		For(&sriovfecv1.SriovFecClusterConfig{}).
		Complete(r)
}

func (r *SriovFecClusterConfigReconciler) getNodesWithIntelAccelerator() (*corev1.NodeList, error) {
	nodeList := &corev1.NodeList{}

	labelsToMatch := &client.MatchingLabels{
		"fpga.intel.com/intel-accelerator-present": "",
	}
	err := r.List(context.TODO(), nodeList, labelsToMatch)
	if err != nil {
		return nil, err
	}

	return nodeList, nil
}

func (r *SriovFecClusterConfigReconciler) renderNodeConfigs(clusterConfig *sriovfecv1.SriovFecClusterConfig,
	nodeList *corev1.NodeList) []sriovfecv1.SriovFecNodeConfig {

	log := r.Log.WithName("renderNodeConfigs")
	log.Info("rendering new node configs")

	nodeConfigs := []sriovfecv1.SriovFecNodeConfig{}

	nodeHasAccelerator := func(nodeName string) bool {
		// check user-provided NodeName against list of nodes with accelerators according to the NFD
		for _, node := range nodeList.Items {
			if node.Name == nodeName {
				return true
			}
		}

		return false
	}

	for _, nodeConfigSpec := range clusterConfig.Spec.Nodes {
		if !nodeHasAccelerator(nodeConfigSpec.NodeName) {
			log.Info("received config for node that has no accelerator - NodeConfig spec will not be generated",
				"nodeName", nodeConfigSpec.NodeName)
			continue
		}

		nodeCfg := sriovfecv1.SriovFecNodeConfig{
			TypeMeta: v1.TypeMeta{
				APIVersion: "v1",
				Kind:       "SriovFecNodeConfig",
			},
			Spec: sriovfecv1.SriovFecNodeConfigSpec{
				PhysicalFunctions: nodeConfigSpec.PhysicalFunctions,
				DrainSkip:         clusterConfig.Spec.DrainSkip,
			},
		}
		nodeCfg.SetName(nodeConfigSpec.NodeName)
		nodeCfg.SetNamespace(NAMESPACE)

		log.Info("creating nodeConfig", "nodeName", nodeConfigSpec.NodeName)

		nodeConfigs = append(nodeConfigs, nodeCfg)
	}

	return nodeConfigs
}

func (r *SriovFecClusterConfigReconciler) syncNodeConfigs(nodeCfgs []sriovfecv1.SriovFecNodeConfig) error {
	log := r.Log.WithName("syncNodeConfigs")
	log.Info("syncing node configs")

	if err := r.removeOldNodeConfigs(nodeCfgs); err != nil {
		return err
	}

	for _, nodeCfg := range nodeCfgs {
		if err := r.updateOrCreateNodeConfig(nodeCfg); err != nil {
			return err
		}
	}

	return nil
}

func (r *SriovFecClusterConfigReconciler) updateOrCreateNodeConfig(nodeCfg sriovfecv1.SriovFecNodeConfig) error {
	log := r.Log.WithName("updateOrCreateNodeConfig")
	log.Info("syncing node config", "name", nodeCfg.Name)

	prev := &sriovfecv1.SriovFecNodeConfig{}

	// try to get previous NodeConfig, if it does not exist - create, if exists - update
	if err := r.Get(context.TODO(),
		types.NamespacedName{Namespace: nodeCfg.Namespace, Name: nodeCfg.Name}, prev); err != nil {

		if errors.IsNotFound(err) {
			log.Info("old NodeConfig not found - creating", "name", nodeCfg.Name)
			if err := r.Create(context.TODO(), &nodeCfg); err != nil {
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

func (r *SriovFecClusterConfigReconciler) removeOldNodeConfigs(newNodeCfgs []sriovfecv1.SriovFecNodeConfig) error {
	log := r.Log.WithName("removeOldNodeConfigs")

	// existing NodeConfigs which are not part of the new ClusterConfig are removed
	// daemons will deconfigure devices and recreate NodeConfigs with empty spec and filled status

	ncList := &sriovfecv1.SriovFecNodeConfigList{}
	if err := r.List(context.TODO(), ncList, &client.ListOptions{}); err != nil && !errors.IsNotFound(err) {
		log.Error(err, "failed to get SriovFecNodeConfigList")
		return err
	}

	for _, nc := range ncList.Items {
		deleteNC := true
		for _, nNC := range newNodeCfgs {
			if nc.GetName() == nNC.GetName() {
				deleteNC = false
				break
			}
		}

		if deleteNC {
			log.Info("deleting existing NodeConfig", "name", nc.GetName())
			if err := r.Delete(context.TODO(), &nc, &client.DeleteOptions{}); err != nil {
				log.Error(err, "failed to delete existing NodeConfig", "name", nc.GetName())
				return err
			}
		}
	}

	return nil
}
