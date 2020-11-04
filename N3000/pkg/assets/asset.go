// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package assets

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"text/template"
	"time"

	"github.com/go-logr/logr"

	secv1 "github.com/openshift/api/security/v1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/deprecated/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	utilruntime.Must(secv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(promv1.AddToScheme(scheme.Scheme))
}

// ReadinessPollConfig stores config for waiting block
// Use when deployment of an asset should wait until the asset is ready
type ReadinessPollConfig struct {
	// How many times readiness should be checked before returning error
	Retries int

	// Delay between retries
	Delay time.Duration
}

// Asset represents a set of Kubernetes objects to be deployed.
type Asset struct {
	// Path contains a filepath to the asset
	Path string

	// BlockingReadiness stores polling configuration.
	BlockingReadiness ReadinessPollConfig

	substitutions map[string]string

	objects []runtime.Object

	log logr.Logger
}

func (a *Asset) loadFromFile() error {
	cleanPath := filepath.Clean(a.Path)

	content, err := ioutil.ReadFile(cleanPath)
	if err != nil {
		return err
	}

	t, err := template.New("asset").Option("missingkey=error").Parse(string(content))
	if err != nil {
		return err
	}

	var templatedContent bytes.Buffer
	err = t.Execute(&templatedContent, a.substitutions)
	if err != nil {
		return err
	}

	rx := regexp.MustCompile("\n-{3}")
	objectsDefs := rx.Split(templatedContent.String(), -1)

	for _, objectDef := range objectsDefs {
		obj, _, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(objectDef), nil, nil)
		if err != nil {
			return err
		}

		a.objects = append(a.objects, obj)
	}

	return nil
}

func (a *Asset) loadFromDir() error {
	return errors.New("not supported yet")
}

func (a *Asset) load() error {
	a.Path = filepath.Clean(a.Path)

	fileInfo, err := os.Stat(a.Path)
	if err != nil {
		return err
	}

	if fileInfo.Mode().IsDir() {
		return a.loadFromDir()
	}

	return a.loadFromFile()
}

func (a *Asset) createOrUpdate(ctx context.Context, c client.Client) error {
	for _, obj := range a.objects {
		a.log.Info("createOrUpdate", "asset", a.Path, "kind", obj.GetObjectKind())

		objCopy := obj.DeepCopyObject()
		result, err := ctrl.CreateOrUpdate(ctx, c, obj, func() error {
			if objCopy.GetObjectKind().GroupVersionKind().Kind == "DaemonSet" {
				ds, ok := obj.(*appsv1.DaemonSet)
				dsCopy, okCopy := objCopy.(*appsv1.DaemonSet)
				if ok && okCopy {
					ds.Spec = dsCopy.Spec
				} else {
					a.log.Error(nil, "kind is daemonset, but casting type failed", "ok", ok, "okCopy", okCopy)
					return errors.New("kind is daemonset, but casting type failed")
				}
			}
			return nil
		})

		if err != nil {
			a.log.Error(err, "CreateOrUpdate")
			return err
		}

		a.log.Info("CreateOrUpdate", "result", result)
	}

	return nil
}

func (a *Asset) waitUntilReady(ctx context.Context, apiReader client.Reader) error {
	if a.BlockingReadiness.Retries == 0 {
		return nil
	}

	for _, obj := range a.objects {
		kind := obj.GetObjectKind().GroupVersionKind().Kind
		if kind == "DaemonSet" {
			a.log.Info("waiting until daemonset is ready", "asset", a.Path)

			backoff := wait.Backoff{
				Steps:    a.BlockingReadiness.Retries,
				Duration: a.BlockingReadiness.Delay,
				Factor:   1,
			}
			f := func() (bool, error) {
				objKey, err := client.ObjectKeyFromObject(obj)
				if err != nil {
					return false, err
				}

				ds := &appsv1.DaemonSet{}
				err = apiReader.Get(ctx, objKey, ds)
				if err != nil {
					return false, err
				}

				a.log.Info("daemonset status", "name", ds.GetName(),
					"NumberUnavailable", ds.Status.NumberUnavailable,
					"DesiredNumberScheduled", ds.Status.DesiredNumberScheduled)

				return ds.Status.NumberUnavailable == 0, nil
			}

			if err := wait.ExponentialBackoff(backoff, f); err != nil {
				a.log.Error(err, "wait for daemonset failed")
				return err
			}
		}
	}

	return nil
}
