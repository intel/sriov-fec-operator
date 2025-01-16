// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2024 Intel Corporation

package daemon

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewDevicePluginController(c client.Client, log *logrus.Logger, nnr types.NamespacedName) *DevicePluginController {
	return &DevicePluginController{
		Client:      c,
		log:         log,
		nodeNameRef: nnr,
	}
}

type DevicePluginController struct {
	client.Client
	log         *logrus.Logger
	nodeNameRef types.NamespacedName
}

func (d *DevicePluginController) RestartDevicePlugin() error {
	pods := &corev1.PodList{}
	err := d.List(context.TODO(), pods,
		client.InNamespace(d.nodeNameRef.Namespace),
		&client.MatchingLabels{"app": "sriov-device-plugin-daemonset"})

	if err != nil {
		return errors.Wrap(err, "failed to get pods")
	}
	if len(pods.Items) == 0 {
		d.log.Info("there is no running instance of device plugin, nothing to restart")
	}

	for i := range pods.Items {
		if pods.Items[i].Spec.NodeName != d.nodeNameRef.Name {
			continue
		}
		if err := d.Delete(context.TODO(), &pods.Items[i], &client.DeleteOptions{}); err != nil {
			return errors.Wrap(err, "failed to delete sriov-device-plugin-daemonset pod")
		}
		backoff := wait.Backoff{Steps: 300, Duration: 1 * time.Second, Factor: 1}
		err = wait.ExponentialBackoff(backoff, d.waitForDevicePluginRestart(pods.Items[i].Name))
		if err == wait.ErrWaitTimeout {
			return fmt.Errorf("failed to restart sriov-device-plugin within specified time")
		}
		return err
	}

	return nil
}

func (d *DevicePluginController) waitForDevicePluginRestart(oldPodName string) func() (bool, error) {
	return func() (bool, error) {
		pods := &corev1.PodList{}

		err := d.List(context.TODO(), pods,
			client.InNamespace(d.nodeNameRef.Namespace),
			&client.MatchingLabels{"app": "sriov-device-plugin-daemonset"})
		if err != nil {
			d.log.WithError(err).Error("failed to list pods for sriov-device-plugin")
			return false, err
		}

		for _, pod := range pods.Items {
			if pod.Spec.NodeName == d.nodeNameRef.Name && pod.Name != oldPodName && isReady(pod) {
				d.log.Info("device-plugin is running")
				return true, nil
			}

		}
		return false, nil
	}
}
