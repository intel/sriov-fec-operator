// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package assets

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Manager loads & deploys assets specified in the Asset field
type Manager struct {
	Client client.Client

	Log    logr.Logger
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
		m.Log.Error(nil, "received empty node list")
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
	log := m.Log.WithName("Load()")
	tv, err := m.buildTemplateVars(ctx, setKernelVar)
	if err != nil {
		log.Error(err, "failed to build template vars")
		return err
	}
	log.V(2).Info("template vars", "tv", tv)

	for idx := range m.Assets {
		log.V(4).Info("loading asset", "path", m.Assets[idx].Path)

		m.Assets[idx].log = m.Log.WithName("asset")
		m.Assets[idx].substitutions = tv

		if err := m.Assets[idx].load(); err != nil {
			log.Error(err, "failed to load asset", "path", m.Assets[idx].Path)
			return err
		}

		log.V(2).Info("asset loaded successfully", "path", m.Assets[idx].Path, "objects", len(m.Assets[idx].objects))
	}

	return nil
}

// Deploy will create (or update) each asset
func (m *Manager) Deploy(ctx context.Context) error {
	log := m.Log.WithName("Deploy()")

	for _, asset := range m.Assets {
		log.V(4).Info("deploying asset", "path", asset.Path, "retries",
			asset.BlockingReadiness.Retries, "delay", asset.BlockingReadiness.Delay.String(),
			"objects", len(asset.objects))

		if err := asset.createOrUpdate(ctx, m.Client, m.Owner, m.Scheme); err != nil {
			log.Error(err, "failed to create asset", "path", asset.Path)
			return err
		}

		log.V(2).Info("asset created successfully", "path", asset.Path)

		if err := asset.waitUntilReady(ctx, m.Client); err != nil {
			log.Error(err, "waitUntilReady")
			return err
		}
	}

	return nil
}
