// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2023 Intel Corporation

package sriovvrb

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vrbv1 "github.com/intel/sriov-fec-operator/api/sriovvrb/v1"
)

var NAMESPACE = os.Getenv("SRIOV_FEC_NAMESPACE")

// VrbclusterconfigReconciler reconciles a Vrbclusterconfig object
type SriovVrbClusterConfigReconciler struct {
	client.Client
	Log *logrus.Logger
}

// +kubebuilder:rbac:groups=sriovvrb.intel.com,resources=sriovvrbclusterconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sriovvrb.intel.com,resources=sriovvrbclusterconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sriovvrb.intel.com,resources=sriovvrbnodeconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sriovvrb.intel.com,resources=sriovvrbnodeconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=list;get;watch;update;patch
// +kubebuilder:rbac:groups="",resources=namespaces;serviceaccounts;secrets;configmaps,verbs=get;list;create;update
// +kubebuilder:rbac:groups=apps,resources=daemonsets;deployments;deployments/finalizers,verbs=get;list;create;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=get;list;create;update
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=use,resourceNames=privileged
// +kubebuilder:rbac:groups="",resources=pods/eviction,verbs=create
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *SriovVrbClusterConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Infof("Reconcile(...) triggered by %s", req.NamespacedName.String())

	clusterConfigList := new(vrbv1.SriovVrbClusterConfigList)
	if err := r.List(context.TODO(), clusterConfigList, client.InNamespace(NAMESPACE)); err != nil {
		r.Log.WithError(err).Error("cannot obtain list of SriovVrbClusterConfig, rescheduling rescheduling reconcile call")
		return ctrl.Result{}, err
	}

	nodes, err := r.getAcceleratedNodes()
	if err != nil {
		r.Log.WithError(err).Info("cannot obtain list of accelerated nodes, rescheduling rescheduling reconcile call")
		return reconcile.Result{}, err
	}

	clusterConfigurationMatcher := createClusterConfigMatcher(r.getOrInitializeSriovVrbNodeConfig, r.Log)
	for _, node := range nodes {
		configurationContextProvider, err := clusterConfigurationMatcher.match(node, clusterConfigList.Items)
		if err != nil {
			r.Log.WithField("node", node.Name).WithField("error", err).Info("Error when matching SriovVrbClusterConfigs")
			continue
		}

		if err := r.synchronizeNodeConfigSpec(*configurationContextProvider); err != nil {
			r.Log.WithField("name", node.Name).WithField("error", err).Info("failed to propagate configuration into SriovVrbNodeConfig")

			err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				snc := new(vrbv1.SriovVrbNodeConfig)
				if err := r.Get(context.TODO(), types.NamespacedName{Namespace: NAMESPACE, Name: node.Name}, snc); err != nil {
					return err
				}

				setConfigurationPropagationConditionFailed(&snc.Status.Conditions, snc.GetGeneration(), err.Error())
				r.Log.
					WithField("vrbnc", snc).
					Info("updating svnc status")
				return r.Status().Update(context.TODO(), snc)
			})

			if err != nil {
				r.Log.WithError(err).WithField("name", node.Name).Error("failed to set ConfigurationPropagationCondition for SriovVrbNodeConfig")
			}
			continue
		}
	}

	return r.requeueIfClusterConfigExists(req.NamespacedName)
}

func (r *SriovVrbClusterConfigReconciler) requeueIfClusterConfigExists(cc types.NamespacedName) (ctrl.Result, error) {
	vrbcc := &vrbv1.SriovVrbClusterConfig{}
	err := r.Get(context.TODO(), cc, vrbcc)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get ClusterConfig to determine whenever reconcile is needed - %v", err)
	}

	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

func (r *SriovVrbClusterConfigReconciler) synchronizeNodeConfigSpec(ncc NodeConfigurationCtx) error {
	copyWithEmptySpec := func(nc vrbv1.SriovVrbNodeConfig) *vrbv1.SriovVrbNodeConfig {
		newNC := nc.DeepCopy()
		newNC.Spec = vrbv1.SriovVrbNodeConfigSpec{
			PhysicalFunctions: []vrbv1.PhysicalFunctionConfigExt{},
		}
		return newNC
	}

	currentNodeConfig := ncc.SriovVrbNodeConfig
	acceleratorConfigContext := ncc.AcceleratorConfigContext

	newNodeConfig := copyWithEmptySpec(ncc.SriovVrbNodeConfig)

	// Use orederedmap for iteration
	for _, pciAddress := range acceleratorConfigContext.Keys() {
		cc, _ := acceleratorConfigContext.Get(pciAddress)
		pf := vrbv1.PhysicalFunctionConfigExt{
			PCIAddress:  pciAddress,
			PFDriver:    cc.Spec.PhysicalFunction.PFDriver,
			VFDriver:    cc.Spec.PhysicalFunction.VFDriver,
			VFAmount:    cc.Spec.PhysicalFunction.VFAmount,
			BBDevConfig: cc.Spec.PhysicalFunction.BBDevConfig,
		}
		newNodeConfig.Spec.DrainSkip = newNodeConfig.Spec.DrainSkip || cc.Spec.DrainSkip
		newNodeConfig.Spec.PhysicalFunctions = append(newNodeConfig.Spec.PhysicalFunctions, pf)
	}

	// copy latest known drainSkip from NodeConfig for cleanup
	if acceleratorConfigContext.Len() == 0 {
		newNodeConfig.Spec.DrainSkip = ncc.Spec.DrainSkip
	}

	if !equality.Semantic.DeepEqual(newNodeConfig.Spec, currentNodeConfig.Spec) {
		r.Log.Info("Node Config Changed")
		return r.Update(context.TODO(), newNodeConfig)
	}
	return nil
}

func (r *SriovVrbClusterConfigReconciler) getAcceleratedNodes() ([]corev1.Node, error) {
	nl := new(corev1.NodeList)
	labelsToMatch := &client.MatchingLabels{
		"fpga.intel.com/intel-accelerator-present": "",
	}
	if err := r.List(context.TODO(), nl, labelsToMatch); err != nil {
		return nil, err
	}
	return nl.Items, nil
}

func (r *SriovVrbClusterConfigReconciler) getOrInitializeSriovVrbNodeConfig(name string) (*vrbv1.SriovVrbNodeConfig, error) {
	nc := new(vrbv1.SriovVrbNodeConfig)
	if err := r.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: NAMESPACE}, nc); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		nc.Name = name
		nc.Namespace = NAMESPACE
		nc.Spec.PhysicalFunctions = []vrbv1.PhysicalFunctionConfigExt{}
	}
	return nc, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SriovVrbClusterConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vrbv1.SriovVrbClusterConfig{}).
		Complete(r)
}

// key: accelerator pciAddress
type NodeConfigurationCtx struct {
	vrbv1.SriovVrbNodeConfig
	AcceleratorConfigContext *orderedmap.OrderedMap[string, vrbv1.SriovVrbClusterConfig]
}

func createClusterConfigMatcher(ncp nodeConfigProvider, l *logrus.Logger) *clusterConfigMatcher {
	return &clusterConfigMatcher{
		getNodeConfig: ncp,
		log:           l,
	}
}

type nodeConfigProvider func(nodeName string) (*vrbv1.SriovVrbNodeConfig, error)

type clusterConfigMatcher struct {
	getNodeConfig nodeConfigProvider
	log           *logrus.Logger
}

func (pm *clusterConfigMatcher) match(node corev1.Node, allConfigs []vrbv1.SriovVrbClusterConfig) (*NodeConfigurationCtx, error) {

	matchingClusterConfigs := matchConfigsForNode(&node, allConfigs)
	nodeConfig, err := pm.getNodeConfig(node.Name)
	if err != nil {
		return nil, fmt.Errorf("error occurred when reading SriovVrbNodeConfig: %s", err.Error())
	}

	acceleratorConfigContext := pm.prepareAcceleratorConfigContext(nodeConfig, matchingClusterConfigs)
	if acceleratorConfigContext == nil {
		return nil, fmt.Errorf("error occurred when preparing acceleratorConfig: %s", err.Error())
	}
	return &NodeConfigurationCtx{*nodeConfig, acceleratorConfigContext}, nil
}

// Use orderedmap to save SriovFecCluster configurations
func (pm *clusterConfigMatcher) prepareAcceleratorConfigContext(nodeConfig *vrbv1.SriovVrbNodeConfig, configs []vrbv1.SriovVrbClusterConfig) *orderedmap.OrderedMap[string, vrbv1.SriovVrbClusterConfig] {
	acceleratorConfigContext := orderedmap.NewOrderedMap[string, vrbv1.SriovVrbClusterConfig]()
	for _, current := range configs {
		for _, accelerator := range nodeConfig.Status.Inventory.SriovAccelerators {
			if current.Spec.AcceleratorSelector.Matches(accelerator) {

				if _, ok := acceleratorConfigContext.Get(accelerator.PCIAddress); !ok {
					acceleratorConfigContext.Set(accelerator.PCIAddress, current)
					continue
				}

				previous, _ := acceleratorConfigContext.Get(accelerator.PCIAddress)
				switch {
				case current.Spec.Priority > previous.Spec.Priority: //override with higher prioritized config
					acceleratorConfigContext.Set(accelerator.PCIAddress, current)
				case current.Spec.Priority == previous.Spec.Priority: //multiple configs with same priority; drop older one
					//TODO: Update Timestamp would be better than CreationTime
					if current.CreationTimestamp.After(previous.CreationTimestamp.Time) {
						pm.log.WithFields(logrus.Fields{
							"Node":                  nodeConfig.Name,
							"SriovVrbClusterConfig": previous.Name,
							"Priority":              previous.Spec.Priority,
							"CreationTimestamp":     previous.CreationTimestamp.String(),
						}).Info("Dropping older ClusterConfig")

						acceleratorConfigContext.Set(accelerator.PCIAddress, current)
					}

				case current.Spec.Priority < previous.Spec.Priority: //drop current with lower priority
					pm.log.WithFields(logrus.Fields{
						"node":                  nodeConfig.Name,
						"SriovVrbClusterConfig": current.Name,
						"priority":              current.Spec.Priority,
					}).Info("Dropping low prioritized ClusterConfig")
				}
			}
		}
	}
	return acceleratorConfigContext
}

func matchConfigsForNode(node *corev1.Node, allConfigs []vrbv1.SriovVrbClusterConfig) (nodeConfigs []vrbv1.SriovVrbClusterConfig) {
	nodeLabels := labels.Set(node.Labels)
	for _, config := range allConfigs {
		nodeSelector := labels.Set(config.Spec.NodeSelector)
		if nodeSelector.AsSelector().Matches(nodeLabels) {
			nodeConfigs = append(nodeConfigs, config)
		}
	}

	// Sort existing SriovFecCluster configurations based on CreationTimestamp to keep the order
	sort.Slice(nodeConfigs, func(i, j int) bool {
		return allConfigs[i].CreationTimestamp.Before(&allConfigs[j].CreationTimestamp)
	})

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
