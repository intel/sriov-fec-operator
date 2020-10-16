// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"context"

	"github.com/go-logr/logr"
	fpgav1 "github.com/otcshare/openshift-operator/N3000/api/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
)

type DaemonController struct {
	Log   logr.Logger
	d     *Daemon
	fm    FortvilleManager
	fpgaM FPGAManager
}

func newDaemonController(d *Daemon) *DaemonController {
	dc := &DaemonController{
		Log: d.Log,
		d:   d,
	}
	//TODO: Need to refactor GetEventBuffer and inject Event to Managers
	dc.fm.d = d

	log := dc.Log.WithName("newDaemonController")
	err := dc.updateNodeStatus()
	if err != nil {
		log.Error(err, "Unable to update N3000NodeStatus")
	}
	return dc
}

func (dc *DaemonController) updateNodeStatus() error {
	log := dc.Log.WithName("updateNodeStatus")
	n, err := dc.getN3000Node()
	if err != nil {
		if apierr.IsNotFound(err) {
			log.V(2).Info("N3000Node resource not found - creating new one with basic status ")
			ns, err := dc.createBasicNodeStatus()
			if err != nil {
				return err
			}

			n := fpgav1.N3000Node{}
			n.Status = *ns
			n.Name = "n3000node-" + dc.d.nodeName
			n.Namespace = namespace

			o := &unstructured.Unstructured{}
			err = scheme.Scheme.Convert(&n, o, nil)
			_, err = dc.d.client.Resource(nodeGVR).Namespace(namespace).
				Create(context.TODO(), o, metav1.CreateOptions{})
			if err != nil {
				log.Error(err, "Error when creating N3000Node resource")
				return err
			}
			return nil
		}
		return err
	}

	log.V(2).Info("N3000Node resource found - updating basic status")
	ns, err := dc.createBasicNodeStatus()
	if err != nil {
		return err
	}

	log.V(2).Info("N3000Node resource found - updating status with nvmupdate inventory data")
	i, err := dc.fm.getInventory()
	if err != nil {
		log.Error(err, "Unable to get inventory...using basic status only")
	} else {
		dc.fm.processInventory(&i, ns) // fill ns with data from inventory
	}

	n.Status = *ns

	o := &unstructured.Unstructured{}
	err = scheme.Scheme.Convert(n, o, nil)
	_, err = dc.d.client.Resource(nodeGVR).Namespace(namespace).
		UpdateStatus(context.TODO(), o, metav1.UpdateOptions{})
	if err != nil {
		log.Error(err, "Error when updating N3000NodeStatus resource")
		return err
	}
	return nil
}

func (dc *DaemonController) start() {

}

func (dc *DaemonController) createBasicNodeStatus() (*fpgav1.N3000NodeStatus, error) {
	ns := &fpgav1.N3000NodeStatus{}
	fortvilleStatus, err := dc.fm.getNetworkDevices()
	if err != nil {
		return nil, err
	}
	ns.Fortville = fortvilleStatus

	fpgaStatus, err := dc.fpgaM.getFPGAStatus()
	if err != nil {
		return nil, err
	}
	ns.FPGA = fpgaStatus
	return ns, nil
}

func (dc *DaemonController) getN3000NodeStatus() (*fpgav1.N3000NodeStatus, error) {
	n, err := dc.getN3000Node()
	if err != nil {
		return nil, err
	}

	return &n.Status, nil
}

func (dc *DaemonController) getN3000Node() (*fpgav1.N3000Node, error) {
	log := dc.Log.WithName("getN3000Node")
	result, err := dc.d.client.Resource(nodeGVR).Namespace(namespace).Get(context.TODO(), "n3000node-"+dc.d.nodeName, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "Error when getting N3000Node resource")
		return nil, err
	}

	n := &fpgav1.N3000Node{}
	err = scheme.Scheme.Convert(result, n, nil)
	if err != nil {
		log.Error(err, "Unable to convert Unstructured to N3000Node")
		return nil, err
	}
	return n, nil
}
