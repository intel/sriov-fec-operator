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
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	sriovfecv2 "github.com/smart-edge-open/openshift-operator/sriov-fec/api/v2"
)

var NAMESPACE = os.Getenv("SRIOV_FEC_NAMESPACE")

// SriovFecClusterConfigReconciler reconciles a SriovFecClusterConfig object
type SriovFecClusterConfigReconciler struct {
	client.Client
	Log    *logrus.Logger
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

func (r *SriovFecClusterConfigReconciler) Reconcile(_ context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Infof("Reconcile(...) triggered by %s", req.NamespacedName.String())

	clusterConfigList := new(sriovfecv2.SriovFecClusterConfigList)
	if err := r.List(context.TODO(), clusterConfigList, client.InNamespace(NAMESPACE)); err != nil {
		r.Log.WithError(err).Error("cannot obtain list of SriovFecClusterConfig, rescheduling rescheduling reconcile call")
		return ctrl.Result{}, err
	}

	for _, sfcc := range clusterConfigList.Items {
		if len(sfcc.Spec.Nodes) != 0 {
			return r.convertClusterConfigWithNodesField(sfcc)
		}
	}

	nodes, err := r.getAcceleratedNodes()
	if err != nil {
		r.Log.WithError(err).Info("cannot obtain list of accelerated nodes, rescheduling rescheduling reconcile call")
		return reconcile.Result{}, err
	}

	clusterConfigurationMatcher := createClusterConfigMatcher(r.getOrInitializeSriovFecNodeConfig, r.Log)
	for _, node := range nodes {
		configurationContextProvider, err := clusterConfigurationMatcher.match(node, clusterConfigList.Items)
		if err != nil {
			r.Log.WithField("node", node.Name).WithField("error", err).Info("Error when matching SriovFecClusterConfigs")
			continue
		}

		if err := r.synchronizeNodeConfigSpec(*configurationContextProvider); err != nil {
			r.Log.WithField("name", node.Name).WithField("error", err).Info("failed to propagate configuration into SriovFecNodeConfig")

			err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				snc := new(sriovfecv2.SriovFecNodeConfig)
				if err := r.Get(context.TODO(), types.NamespacedName{Namespace: NAMESPACE, Name: node.Name}, snc); err != nil {
					return err
				}

				setConfigurationPropagationConditionFailed(&snc.Status.Conditions, snc.GetGeneration(), err.Error())
				r.Log.
					WithField("sfnc", snc).
					Info("updating svnc status")
				return r.Status().Update(context.TODO(), snc)
			})

			if err != nil {
				r.Log.WithError(err).WithField("name", node.Name).Error("failed to set ConfigurationPropagationCondition for SriovFecNodeConfig")
			}
			continue
		}
	}

	return r.requeueIfClusterConfigExists(req.NamespacedName)
}

func (r *SriovFecClusterConfigReconciler) requeueIfClusterConfigExists(cc types.NamespacedName) (ctrl.Result, error) {
	sfcc := &sriovfecv2.SriovFecClusterConfig{}
	err := r.Get(context.TODO(), cc, sfcc)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get ClusterConfig to determine whenever reconcile is needed - %v", err)
	}

	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

func (r *SriovFecClusterConfigReconciler) convertClusterConfigWithNodesField(sfcc sriovfecv2.SriovFecClusterConfig) (ctrl.Result, error) {
	r.Log.Infof("converting %v", sfcc.Name)
	for ncIdx, nodeConfig := range sfcc.Spec.Nodes {
		for pfIdx, pf := range nodeConfig.PhysicalFunctions {
			newCC := sfcc.DeepCopy()
			newCC.Name = fmt.Sprintf("%s-%d-%d", newCC.Name, ncIdx, pfIdx)
			newCC.Spec.Nodes = nil
			newCC.Spec.NodeSelector = map[string]string{
				"kubernetes.io/hostname": nodeConfig.NodeName,
			}
			newCC.Spec.AcceleratorSelector = sriovfecv2.AcceleratorSelector{
				PCIAddress: pf.PCIAddress,
			}
			newCC.Spec.PhysicalFunction = sriovfecv2.PhysicalFunctionConfig{
				PFDriver:    pf.PFDriver,
				VFDriver:    pf.VFDriver,
				VFAmount:    pf.VFAmount,
				BBDevConfig: pf.BBDevConfig,
			}
			newCC.ResourceVersion = ""
			err := r.Create(context.TODO(), newCC)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to write converted ClusterConfig - %v", err)
			}
		}
	}
	err := r.Client.Delete(context.TODO(), &sfcc, &client.DeleteOptions{})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to delete converted ClusterConfig - %v", err)
	}
	r.Log.Infof("Successfully converted %v ClusterConfig", sfcc.Name)
	return ctrl.Result{}, nil
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
		pf := sriovfecv2.PhysicalFunctionConfigExt{
			PCIAddress:  pciAddress,
			PFDriver:    cc.Spec.PhysicalFunction.PFDriver,
			VFDriver:    cc.Spec.PhysicalFunction.VFDriver,
			VFAmount:    cc.Spec.PhysicalFunction.VFAmount,
			BBDevConfig: cc.Spec.PhysicalFunction.BBDevConfig,
		}
		newNodeConfig.Spec.DrainSkip = newNodeConfig.Spec.DrainSkip || cc.Spec.DrainSkip
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

func createClusterConfigMatcher(ncp nodeConfigProvider, l *logrus.Logger) *clusterConfigMatcher {
	return &clusterConfigMatcher{
		getNodeConfig: ncp,
		log:           l,
	}
}

type nodeConfigProvider func(nodeName string) (*sriovfecv2.SriovFecNodeConfig, error)

type clusterConfigMatcher struct {
	getNodeConfig nodeConfigProvider
	log           *logrus.Logger
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
						pm.log.WithFields(logrus.Fields{
							"Node":                  nodeConfig.Name,
							"SriovFecClusterConfig": previous.Name,
							"Priority":              previous.Spec.Priority,
							"CreationTimestamp":     previous.CreationTimestamp.String(),
						}).Info("Dropping older ClusterConfig")

						acceleratorConfigContext[accelerator.PCIAddress] = current
					}

				case current.Spec.Priority < previous.Spec.Priority: //drop current with lower priority
					pm.log.WithFields(logrus.Fields{
						"node":                  nodeConfig.Name,
						"SriovFecClusterConfig": current.Name,
						"priority":              current.Spec.Priority,
					}).Info("Dropping low prioritized ClusterConfig")
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
