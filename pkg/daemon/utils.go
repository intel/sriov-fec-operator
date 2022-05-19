// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Intel Corporation

package daemon

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
)

type resourceNamePredicate struct {
	predicate.Funcs
	requiredName string
	log          *logrus.Logger
}

func (r resourceNamePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectNew.GetName() != r.requiredName {
		r.log.WithField("expected name", r.requiredName).Info("CR intended for another node - ignoring")
		return false
	}
	return true
}

func (r resourceNamePredicate) Create(e event.CreateEvent) bool {
	if e.Object.GetName() != r.requiredName {
		r.log.WithField("expected name", r.requiredName).Info("CR intended for another node - ignoring")
		return false
	}
	return true
}

//returns result indicating necessity of re-queuing Reconcile after configured resyncPeriod
func requeueLater() (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: resyncPeriod}, nil
}

//returns result indicating necessity of re-queuing Reconcile(...) immediately; non-nil err will be logged by controller
func requeueNowWithError(e error) (reconcile.Result, error) {
	return reconcile.Result{Requeue: true}, e
}

//returns result indicating necessity of re-queuing Reconcile(...):
//immediately - in case when given err is non-nil;
//on configured schedule, when err is nil
func requeueLaterOrNowIfError(e error) (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: resyncPeriod}, e
}

var procCmdlineFilePath = "/host/proc/cmdline"
var kernelParams = []string{"intel_iommu=on", "iommu=pt"}

// anyKernelParamsMissing checks current kernel cmdline
// returns true if /proc/cmdline requires update
func verifyKernelConfiguration() error {
	cmdlineBytes, err := ioutil.ReadFile(procCmdlineFilePath)
	if err != nil {
		return errors.WithMessagef(err, "failed to read file contents: path: %s", procCmdlineFilePath)
	}
	cmdline := string(cmdlineBytes)
	for _, param := range kernelParams {
		if !strings.Contains(cmdline, param) {
			return fmt.Errorf("missing kernel param(%s)", param)
		}
	}
	return nil
}
