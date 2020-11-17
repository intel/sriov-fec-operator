// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package drainhelper

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/kubectl/pkg/drain"
)

const (
	drainHelperTimeoutEnvVarName = "DRAIN_TIMEOUT_SECONDS"
	drainHelperTimeoutDefault    = int64(90)
	leaseDurationEnvVarName      = "LEASE_DURATION_SECONDS"
	leaseDurationDefault         = int64(600)
)

// logWriter is a wrapper around logr's log.Info() to allow drain.Helper logging
type logWriter struct {
	log logr.Logger
}

func (w logWriter) Write(p []byte) (n int, err error) {
	w.log.Info(strings.TrimSuffix(string(p), "\n"))
	return len(p), nil
}

type DrainHelper struct {
	log       logr.Logger
	clientSet *clientset.Clientset
	nodeName  string

	drainer              *drain.Helper
	leaseLock            *resourcelock.LeaseLock
	leaderElectionConfig leaderelection.LeaderElectionConfig
}

func NewDrainHelper(l logr.Logger, cs *clientset.Clientset, nodeName, namespace string) *DrainHelper {
	log := l.WithName("drainhelper")

	drainTimeout := drainHelperTimeoutDefault
	drainTimeoutStr := os.Getenv(drainHelperTimeoutEnvVarName)
	if drainTimeoutStr != "" {
		val, err := strconv.ParseInt(drainTimeoutStr, 10, 64)
		if err != nil {
			log.Error(err, "failed to parse env variable to int64 - using default value",
				"variable", drainHelperTimeoutEnvVarName)
		} else {
			drainTimeout = val
		}
	}
	log.Info("drain settings", "timeout seconds", drainTimeout)

	leaseDur := leaseDurationDefault
	leaseDurStr := os.Getenv(leaseDurationEnvVarName)
	if leaseDurStr != "" {
		val, err := strconv.ParseInt(leaseDurStr, 10, 64)
		if err != nil {
			log.Error(err, "failed to parse env variable to int64 - using default value",
				"variable", leaseDurationEnvVarName)
		} else {
			leaseDur = val
		}
	}
	log.Info("lease settings", "duration seconds", leaseDur)

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      "n3000-daemon-lease",
			Namespace: namespace,
		},
		Client: cs.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: nodeName,
		},
	}

	return &DrainHelper{
		log:       log,
		clientSet: cs,
		nodeName:  nodeName,

		drainer: &drain.Helper{
			Client:              cs,
			Force:               true,
			IgnoreAllDaemonSets: true,
			DeleteLocalData:     true,
			GracePeriodSeconds:  -1,
			Timeout:             time.Duration(drainTimeout) * time.Second,
			OnPodDeletedOrEvicted: func(pod *corev1.Pod, usingEviction bool) {
				act := "Deleted"
				if usingEviction {
					act = "Evicted"
				}
				log.V(2).Info("pod evicted or deleted",
					"action", act, "pod", fmt.Sprintf("%s/%s", pod.Name, pod.Namespace))
			},
			Out:    logWriter{log},
			ErrOut: logWriter{log},
		},

		leaseLock: lock,
		leaderElectionConfig: leaderelection.LeaderElectionConfig{
			Lock:            lock,
			ReleaseOnCancel: true,
			LeaseDuration:   time.Duration(leaseDur) * time.Second,
			RenewDeadline:   15 * time.Second,
			RetryPeriod:     5 * time.Second,
		},
	}
}

// Run joins leader election and drains the node if becomes a leader.
//
// f is a function that takes a context and returns a bool. It should return true if uncordon should be performed.
// If `f` returns false, the uncordon does not take place. This is useful in 2-step scenario like sriov-fec-daemon where
// reboot must be performed without loosing the leadership and without the uncordon.
func (dh *DrainHelper) Run(f func(context.Context) bool) error {
	log := dh.log.WithName("Run()")

	defer func() {
		// Following mitigation is needed because of the bug in the leader election's release functionality
		// Release fails because the input (leader election record) is created incomplete (missing fields):
		// Failed to release lock: Lease.coordination.k8s.io "n3000-daemon-lease" is invalid:
		// ... spec.leaseDurationSeconds: Invalid value: 0: must be greater than 0
		// When the leader election finishes (Run() ends), we need to clean up the Lease manually.
		// See: https://github.com/kubernetes/kubernetes/pull/80954
		// This however is not critical - if the leader will not refresh the lease,
		// another node will take it after some time.

		dh.log.Info("releasing the lock (bug mitigation)")

		leaderElectionRecord, _, err := dh.leaseLock.Get(context.Background())
		if err != nil {
			log.Error(err, "failed to get the LeaderElectionRecord")
			return
		}
		leaderElectionRecord.HolderIdentity = ""
		if err := dh.leaseLock.Update(context.Background(), *leaderElectionRecord); err != nil {
			log.Error(err, "failed to update the LeaderElectionRecord")
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var innerErr error

	lec := dh.leaderElectionConfig
	lec.Callbacks = leaderelection.LeaderCallbacks{
		OnStartedLeading: func(ctx context.Context) {
			log.Info("started leading")

			uncordonAndFreeLeadership := func() {
				// always try to uncordon the node
				// e.g. when cordoning succeeds, but draining fails
				log.V(4).Info("uncordoning node")
				if err := dh.uncordon(ctx); err != nil {
					log.Error(err, "uncordon failed")
					innerErr = err
				}

				log.V(4).Info("cancelling the context to finish the leadership")
				cancel()
			}

			log.V(4).Info("cordoning & draining node")
			if err := dh.cordonAndDrain(ctx); err != nil {
				log.Error(err, "cordonAndDrain failed")
				innerErr = err
				uncordonAndFreeLeadership()
				return
			}

			log.Info("worker function - start")
			performUncordon := f(ctx)
			log.Info("worker function - end", "performUncordon", performUncordon)
			if performUncordon {
				uncordonAndFreeLeadership()
			}
		},
		OnStoppedLeading: func() {
			log.V(4).Info("stopped leading")
		},
		OnNewLeader: func(id string) {
			if id != dh.nodeName {
				log.V(2).Info("new leader elected", "leader", id, "this", dh.nodeName)
			}
		},
	}

	le, err := leaderelection.NewLeaderElector(lec)
	if err != nil {
		log.Error(err, "failed to create new leader elector")
		return err
	}

	le.Run(ctx)

	if innerErr != nil {
		log.Error(innerErr, "error during (un)cordon or drain actions")
	}

	return innerErr
}

func (dh *DrainHelper) cordonAndDrain(ctx context.Context) error {
	log := dh.log.WithName("cordonAndDrain()")

	node, nodeGetErr := dh.clientSet.CoreV1().Nodes().Get(ctx, dh.nodeName, metav1.GetOptions{})
	if nodeGetErr != nil {
		log.Error(nodeGetErr, "failed to get the node object")
		return nodeGetErr
	}

	var e error
	backoff := wait.Backoff{Steps: 5, Duration: 15 * time.Second, Factor: 2}
	f := func() (bool, error) {
		if err := drain.RunCordonOrUncordon(dh.drainer, node, true); err != nil {
			log.Info("failed to cordon the node - retrying", "nodeName", dh.nodeName, "reason", err.Error())
			e = err
			return false, nil
		}

		if err := drain.RunNodeDrain(dh.drainer, dh.nodeName); err != nil {
			log.Info("failed to drain the node - retrying", "nodeName", dh.nodeName, "reason", err.Error())
			e = err
			return false, nil
		}

		return true, nil
	}

	log.Info("starting drain attempts")
	if err := wait.ExponentialBackoff(backoff, f); err != nil {
		if err == wait.ErrWaitTimeout {
			log.Error(e, "failed to drain node - timed out")
			return e
		}
		log.Error(err, "failed to drain node")
		return err
	}

	log.Info("node drained")
	return nil
}

func (dh *DrainHelper) uncordon(ctx context.Context) error {
	log := dh.log.WithName("uncordon()")

	node, err := dh.clientSet.CoreV1().Nodes().Get(ctx, dh.nodeName, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "failed to get the node object")
		return err
	}

	var e error
	backoff := wait.Backoff{Steps: 5, Duration: 15 * time.Second, Factor: 2}
	f := func() (bool, error) {
		if err := drain.RunCordonOrUncordon(dh.drainer, node, false); err != nil {
			log.Error(err, "failed to uncordon the node - retrying", "nodeName", dh.nodeName)
			e = err
			return false, nil
		}

		return true, nil
	}

	log.Info("starting uncordon attempts")
	if err := wait.ExponentialBackoff(backoff, f); err != nil {
		if err == wait.ErrWaitTimeout {
			log.Error(e, "failed to uncordon node - timed out")
			return e
		}
		log.Error(err, "failed to uncordon node")
		return err
	}

	return nil
}
