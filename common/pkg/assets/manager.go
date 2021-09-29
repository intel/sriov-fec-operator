// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package assets

import (
	"context"
	"errors"
	"github.com/smart-edge-open/openshift-operator/common/pkg/utils"
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
	Client client.Client

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
	tp[m.EnvPrefix+"GENERIC_K8S"] = "false"

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

// LoadAndDeploy issues an asset load and then deployment
func (m *Manager) LoadAndDeploy(ctx context.Context, setKernelVar bool) error {
	if err := m.Load(ctx, setKernelVar); err != nil {
		return err
	}
	if err := m.Deploy(ctx); err != nil {
		return err
	}
	return nil
}

// Load loads given assets from paths
func (m *Manager) Load(ctx context.Context, setKernelVar bool) error {
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

		if err := m.Assets[idx].load(); err != nil {
			m.Log.WithError(err).WithField("path", m.Assets[idx].Path).Error("failed to load asset")
			return err
		}

		m.Log.WithField("path", m.Assets[idx].Path).WithField("objects", len(m.Assets[idx].objects)).
			Info("asset loaded successfully")
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
