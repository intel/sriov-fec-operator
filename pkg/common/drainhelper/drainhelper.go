// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2025 Intel Corporation

package drainhelper

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"strconv"
	"strings"
	"time"

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
	LeaseDurationEnvVarName      = "LEASE_DURATION_SECONDS"
	LeaseDurationDefault         = int64(137)
)

// logWriter is a wrapper around logrus log.Info() to allow drain.Helper logging
type logWriter struct {
	log *logrus.Logger
}

func (w logWriter) Write(p []byte) (n int, err error) {
	w.log.Info(strings.TrimSuffix(string(p), "\n"))
	return len(p), nil
}

type DrainHelper struct {
	log       *logrus.Logger
	clientSet *clientset.Clientset
	nodeName  string

	drainer              *drain.Helper
	leaseLock            *resourcelock.LeaseLock
	leaderElectionConfig leaderelection.LeaderElectionConfig
}

func NewDrainHelper(log *logrus.Logger, cs *clientset.Clientset, nodeName, namespace string, isSingleNodeCluster bool) *DrainHelper {
	drainTimeout := drainHelperTimeoutDefault
	drainTimeoutStr := os.Getenv(drainHelperTimeoutEnvVarName)
	if drainTimeoutStr != "" {
		val, err := strconv.ParseInt(drainTimeoutStr, 10, 64)
		if err != nil {
			log.WithError(err).WithField("variable", drainHelperTimeoutEnvVarName).
				Error("failed to parse env variable to int64 - using default value")
		} else {
			drainTimeout = val
		}
	}
	log.WithField("timeout seconds", drainTimeout).Info("drain settings")

	leaseDur := LeaseDurationDefault
	leaseDurStr := os.Getenv(LeaseDurationEnvVarName)
	if leaseDurStr != "" {
		val, err := strconv.ParseInt(leaseDurStr, 10, 64)
		if err != nil {
			log.WithError(err).WithField("variable", LeaseDurationEnvVarName).Error("failed to parse env variable to int64 - using default value")
		} else {
			leaseDur = val
		}
	}
	log.WithField("duration seconds", leaseDur).Info("lease settings")

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
			Ctx:                 context.Background(),
			Client:              cs,
			Force:               true,
			IgnoreAllDaemonSets: true,
			DeleteEmptyDirData:  true,
			GracePeriodSeconds:  -1,
			Timeout:             time.Duration(drainTimeout) * time.Second,
			OnPodDeletedOrEvicted: func(pod *corev1.Pod, usingEviction bool) {
				act := "Deleted"
				if usingEviction {
					act = "Evicted"
				}
				log.WithField("action", act).WithField("pod", fmt.Sprintf("%s/%s", pod.Name, pod.Namespace)).
					Info("pod evicted or deleted")
			},
			Out:    logWriter{log},
			ErrOut: logWriter{log},
		},

		leaseLock:            lock,
		leaderElectionConfig: CustomizedLeaderElectionConfig(lock, leaseDur, isSingleNodeCluster),
	}
}

// More details about values are available here:
// https://github.com/openshift/library-go/commit/2612981f3019479805ac8448b997266fc07a236a#diff-61dd95c7fd45fa18038e825205fbfab8a803f1970068157608b6b1e9e6c27248R127-R150
func CustomizedLeaderElectionConfig(lock *resourcelock.LeaseLock, leaseDur int64, isSingleNodeCluster bool) leaderelection.LeaderElectionConfig {
	lec := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   time.Duration(leaseDur) * time.Second,
		RenewDeadline:   107 * time.Second,
		RetryPeriod:     26 * time.Second,
	}
	if isSingleNodeCluster {
		lec.LeaseDuration = 270 * time.Second
		lec.RenewDeadline = 240 * time.Second
		lec.RetryPeriod = 60 * time.Second
	}
	return lec
}

// Run joins leader election and drains(only if drain is set) the node if becomes a leader.
//
// f is a function that takes a context and returns a bool.
// It should return true if uncordon should be performed(Only applicable if drain is set to true).
// If `f` returns false, the uncordon does not take place. This is useful in 2-step scenario like sriov-fec-daemon where
// reboot must be performed without loosing the leadership and without the uncordon.
func (dh *DrainHelper) Run(f func(context.Context) bool, drain bool) error {
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
			dh.log.WithError(err).Error("failed to get the LeaderElectionRecord")
			return
		}
		leaderElectionRecord.HolderIdentity = ""
		if err := dh.leaseLock.Update(context.Background(), *leaderElectionRecord); err != nil {
			dh.log.WithError(err).Error("failed to update the LeaderElectionRecord")
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var innerErr error

	lec := dh.leaderElectionConfig
	lec.Callbacks = leaderelection.LeaderCallbacks{
		OnStartedLeading: func(ctx context.Context) {
			defer func() {
				dh.log.Info("cancelling the context to finish the leadership")
				cancel()
			}()

			dh.log.Info("started leading")

			uncordon := func() {
				// always try to uncordon the node
				// e.g. when cordoning succeeds, but draining fails
				dh.log.Info("uncordoning node")
				if err := dh.uncordon(ctx); err != nil {
					dh.log.WithError(err).Error("uncordon failed")
					innerErr = err
				}
			}

			if drain {
				dh.log.Info("cordoning & draining node")
				if err := dh.cordonAndDrain(ctx); err != nil {
					dh.log.WithError(err).Error("cordonAndDrain failed")
					innerErr = err
					uncordon()
					return
				}
			}

			dh.log.Info("worker function - start")
			performUncordon := f(ctx)
			dh.log.WithField("performUncordon", performUncordon).Info("worker function - end")
			if drain && performUncordon {
				uncordon()
			}
		},
		OnStoppedLeading: func() {
			dh.log.Info("stopped leading")
		},
		OnNewLeader: dh.onNewLeaderFunction,
	}

	le, err := leaderelection.NewLeaderElector(lec)
	if err != nil {
		dh.log.WithError(err).Error("failed to create new leader elector")
		return err
	}

	le.Run(ctx)

	if innerErr != nil {
		dh.log.WithError(innerErr).Error("error during (un)cordon or drain actions")
	}

	return innerErr
}

func (dh *DrainHelper) onNewLeaderFunction(id string) {
	if id != dh.nodeName {
		dh.log.WithField("this", dh.nodeName).WithField("leader", id).Info("new leader elected")
	}
}

func (dh *DrainHelper) cordonAndDrain(ctx context.Context) error {
	node, nodeGetErr := dh.clientSet.CoreV1().Nodes().Get(ctx, dh.nodeName, metav1.GetOptions{})
	if nodeGetErr != nil {
		dh.log.WithError(nodeGetErr).Error("failed to get the node object")
		return nodeGetErr
	}

	var e error
	backoff := wait.Backoff{Steps: 5, Duration: 15 * time.Second, Factor: 2}
	f := func() (bool, error) {
		if err := drain.RunCordonOrUncordon(dh.drainer, node, true); err != nil {
			dh.log.WithField("nodeName", dh.nodeName).WithField("reason", err.Error()).
				Info("failed to cordon the node - retrying")
			e = err
			return false, nil
		}

		if err := drain.RunNodeDrain(dh.drainer, dh.nodeName); err != nil {
			dh.log.WithField("nodeName", dh.nodeName).WithField("reason", err.Error()).
				Info("failed to drain the node - retrying")
			e = err
			return false, nil
		}

		return true, nil
	}

	dh.log.Info("starting drain attempts")
	if err := wait.ExponentialBackoff(backoff, f); err != nil {
		if err == wait.ErrWaitTimeout {
			dh.log.WithError(e).Error("failed to drain node - timed out")
			return e
		}
		dh.log.WithError(err).Error("failed to drain node")
		return err
	}

	dh.log.Info("node drained")
	return nil
}

func (dh *DrainHelper) uncordon(ctx context.Context) error {
	node, err := dh.clientSet.CoreV1().Nodes().Get(ctx, dh.nodeName, metav1.GetOptions{})
	if err != nil {
		dh.log.WithError(err).Error("failed to get the node object")
		return err
	}

	var e error
	backoff := wait.Backoff{Steps: 5, Duration: 15 * time.Second, Factor: 2}
	f := func() (bool, error) {
		if err := drain.RunCordonOrUncordon(dh.drainer, node, false); err != nil {
			dh.log.WithField("nodeName", dh.nodeName).WithError(err).Error("failed to uncordon the node - retrying")
			e = err
			return false, nil
		}

		return true, nil
	}

	dh.log.Info("starting uncordon attempts")
	if err := wait.ExponentialBackoff(backoff, f); err != nil {
		if err == wait.ErrWaitTimeout {
			dh.log.WithError(e).Error("failed to uncordon node - timed out")
			return e
		}
		dh.log.WithError(err).Error("failed to uncordon node")
		return err
	}
	dh.log.Info("node uncordoned")

	return nil
}
