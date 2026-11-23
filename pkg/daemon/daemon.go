// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2026 Intel Corporation

package daemon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	ctrl "sigs.k8s.io/controller-runtime"
)

type ConfigurationConditionReason string

const (
	ConditionConfigured       string                       = "Configured"
	ConfigurationInProgress   ConfigurationConditionReason = "InProgress"
	ConfigurationFailed       ConfigurationConditionReason = "Failed"
	ConfigurationNotRequested ConfigurationConditionReason = "NotRequested"
	ConfigurationSucceeded    ConfigurationConditionReason = "Succeeded"
)

var (
	resyncPeriod        = time.Minute
	procCmdlineFilePath = "/proc/cmdline"
	sysLockdownFilePath = "/sys/kernel/security/lockdown"
	sysModuleDir        = "/sys/module"
	kernelParams        = []string{"intel_iommu=on", "iommu=pt"}
)

type DrainAndExecute func(configurer func(ctx context.Context) bool, drain bool) error

type RestartDevicePluginFunction func() error

type procState int

const (
	procAlive procState = iota
	procDead
	procUnknown
)

func (s procState) String() string {
	switch s {
	case procAlive:
		return "alive"
	case procDead:
		return "dead"
	}
	return "unknown"
}

// runPgrepCount runs "pgrep --count --full <pattern>" and returns its output.
// pgrep exits with 1 when no process matches; that is not an error here.
var runPgrepCount = func(log *logrus.Logger, pattern string) (string, error) {
	return execAndSuppress([]string{"pgrep", "--count", "--full", pattern}, log, func(e error) bool {
		var ee *exec.ExitError
		return errors.As(e, &ee) && ee.ExitCode() == 1
	})
}

// pfBbConfigProcState tells whether a pf_bb_config process serves pciAddr.
// It returns procUnknown when that cannot be determined (pgrep failed or
// printed something unexpected). Callers must not treat procUnknown as dead:
// a dead pf_bb_config triggers a full reconfiguration of the PF.
func pfBbConfigProcState(log *logrus.Logger, pciAddr string) procState {
	defaultLogLevel := log.GetLevel()
	if defaultLogLevel == logrus.InfoLevel {
		log.SetLevel(logrus.WarnLevel)
	}
	stdout, err := runPgrepCount(log, fmt.Sprintf("pf_bb_config.*%s", pciAddr))
	log.SetLevel(defaultLogLevel)
	if err != nil {
		log.WithError(err).WithField("pciAddr", pciAddr).Error("failed to determine status of pf-bb-config daemon")
		return procUnknown
	}
	count, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil || count < 0 {
		log.WithError(err).WithField("pciAddr", pciAddr).WithField("output", stdout).
			Error("failed to parse 'pgrep' output")
		return procUnknown
	}
	if count == 0 {
		return procDead
	}
	return procAlive
}

// pfBbConfigProcIsDead returns true only when pf_bb_config is known not to
// run for pciAddr.
func pfBbConfigProcIsDead(log *logrus.Logger, pciAddr string) bool {
	state := pfBbConfigProcState(log, pciAddr)
	if state == procUnknown {
		log.WithField("pciAddr", pciAddr).
			Warning("cannot determine whether pf-bb-config is running; not reconfiguring the PF on that basis")
	}
	return state == procDead
}

func isReady(p corev1.Pod) bool {
	for _, condition := range p.Status.Conditions {
		if condition.Type == corev1.PodReady && p.Status.Phase == corev1.PodRunning {
			return true
		}
	}
	return false
}

func CreateManager(config *rest.Config, scheme *runtime.Scheme, namespace string, metricsPort int, healthProbePort int, log *logrus.Logger) (manager.Manager, error) {
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     ":" + strconv.Itoa(metricsPort),
		LeaderElection:         false,
		Namespace:              namespace,
		HealthProbeBindAddress: ":" + strconv.Itoa(healthProbePort),
	})
	if err != nil {
		return nil, err
	}

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		log.WithError(err).Error("unable to set up health check")
		return nil, err
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		log.WithError(err).Error("unable to set up ready check")
		return nil, err
	}
	return mgr, nil
}

func moduleParameterIsEnabled(moduleName, parameter string) error {
	value, err := os.ReadFile(filepath.Join(sysModuleDir, moduleName, "parameters", parameter))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// module is not loaded - we will automatically append required parameter during modprobe
			return nil
		} else {
			return fmt.Errorf("failed to check parameter %v for %v module - %v", parameter, moduleName, err)
		}
	}
	if strings.Contains(strings.ToLower(string(value)), "n") {
		return fmt.Errorf("%v is loaded and doesn't have %v set", moduleName, parameter)
	}
	return nil
}

func validateOrdinalKernelParams(cmdline string) error {
	for _, param := range kernelParams {
		if !strings.Contains(cmdline, param) {
			return fmt.Errorf("missing kernel param(%s)", param)
		}
	}
	return nil
}
