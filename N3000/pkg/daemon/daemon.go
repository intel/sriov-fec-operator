// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"os"
	"sync"
	"time"

	//"github.com/go-logr/logr"
	"github.com/go-logr/logr"
	fpgav1 "github.com/otcshare/openshift-operator/N3000/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type Daemon struct {
	Log            logr.Logger
	nodeName       string
	namespace      string
	kubeconfig     string
	n3000node      *fpgav1.N3000Node
	client         dynamic.Interface
	stopCh         <-chan struct{}
	eventBuffer    NodeEvent
	eventBufferMtx sync.Mutex
}

func NewDaemon(sCh <-chan struct{}, log logr.Logger) *Daemon {
	d := &Daemon{
		Log:       log,
		stopCh:    sCh,
		nodeName:  nodeName,
		namespace: namespace,
	}

	d.initDynamicClient()
	fpgav1.AddToScheme(scheme.Scheme)
	return d
}

var (
	nodeGVR = schema.GroupVersionResource{
		Group:    "fpga.intel.com",
		Version:  "v1",
		Resource: "n3000nodes",
	}
	informerResync = time.Second * 10
	namespace      = os.Getenv("NAMESPACE")
	nodeName       = os.Getenv("NODENAME")
)

type NodeEvent struct {
	eventType NodeEventType
	oldObj    *fpgav1.N3000Node
	newObj    *fpgav1.N3000Node
}
type NodeEventType string

const (
	Add    NodeEventType = "add"
	Update NodeEventType = "update"
	Delete NodeEventType = "delete"
)

func (d *Daemon) Start() error {
	log := d.Log.WithName("Start")
	go d.startDynamicInformer()

	dc := newDaemonController(d)
	dc.start()

	for {
		select {
		case <-d.stopCh:
			log.Info("Stopping daemon")
			return nil
		}
	}

}

func (d *Daemon) initDynamicClient() {
	log := d.Log.WithName("initDynamicClient")
	log.Info("Initializing k8s client")
	var config *rest.Config
	var err error

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig != "" {
		log.Info("Using KUBECONFIG")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		panic(err)
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	d.client = client
}

func (d *Daemon) startDynamicInformer() {
	informerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(d.client,
		informerResync,
		d.namespace,
		nil)
	informer := informerFactory.ForResource(nodeGVR)
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    d.nodeAddFunc,
		UpdateFunc: d.nodeUpdateFunc,
		DeleteFunc: d.nodeDeleteFunc,
	})
	informer.Informer().Run(d.stopCh)
}

func (d *Daemon) nodeAddFunc(obj interface{}) {
	log := d.Log.WithName("nodeAddFunc")
	log.V(2).Info("N3000Nodes resource added", "node", d.nodeName)
	d.eventBufferMtx.Lock()
	defer d.eventBufferMtx.Unlock()

	var n fpgav1.N3000Node
	err := scheme.Scheme.Convert(obj, &n, nil)
	if err != nil {
		log.Error(err, "Unable to convert Object to N3000Node...ignoring")
		return
	}
	d.eventBuffer = NodeEvent{
		eventType: Add,
		oldObj:    nil,
		newObj:    &n,
	}
	o := &unstructured.Unstructured{}
	err = scheme.Scheme.Convert(&n, o, nil)
	if err != nil {
		log.Error(err, "Unable to convert N3000Node to obj...ignoring")
		return
	}
}

func (d *Daemon) nodeUpdateFunc(old, obj interface{}) {
	log := d.Log.WithName("nodeUpdateFunc")
	log.V(2).Info("N3000Nodes resource updated", "node", d.nodeName)
	d.eventBufferMtx.Lock()
	defer d.eventBufferMtx.Unlock()
	var no fpgav1.N3000Node
	err := scheme.Scheme.Convert(obj, &no, nil)
	if err != nil {
		log.Error(err, "Unable to convert old Object to N3000Node...ignoring")
		return
	}
	var nn fpgav1.N3000Node
	err = scheme.Scheme.Convert(obj, &nn, nil)
	if err != nil {
		log.Error(err, "Unable to convert new Object to N3000Node...ignoring")
		return
	}

	d.eventBuffer = NodeEvent{
		eventType: Update,
		oldObj:    &no,
		newObj:    &nn,
	}
}

func (d *Daemon) nodeDeleteFunc(obj interface{}) {
	log := d.Log.WithName("nodeDeleteFunc")
	log.V(2).Info("N3000Nodes resource deleted", "node", d.nodeName)
	d.eventBufferMtx.Lock()
	defer d.eventBufferMtx.Unlock()
	var n fpgav1.N3000Node
	err := scheme.Scheme.Convert(obj, &n, nil)
	if err != nil {
		log.Error(err, "Unable to convert Object to N3000Node...ignoring")
		return
	}
	d.eventBuffer = NodeEvent{
		eventType: Delete,
		oldObj:    nil,
		newObj:    &n,
	}
}

func (d *Daemon) GetEventBuffer() NodeEvent {
	d.eventBufferMtx.Lock()
	defer d.eventBufferMtx.Unlock()
	eb := d.eventBuffer
	return eb
}
