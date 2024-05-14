// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2024 Intel Corporation

package daemon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
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
	kernelParams        = []string{"intel_iommu=on", "iommu=pt"}
)

type DrainAndExecute func(configurer func(ctx context.Context) bool, drain bool) error

type RestartDevicePluginFunction func() error

func pfBbConfigProcIsDead(log *logrus.Logger, pciAddr string) bool {
	stdout, err := execCmd([]string{
		"pgrep",
		"--count",
		"--full",
		fmt.Sprintf("pf_bb_config.*%s", pciAddr),
	}, log)
	if err != nil {
		log.WithError(err).Error("failed to determine status of pf-bb-config daemon")
		return true
	}
	matchingProcCount, err := strconv.Atoi(stdout[0:1]) //stdout contains characters like '\n', so we are removing them
	if err != nil {
		log.WithError(err).Error("failed to convert 'pgrep' output to int")
		return true
	}
	return matchingProcCount == 0
}

func isReady(p corev1.Pod) bool {
	for _, condition := range p.Status.Conditions {
		if condition.Type == corev1.PodReady && p.Status.Phase == corev1.PodRunning {
			return true
		}
	}
	return false
}

func CreateManager(config *rest.Config, scheme *runtime.Scheme, namespace string, metricsPort int, HealthProbePort int, log *logrus.Logger) (manager.Manager, error) {
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     ":" + strconv.Itoa(metricsPort),
		LeaderElection:         false,
		Namespace:              namespace,
		HealthProbeBindAddress: ":" + strconv.Itoa(HealthProbePort),
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
	value, err := os.ReadFile("/sys/module/" + moduleName + "/parameters/" + parameter)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// module is not loaded - we will automatically append required parameter during modprobe
			return nil
		} else {
			return fmt.Errorf("failed to check parameter %v for %v module - %v", parameter, moduleName, err)
		}
	}
	if strings.Contains(strings.ToLower(string(value)), "n") {
		return fmt.Errorf(moduleName + " is loaded and doesn't has " + parameter + " set")
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
