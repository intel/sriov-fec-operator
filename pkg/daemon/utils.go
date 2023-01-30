// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Intel Corporation

package daemon

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
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

// returns result indicating necessity of re-queuing Reconcile after configured resyncPeriod
func requeueLater() (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: resyncPeriod}, nil
}

// returns result indicating necessity of re-queuing Reconcile(...) immediately; non-nil err will be logged by controller
func requeueNowWithError(e error) (reconcile.Result, error) {
	return reconcile.Result{Requeue: true}, e
}

// returns result indicating necessity of re-queuing Reconcile(...):
// immediately - in case when given err is non-nil;
// on configured schedule, when err is nil
func requeueLaterOrNowIfError(e error) (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: resyncPeriod}, e
}

// operator is unable to write to sysfs files if device is currently in use
// this function is supposed to either write successfully to file or return timeout error
func writeFileWithTimeout(filename, data string) error {
	done := make(chan struct{})
	var err error

	go func() {
		err = os.WriteFile(filename, []byte(data), os.ModeAppend)
		done <- struct{}{}
	}()

	select {
	case <-done:
		return err
	case <-time.After(5 * time.Second):
		return fmt.Errorf("failed to write to sysfs file. Usually it means that device is in use by other process")
	}

}
