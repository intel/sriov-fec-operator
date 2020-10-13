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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fpgav1 "github.com/otcshare/openshift-operator/N3000/api/v1"
)

const (
	DEFAULT_N3000_CONFIG_NAME = "n3000"
)

var log = ctrl.Log.WithName("N3000ClusterController")
var namespace = os.Getenv("NAMESPACE")

type state struct {
	name string
	obj  []runtime.Object
}

func (r *N3000ClusterReconciler) updateStatus(n3000cluster *fpgav1.N3000Cluster,
	status fpgav1.N3000ClusterSyncStatus, reason string) {
	n3000cluster.Status.SyncStatus = status
	n3000cluster.Status.LastSyncError = reason
	if err := r.Status().Update(context.Background(), n3000cluster, &client.UpdateOptions{}); err != nil {
		log.Error(err, "failed to update cluster config's status")
	}
}

func (s *state) loadAssets(path string) error {
	log.Info("Loading assets from:", "path:", path)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	d := scheme.Codecs.UniversalDeserializer().Decode
	rx := regexp.MustCompile("\n-{3}")
	objDefs := rx.Split(string(data), -1)

	for _, i := range objDefs {
		o, _, _ := d([]byte(i), nil, nil)
		log.Info("Found", "kind:", o.GetObjectKind())
		s.obj = append(s.obj, o)
	}
	return nil
}

func (s *state) execute(c *N3000ClusterController) (bool, error) {
	log.Info("Executing", "state:", s.name)
	for _, stateObj := range s.obj {
		log.Info("Creating:", "kind:", stateObj.GetObjectKind())
		metaObj, ok := stateObj.(metav1.Object)
		if !ok {
			c.r.updateStatus(c.apiInstance, fpgav1.FailedSync, "failed to get meta object")
			return false, errors.New("stateObj assertion error")
		}
		err := controllerutil.SetControllerReference(c.apiInstance, metaObj, c.r.Scheme)
		if err != nil {
			c.r.updateStatus(c.apiInstance, fpgav1.FailedSync, "failed to get controller reference")
			return false, err
		}

		switch t := stateObj.(type) {
		case *fpgav1.N3000Node:
			err = createOrUpdateNodeSpec(c, t)
		default:
			err = createOrSkip(c, stateObj)
		}

		if err != nil {
			return false, err
		}

		if !isReady(c.ctx, stateObj, c.r) {
			return false, nil
		}
	}
	return true, nil
}

func createOrSkip(c *N3000ClusterController, o runtime.Object) error {
	err := c.r.Client.Create(c.ctx, o.DeepCopyObject())
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			log.Info("Object already exists...skipping")
		} else {
			c.r.updateStatus(c.apiInstance, fpgav1.FailedSync, "failed to create default spec")
			return err
		}
	}
	return nil
}

func createOrUpdateNodeSpec(c *N3000ClusterController, n *fpgav1.N3000Node) error {
	currObj := &fpgav1.N3000Node{}
	err := c.r.Client.Get(c.ctx, types.NamespacedName{Namespace: namespace, Name: n.Name}, currObj)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			err = c.r.Client.Create(c.ctx, n)
			if err != nil {
				c.r.updateStatus(c.apiInstance, fpgav1.FailedSync, "failed to create node spec")
				return err
			}
		} else {
			c.r.updateStatus(c.apiInstance, fpgav1.FailedSync, "failed to get node spec")
			return err
		}
	}
	log.Info("Object already exists...updating .Spec")
	currObj.Spec = n.Spec
	return c.r.Client.Update(c.ctx, currObj)
}

func isPodReady(ctx context.Context, r *N3000ClusterReconciler, clo []client.ListOption, phase corev1.PodPhase) bool {
	pl := &corev1.PodList{}
	if err := r.Client.List(ctx, pl, clo...); err != nil {
		log.Info("client.List", "error:", err)
		return false
	}
	if len(pl.Items) == 0 {
		log.Info("No Pod found for:", "client.ListOption", clo)
		return false
	}
	for _, p := range pl.Items {
		if p.Status.Phase != phase {
			log.Info("Pod not ready on", "Node:", p.Status.NominatedNodeName, "Message:", p.Status.Message)
			return false
		}
	}
	return true
}

func isReady(ctx context.Context, o runtime.Object, r *N3000ClusterReconciler) bool {
	k := o.GetObjectKind().GroupVersionKind().Kind
	if k == "DaemonSet" {
		log.Info("Is daemonset ready")
		typedObj := o.(*appsv1.DaemonSet)
		dsl := &appsv1.DaemonSetList{}
		clo := []client.ListOption{
			client.InNamespace(typedObj.GetNamespace()),
			client.MatchingLabels{"app": typedObj.GetName()},
		}
		if err := r.Client.List(ctx, dsl, clo...); err != nil {
			log.Info("client.List error:", err)
			return false
		}
		if len(dsl.Items) == 0 {
			log.Info("No DaemonSet found:", typedObj.GetName())
			return false
		}
		if nu := dsl.Items[0].Status.NumberUnavailable; nu != 0 {
			log.Info("NumberUnavailable", nu)
			return false
		}
		return isPodReady(ctx, r, clo, "Running")
	} else if k == "Pod" {
		log.Info("Is pod ready")
		typedObj := o.(*corev1.Pod)
		clo := []client.ListOption{
			client.InNamespace(typedObj.GetNamespace()),
			client.MatchingLabels{"app": typedObj.GetName()},
		}
		return isPodReady(ctx, r, clo, "Succeeded")
	}
	log.Info("Is ready true for", "kind:", k)
	return true
}

func newStateFromFile(k8sDefPath string) (*state, error) {
	ns := new(state)
	path := filepath.Clean(k8sDefPath)

	ns.name = strings.Split(filepath.Base(path), ".")[0]
	err := ns.loadAssets(path)
	return ns, err
}

func (c *N3000ClusterController) newStateFromClusterResource() (*state, error) {
	clRes := &fpgav1.N3000Cluster{}
	err := c.r.Client.Get(c.ctx, types.NamespacedName{Name: DEFAULT_N3000_CONFIG_NAME, Namespace: namespace}, clRes)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Error(err, "fpgav1.N3000Cluster resource not found")
		}
		return nil, err
	}

	nodes := &corev1.NodeList{}
	// TODO add NFD label for fpga
	err = c.r.Client.List(c.ctx, nodes, &client.MatchingLabels{"node-role.kubernetes.io/worker": ""})
	if err != nil {
		log.Error(err, "Unable to list the nodes")
		return nil, err
	}

	ns := new(state)
	ns.name = "N3000Node"
	for _, res := range clRes.Spec.Nodes {
		for _, node := range nodes.Items {
			if res.NodeName == node.Name {
				nodeRes := &fpgav1.N3000Node{}
				nodeRes.ObjectMeta = metav1.ObjectMeta{Name: "n3000node-" + res.NodeName,
					Namespace: namespace,
				}
				nodeRes.Spec.FPGA = res.FPGA
				nodeRes.Spec.Fortville = res.Fortville
				ns.obj = append(ns.obj, nodeRes)
				break
			}
		}
	}
	return ns, nil
}

type N3000ClusterController struct {
	ctx         context.Context
	apiInstance *fpgav1.N3000Cluster
	r           *N3000ClusterReconciler
	states      []*state
}

func newController(ctx context.Context, api *fpgav1.N3000Cluster, r *N3000ClusterReconciler) (*N3000ClusterController, error) {
	nc := new(N3000ClusterController)
	nc.apiInstance = api
	nc.r = r
	nc.ctx = ctx
	err := nc.loadStates()
	return nc, err
}

func (c *N3000ClusterController) loadStates() error {
	// TODO: add all state yamls
	// if err := c.addState(newStateFromFile("./test.yaml")); err != nil {
	// 	return err
	// }
	if err := c.addState(c.newStateFromClusterResource()); err != nil {
		return err
	}
	return nil
}

func (c *N3000ClusterController) addState(s *state, err error) error {
	if err != nil {
		return err
	}
	c.states = append(c.states, s)
	return nil
}

func (c *N3000ClusterController) doAll() (ctrl.Result, error) {
	for _, s := range c.states {
		ready, err := s.execute(c)
		if err != nil {
			log.Info("Error when executing state", "err", err.Error())
			if c.apiInstance.Status.SyncStatus != fpgav1.InprogressSync {
				c.r.updateStatus(c.apiInstance, fpgav1.InprogressSync, "")
			}
			return ctrl.Result{RequeueAfter: time.Second * 10}, err
		}
		if !ready {
			log.Info("State not ready - requeue")
			if c.apiInstance.Status.SyncStatus != fpgav1.InprogressSync {
				c.r.updateStatus(c.apiInstance, fpgav1.InprogressSync, "")
			}
			return ctrl.Result{RequeueAfter: time.Second * 10}, nil
		}
	}
	c.r.updateStatus(c.apiInstance, fpgav1.SucceededSync, "")
	return ctrl.Result{}, nil
}

// N3000ClusterReconciler reconciles a N3000Cluster object
type N3000ClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=fpga.intel.com,resources=n3000clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=fpga.intel.com,resources=n3000clusters/status,verbs=get;update;patch

func (r *N3000ClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	log.Info("Reconciling N3000ClusterReconciler")

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
		log.Info("received ClusterConfig, but it not an expected one - it'll be ignored",
			"expectedNamespace", namespace, "expectedName", DEFAULT_N3000_CONFIG_NAME)

		r.updateStatus(clusterConfig, fpgav1.FailedSync, fmt.Sprintf(
			"Only N3000Cluster with name '%s' and namespace '%s' are handled",
			DEFAULT_N3000_CONFIG_NAME, namespace))

		return ctrl.Result{}, nil
	}

	c, err := newController(ctx, clusterConfig, r)
	if err != nil {
		log.Error(err, "newController failed")
		r.updateStatus(clusterConfig, fpgav1.FailedSync, "failed in new controller - check logs")
		return ctrl.Result{}, err
	}

	res, err := c.doAll()
	if err != nil {
		log.Error(err, "doAll failed")
		return ctrl.Result{}, err
	}

	return res, nil
}

func (r *N3000ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fpgav1.N3000Cluster{}).
		Complete(r)
}
