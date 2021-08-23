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
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sriovfecv2 "github.com/otcshare/openshift-operator/sriov-fec/api/v2"
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

//TODO: if ClusterConfig is already Succeeded, but a new node is added/labeled - should we update node or skip it?
func (r *SriovFecClusterConfigReconciler) Reconcile(_ context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("sriovfecclusterconfig", req.NamespacedName)
	log.V(2).Info("Reconciling SriovFecClusterConfig")

	allClusterConfigs := new(sriovfecv2.SriovFecClusterConfigList)
	if err := r.List(context.TODO(), allClusterConfigs, client.InNamespace(NAMESPACE)); err != nil {
		log.V(1).Info("cannot obtain list of SriovFecClusterConfig, rescheduling rescheduling reconcile call", "err", err)
		return ctrl.Result{}, err
	}

	nodes, err := r.getAcceleratedNodes()
	if err != nil {
		log.V(1).Info("cannot obtain list of accelerated nodes, rescheduling rescheduling reconcile call", "err", err)
		return reconcile.Result{}, err
	}

	clusterConfigurationMatcher := createClusterConfigMatcher(r.getOrInitializeSriovFecNodeConfig, log)
	for _, node := range nodes {
		configurationContextProvider, err := clusterConfigurationMatcher.match(node, allClusterConfigs.Items)
		if err != nil {
			log.V(1).Info("Error when matching SriovFecClusterConfigs", "node", node.Name, "error", err)
			continue
		}

		if err := r.synchronizeNodeConfigSpec(*configurationContextProvider); err != nil {
			log.V(1).Info("failed to propagate configuration into SriovFecNodeConfig", "name", node.Name, "error", err)

			err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				snc := new(sriovfecv2.SriovFecNodeConfig)
				if err := r.Get(context.TODO(), types.NamespacedName{Namespace: NAMESPACE, Name: node.Name}, snc); err != nil {
					return err
				}

				setConfigurationPropagationConditionFailed(&snc.Status.Conditions, snc.GetGeneration(), err.Error())
				return r.Status().Update(context.TODO(), snc)
			})

			if err != nil {
				log.V(1).Info("failed to set ConfigurationPropagationCondition for SriovFecNodeConfig", "name", node.Name, "error", err)
			}
			continue
		}
	}

	return ctrl.Result{}, err
}

func (r *SriovFecClusterConfigReconciler) synchronizeNodeConfigSpec(ncc NodeConfigurationCtx) error {
	copyWithEmptySpec := func(nc sriovfecv2.SriovFecNodeConfig) *sriovfecv2.SriovFecNodeConfig {
		newNC := nc.DeepCopy()
		newNC.Spec = sriovfecv2.SriovFecNodeConfigSpec{
			PhysicalFunctions: []sriovfecv2.PhysicalFunctionConfigExt{},
		}
		return newNC
	}

	currentNodeConfig := ncc.SriovFecNodeConfig
	acceleratorConfigContext := ncc.AcceleratorConfigContext

	newNodeConfig := copyWithEmptySpec(ncc.SriovFecNodeConfig)

	for pciAddress, cc := range acceleratorConfigContext {
		pf := sriovfecv2.PhysicalFunctionConfigExt{PCIAddress: pciAddress}
		pf.PhysicalFunctionConfig = cc.Spec.PhysicalFunction
		newNodeConfig.Spec.PhysicalFunctions = append(newNodeConfig.Spec.PhysicalFunctions, pf)
	}

	if !equality.Semantic.DeepEqual(newNodeConfig.Spec, currentNodeConfig.Spec) {
		return r.Update(context.TODO(), newNodeConfig)
	}
	return nil
}

func (r *SriovFecClusterConfigReconciler) getAcceleratedNodes() ([]corev1.Node, error) {
	nl := new(corev1.NodeList)
	labelsToMatch := &client.MatchingLabels{
		"fpga.intel.com/intel-accelerator-present": "",
	}
	if err := r.List(context.TODO(), nl, labelsToMatch); err != nil {
		return nil, err
	}
	return nl.Items, nil
}

func (r *SriovFecClusterConfigReconciler) getOrInitializeSriovFecNodeConfig(name string) (*sriovfecv2.SriovFecNodeConfig, error) {
	nc := new(sriovfecv2.SriovFecNodeConfig)
	if err := r.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: NAMESPACE}, nc); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		nc.Name = name
		nc.Namespace = NAMESPACE
		nc.Spec.PhysicalFunctions = []sriovfecv2.PhysicalFunctionConfigExt{}
	}
	return nc, nil
}

func (r *SriovFecClusterConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Add NodeConfigs & DaemonSet
	return ctrl.NewControllerManagedBy(mgr).
		For(&sriovfecv2.SriovFecClusterConfig{}).
		Complete(r)
}

// key: accelerator pciAddress
type AcceleratorConfigContext map[string]sriovfecv2.SriovFecClusterConfig
type NodeConfigurationCtx struct {
	sriovfecv2.SriovFecNodeConfig
	AcceleratorConfigContext
}

func createClusterConfigMatcher(ncp nodeConfigProvider, l logr.Logger) *clusterConfigMatcher {
	return &clusterConfigMatcher{
		getNodeConfig: ncp,
		log:           l,
	}
}

type nodeConfigProvider func(nodeName string) (*sriovfecv2.SriovFecNodeConfig, error)

type clusterConfigMatcher struct {
	getNodeConfig nodeConfigProvider
	log           logr.Logger
}

func (pm *clusterConfigMatcher) match(node corev1.Node, allConfigs []sriovfecv2.SriovFecClusterConfig) (*NodeConfigurationCtx, error) {

	matchingClusterConfigs := matchConfigsForNode(&node, allConfigs)
	nodeConfig, err := pm.getNodeConfig(node.Name)
	if err != nil {
		return nil, fmt.Errorf("error occurred when reading SriovFecNodeConfig: %s", err.Error())
	}

	acceleratorConfigContext := pm.prepareAcceleratorConfigContext(nodeConfig, matchingClusterConfigs)
	return &NodeConfigurationCtx{*nodeConfig, acceleratorConfigContext}, nil
}

func (pm *clusterConfigMatcher) prepareAcceleratorConfigContext(nodeConfig *sriovfecv2.SriovFecNodeConfig, configs []sriovfecv2.SriovFecClusterConfig) AcceleratorConfigContext {
	acceleratorConfigContext := make(AcceleratorConfigContext)
	for _, current := range configs {
		for _, accelerator := range nodeConfig.Status.Inventory.SriovAccelerators {
			if current.Spec.AcceleratorSelector.Matches(accelerator) {

				if _, ok := acceleratorConfigContext[accelerator.PCIAddress]; !ok {
					acceleratorConfigContext[accelerator.PCIAddress] = current
					continue
				}

				previous := acceleratorConfigContext[accelerator.PCIAddress]
				switch {
				case current.Spec.Priority > previous.Spec.Priority: //override with higher prioritized config
					acceleratorConfigContext[accelerator.PCIAddress] = current
				case current.Spec.Priority == previous.Spec.Priority: //multiple configs with same priority; drop older one
					//TODO: Update Timestamp would be better than CreationTime
					if current.CreationTimestamp.After(previous.CreationTimestamp.Time) {
						pm.log.V(2).
							Info("Dropping older ClusterConfig",
								"Node", nodeConfig.Name,
								"SriovFecClusterConfig", previous.Name,
								"Priority", previous.Spec.Priority,
								"CreationTimestamp", previous.CreationTimestamp.String())

						acceleratorConfigContext[accelerator.PCIAddress] = current
					}

				case current.Spec.Priority < previous.Spec.Priority: //drop current with lower priority
					pm.log.V(2).
						Info("Dropping low prioritized ClusterConfig",
							"node", nodeConfig.Name,
							"SriovFecClusterConfig", current.Name,
							"priority", current.Spec.Priority)
				}
			}
		}
	}
	return acceleratorConfigContext
}

func matchConfigsForNode(node *corev1.Node, allConfigs []sriovfecv2.SriovFecClusterConfig) (nodeConfigs []sriovfecv2.SriovFecClusterConfig) {
	nodeLabels := labels.Set(node.Labels)
	for _, config := range allConfigs {
		nodeSelector := labels.Set(config.Spec.NodeSelector)
		if nodeSelector.AsSelector().Matches(nodeLabels) {
			nodeConfigs = append(nodeConfigs, config)
		}
	}
	return
}

func setConfigurationPropagationConditionFailed(conditions *[]metav1.Condition, generation int64, msg string) {
	meta.SetStatusCondition(conditions, metav1.Condition{
		Type:               "ConfigurationPropagationCondition",
		Status:             metav1.ConditionFalse,
		ObservedGeneration: generation,
		Reason:             "Failed",
		Message:            msg,
	})
}
