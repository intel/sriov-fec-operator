// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package assets

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/go-logr/logr"

	secv1 "github.com/openshift/api/security/v1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/apimachinery/pkg/api/equality"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	rscheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(secv1.AddToScheme(rscheme))
	utilruntime.Must(promv1.AddToScheme(rscheme))
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

	objects []client.Object

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
		obj := unstructured.Unstructured{}
		r := strings.NewReader(objectDef)
		decoder := yaml.NewYAMLOrJSONDecoder(r, 4096)
		err := decoder.Decode(&obj)
		if err != nil {
			return err
		}

		a.objects = append(a.objects, &obj)
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

func (a *Asset) setOwner(owner metav1.Object, obj runtime.Object, s *runtime.Scheme) error {
	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return errors.New(obj.GetObjectKind().GroupVersionKind().String() + " is not metav1.Object")
	}

	if owner.GetNamespace() == metaObj.GetNamespace() {
		a.log.V(4).Info("set owner for object", "owner", owner.GetName()+"."+owner.GetNamespace(),
			"object", metaObj.GetName()+"."+metaObj.GetNamespace())
		if err := controllerutil.SetControllerReference(owner, metaObj, s); err != nil {
			return err
		}
	} else {
		a.log.V(4).Info("Unsupported owner for object...skipping", "owner", owner.GetName()+"."+owner.GetNamespace(),
			"object", metaObj.GetName()+"."+metaObj.GetNamespace())
	}
	return nil
}

func (a *Asset) createOrUpdate(ctx context.Context, c client.Client, o metav1.Object, s *runtime.Scheme) error {
	for _, obj := range a.objects {
		a.log.V(4).Info("create or update", "asset", a.Path, "kind", obj.GetObjectKind())

		err := a.setOwner(o, obj, s)
		if err != nil {
			return err
		}

		gvk := obj.GetObjectKind().GroupVersionKind()
		old := &unstructured.Unstructured{}
		old.SetGroupVersionKind(gvk)
		key := client.ObjectKeyFromObject(obj)
		if err := c.Get(ctx, key, old); err != nil {
			if !apierr.IsNotFound(err) {
				a.log.Error(err, "Failed to get an object", "key", key, "GroupVersionKind", gvk)
				return err
			}

			// Object does not exist
			if err := c.Create(ctx, obj); err != nil {
				a.log.Error(err, "Create failed", "key", key, "GroupVersionKind", gvk)
				return err
			}
			a.log.V(2).Info("Object created", "key", key, "GroupVersionKind", gvk)
		} else {
			if strings.ToLower(old.GetObjectKind().GroupVersionKind().Kind) == "configmap" {
				isImmutable, ok := old.Object["immutable"].(bool)
				if !ok {
					a.log.V(2).Info("Failed to read 'immutable' field.", "key", key, "GroupVersionKind", gvk)
				} else {
					if isImmutable {
						a.log.V(2).Info("Skipping update because it is marked as immutable", "key", key, "GroupVersionKind", gvk)
						continue
					}
				}
			}

			if !equality.Semantic.DeepDerivative(obj, old) {
				obj.SetResourceVersion(old.GetResourceVersion())
				if err := c.Update(ctx, obj); err != nil {
					a.log.Error(err, "Update failed", "key", key, "GroupVersionKind", gvk)
					return err
				}
				a.log.V(2).Info("Object updated", "key", key, "GroupVersionKind", gvk)
			} else {
				a.log.V(4).Info("Object has not changed", "key", key, "GroupVersionKind", gvk)
			}
		}
	}

	return nil
}

func (a *Asset) waitUntilReady(ctx context.Context, apiReader client.Reader) error {
	if a.BlockingReadiness.Retries == 0 {
		return nil
	}

	for _, obj := range a.objects {
		if obj.GetObjectKind().GroupVersionKind().Kind == "DaemonSet" {
			a.log.V(2).Info("waiting until daemonset is ready", "asset", a.Path)

			backoff := wait.Backoff{
				Steps:    a.BlockingReadiness.Retries,
				Duration: a.BlockingReadiness.Delay,
				Factor:   1,
			}
			f := func() (bool, error) {
				objKey := client.ObjectKeyFromObject(obj)
				ds := &appsv1.DaemonSet{}
				err := apiReader.Get(ctx, objKey, ds)
				if err != nil {
					return false, err
				}

				a.log.V(4).Info("daemonset status", "name", ds.GetName(),
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
