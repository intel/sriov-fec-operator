// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Intel Corporation

package daemon

import (
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
