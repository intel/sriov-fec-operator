// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2024 Intel Corporation

package assets

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/intel/sriov-fec-operator/pkg/common/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	// +kubebuilder:scaffold:imports
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	fakeOwner metav1.Object

	testEnv *envtest.Environment

	fakeConfigMapName string
	fakeAssetFile     string

	tolerationKey      = "key"
	tolerationOperator = corev1.TolerationOperator("Exists")
	tolerationEffect   = corev1.TaintEffect("NoSchedule")
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"N3000 assets Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	var err error
	logf.SetLogger(logr.New(utils.NewLogWrapper()))

	fakeAssetFile = "test/101-fake-labeler.yaml"
	fakeConfigMapName = "fake-config"

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
	}

	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	fakeOwner = &appsv1.Deployment{}
	fakeOwner.SetUID("123")
	fakeOwner.SetName("123")

	name := "sriov-fec-controller-manager"
	err = os.Setenv("NAME", name+"-123-234")
	Expect(err).To(Succeed())

	deploymentforTests := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(0),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"test": "test"},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Tolerations: []corev1.Toleration{{
						Key:      tolerationKey,
						Operator: tolerationOperator,
						Effect:   tolerationEffect,
					}},

					Containers: []corev1.Container{{
						Name:  "test-container",
						Image: "test-container",
					}},
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"test": "test"},
				},
			},
			Strategy:                appsv1.DeploymentStrategy{},
			MinReadySeconds:         0,
			RevisionHistoryLimit:    nil,
			Paused:                  false,
			ProgressDeadlineSeconds: nil,
		},
	}
	err = k8sClient.Create(context.TODO(), deploymentforTests)
	Expect(err).To(Succeed())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
