// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package assets

import (
	"context"
	"errors"
	"github.com/otcshare/openshift-operator/sriov-fec/pkg/common/utils"
	"github.com/sirupsen/logrus"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Manager loads & deploys assets specified in the Asset field
type Manager struct {
	Client    client.Client
	Namespace string

	Log    *logrus.Logger
	Assets []Asset

	// Prefix used to gather enviroment variables for the templating the assets
	EnvPrefix string

	// Can be removed after sigs.k8s.io/controller-runtime v0.7.0 release where client.Scheme() is available
	Scheme *runtime.Scheme

	Owner metav1.Object
}

// buildTemplateVars creates map with variables for templating.
// Template variables are env variables with specified prefix and additional information
// from cluster such as kernel
func (m *Manager) buildTemplateVars(ctx context.Context, setKernelVar bool) (map[string]string, error) {
	tp := make(map[string]string)

	for _, pair := range os.Environ() {
		kv := strings.Split(pair, "=")
		if len(kv) == 2 && strings.HasPrefix(kv[0], m.EnvPrefix) {
			tp[kv[0]] = kv[1]
		}
	}

	if !setKernelVar {
		return tp, nil
	}

	nodes := &corev1.NodeList{}
	err := m.Client.List(ctx, nodes, &client.MatchingLabels{"fpga.intel.com/intel-accelerator-present": ""})
	if err != nil {
		return nil, err
	}

	if len(nodes.Items) == 0 {
		m.Log.Error("received empty node list")
		return nil, errors.New("empty node list while building template vars")
	}

	tp["kernel"] = nodes.Items[0].Status.NodeInfo.KernelVersion

	return tp, nil
}

// DeployConfigMaps issues an asset load from the path and then deployment
func (m *Manager) DeployConfigMaps(ctx context.Context, setKernelVar bool) error {
	if err := m.LoadFromFile(ctx, setKernelVar); err != nil {
		return err
	}
	if err := m.Deploy(ctx); err != nil {
		return err
	}

	return nil
}

// LoadFromFile loads given asset from the path
func (m *Manager) LoadFromFile(ctx context.Context, setKernelVar bool) error {
	tv, err := m.buildTemplateVars(ctx, setKernelVar)
	if err != nil {
		m.Log.WithError(err).Error("failed to build template vars")
		return err
	}
	m.Log.WithField("tv", tv).Info("template vars")

	for idx := range m.Assets {
		m.Log.WithField("path", m.Assets[idx].Path).Info("loading asset")

		assetLogger := utils.NewLogger()

		m.Assets[idx].log = assetLogger
		m.Assets[idx].substitutions = tv

		if err := m.Assets[idx].loadFromFile(); err != nil {
			m.Log.WithError(err).WithField("path", m.Assets[idx].Path).Error("failed to loadFromFile asset")
			return err
		}

		m.Log.WithFields(logrus.Fields{
			"path":           m.Assets[idx].Path,
			"num of objects": len(m.Assets[idx].objects),
		}).Info("asset loaded successfully")
	}

	return nil
}

// LoadFromConfigMapAndDeploy issues an asset load from the ConfigMap and then deployment
func (m *Manager) LoadFromConfigMapAndDeploy(ctx context.Context) error {
	if err := m.LoadFromConfigMap(ctx); err != nil {
		return err
	}
	if err := m.Deploy(ctx); err != nil {
		return err
	}

	return nil
}

// LoadFromConfigMap loads given asset from the ConfigMap
func (m *Manager) LoadFromConfigMap(ctx context.Context) error {
	for idx := range m.Assets {
		m.Log.WithField("configMapName", m.Assets[idx].ConfigMapName).Info("loading asset")

		if err := m.Assets[idx].loadFromConfigMap(ctx, m.Client, m.Namespace); err != nil {
			m.Log.WithError(err).WithField("ConfigMap name", m.Assets[idx].ConfigMapName).Error("failed to loadFromConfigMap")
			return err
		}

		m.Log.WithFields(logrus.Fields{
			"ConfigMap name": m.Assets[idx].ConfigMapName,
			"num of objects": len(m.Assets[idx].objects),
		}).Info("asset loaded successfully")
	}

	return nil
}

// Deploy will create (or update) each asset
func (m *Manager) Deploy(ctx context.Context) error {
	for _, asset := range m.Assets {
		m.Log.WithFields(logrus.Fields{
			"path":    asset.Path,
			"retries": asset.BlockingReadiness.Retries,
			"delay":   asset.BlockingReadiness.Delay.String(),
			"objects": len(asset.objects),
		}).Info("deploying asset")

		if err := asset.createOrUpdate(ctx, m.Client, m.Owner, m.Scheme); err != nil {
			m.Log.WithError(err).WithField("path", asset.Path).Error("failed to create asset")
			return err
		}

		m.Log.WithField("path", asset.Path).Info("asset created successfully")

		if err := asset.waitUntilReady(ctx, m.Client); err != nil {
			m.Log.WithError(err).Error("waitUntilReady")
			return err
		}
	}

	return nil
}
