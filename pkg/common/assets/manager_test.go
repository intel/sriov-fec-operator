// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2022 Intel Corporation

package assets

import (
	"context"
	"github.com/smart-edge-open/sriov-fec-operator/sriov-fec/pkg/common/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
)

//  runtime.Object implementation
type InvalidRuntimeType struct {
}

func (*InvalidRuntimeType) GetNamespace() string                                   { return "" }
func (*InvalidRuntimeType) SetNamespace(namespace string)                          {}
func (*InvalidRuntimeType) GetName() string                                        { return "" }
func (*InvalidRuntimeType) SetName(name string)                                    {}
func (*InvalidRuntimeType) GetGenerateName() string                                { return "" }
func (*InvalidRuntimeType) SetGenerateName(name string)                            {}
func (*InvalidRuntimeType) GetUID() types.UID                                      { return "" }
func (*InvalidRuntimeType) SetUID(uid types.UID)                                   {}
func (*InvalidRuntimeType) GetResourceVersion() string                             { return "" }
func (*InvalidRuntimeType) SetResourceVersion(version string)                      {}
func (*InvalidRuntimeType) GetGeneration() int64                                   { return 0 }
func (*InvalidRuntimeType) SetGeneration(generation int64)                         {}
func (*InvalidRuntimeType) GetSelfLink() string                                    { return "" }
func (*InvalidRuntimeType) SetSelfLink(selfLink string)                            {}
func (*InvalidRuntimeType) GetCreationTimestamp() v1.Time                          { return v1.Now() }
func (*InvalidRuntimeType) SetCreationTimestamp(timestamp v1.Time)                 {}
func (*InvalidRuntimeType) GetDeletionTimestamp() *v1.Time                         { return nil }
func (*InvalidRuntimeType) SetDeletionTimestamp(timestamp *v1.Time)                {}
func (*InvalidRuntimeType) GetDeletionGracePeriodSeconds() *int64                  { return nil }
func (*InvalidRuntimeType) SetDeletionGracePeriodSeconds(*int64)                   {}
func (*InvalidRuntimeType) GetLabels() map[string]string                           { return nil }
func (*InvalidRuntimeType) SetLabels(labels map[string]string)                     {}
func (*InvalidRuntimeType) GetAnnotations() map[string]string                      { return nil }
func (*InvalidRuntimeType) SetAnnotations(annotations map[string]string)           {}
func (*InvalidRuntimeType) GetFinalizers() []string                                { return nil }
func (*InvalidRuntimeType) SetFinalizers(finalizers []string)                      {}
func (*InvalidRuntimeType) GetOwnerReferences() []v1.OwnerReference                { return nil }
func (*InvalidRuntimeType) SetOwnerReferences([]v1.OwnerReference)                 {}
func (*InvalidRuntimeType) GetZZZ_DeprecatedClusterName() string                   { return "" }
func (*InvalidRuntimeType) SetZZZ_DeprecatedClusterName(clusterName string)        {}
func (*InvalidRuntimeType) GetManagedFields() []v1.ManagedFieldsEntry              { return nil }
func (*InvalidRuntimeType) SetManagedFields(managedFields []v1.ManagedFieldsEntry) {}

func (i *InvalidRuntimeType) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}
func (i *InvalidRuntimeType) DeepCopyObject() runtime.Object {
	return i
}

var _ = Describe("Asset Tests", func() {

	log := utils.NewLogger()

	var _ = Describe("Manager - load configmap from file and deploy", func() {
		var _ = It("Run Manager with no assets (setKernel false)", func() {
			var err error
			log = utils.NewLogger()

			manager := Manager{Client: k8sClient, Log: log}

			err = manager.DeployConfigMaps(context.TODO(), false)
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("Run Manager with no assets (setKernel true)", func() {
			var err error
			log = utils.NewLogger()

			manager := Manager{Client: k8sClient, Log: log}

			err = manager.DeployConfigMaps(context.TODO(), true)
			Expect(err).To(HaveOccurred())
		})
		var _ = It("Run Manager (setKernel true)", func() {
			var err error
			log = utils.NewLogger()

			assets := []Asset{
				{
					log:  log,
					Path: "/tmp/dummy.bin",
				},
			}

			manager := Manager{Client: k8sClient,
				Log:    log,
				Assets: assets}

			err = manager.DeployConfigMaps(context.TODO(), true)
			Expect(err).To(HaveOccurred())
		})
		var _ = It("Run Manager loadFromDir (setKernel false)", func() {
			var err error
			log = utils.NewLogger()

			assets := []Asset{
				{
					log:  log,
					Path: "/tmp/",
				},
			}

			manager := Manager{Client: k8sClient,
				Log:    log,
				Assets: assets}

			err = manager.DeployConfigMaps(context.TODO(), false)
			Expect(err).To(HaveOccurred())
		})
		var _ = It("Run Manager loadFromFile (setKernel false)", func() {
			var err error
			log = utils.NewLogger()

			assets := []Asset{
				{
					log:           log,
					Path:          fakeAssetFile,
					substitutions: map[string]string{"one": "two"},
				},
			}

			manager := Manager{Client: k8sClient,
				Log:    log,
				Assets: assets,
				Owner:  fakeOwner,
				Scheme: scheme.Scheme}

			err = manager.DeployConfigMaps(context.TODO(), false)
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("Run LoadAndDeploy (fail setting Owner)", func() {
			var err error
			log = utils.NewLogger()

			var invalidObject InvalidRuntimeType

			assets := []Asset{
				{
					log:           log,
					Path:          fakeAssetFile,
					substitutions: map[string]string{"one": "two"},
					objects: []client.Object{
						&invalidObject},
				},
			}

			manager := Manager{Client: k8sClient,
				Log:    log,
				Assets: assets,
				Owner:  fakeOwner,
				Scheme: scheme.Scheme}

			Expect(manager).ToNot(Equal(nil))

			// Create a Node
			node := &corev1.Node{
				ObjectMeta: v1.ObjectMeta{
					Name: "dummy",
					Labels: map[string]string{
						"fpga.intel.com/intel-accelerator-present": "",
					},
				},
			}

			err = k8sClient.Create(context.Background(), node)
			Expect(err).ToNot(HaveOccurred())

			err = manager.DeployConfigMaps(context.TODO(), true)
			Expect(err).To(HaveOccurred())

			// Cleanup
			err = k8sClient.Delete(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("Run Manager loadFromFile (bad file)", func() {
			var err error
			log = utils.NewLogger()

			assets := []Asset{
				{
					log:           log,
					Path:          "/dev/null",
					substitutions: map[string]string{"one": "two"},
					BlockingReadiness: ReadinessPollConfig{
						Retries: 1,
					},
				},
			}

			manager := Manager{Client: k8sClient,
				Log:    log,
				Assets: assets,
				Owner:  fakeOwner,
				Scheme: scheme.Scheme}

			err = manager.DeployConfigMaps(context.TODO(), false)
			Expect(err).To(HaveOccurred())
		})
		var _ = It("Run Manager loadFromFile (missing file)", func() {
			var err error
			log = utils.NewLogger()

			assets := []Asset{
				{
					log:           log,
					Path:          "/dev/null_fake",
					substitutions: map[string]string{"one": "two"},
					BlockingReadiness: ReadinessPollConfig{
						Retries: 1,
					},
				},
			}

			manager := Manager{Client: k8sClient,
				Log:    log,
				Assets: assets,
				Owner:  fakeOwner,
				Scheme: scheme.Scheme}

			err = manager.DeployConfigMaps(context.TODO(), false)
			Expect(err).To(HaveOccurred())
		})
	})

	var _ = Describe("Manager - load objects from configmap and deploy", func() {
		var _ = It("Run Manager loadFromFile (setKernel false, no retries)", func() {
			var err error
			log = utils.NewLogger()

			assets := []Asset{
				{
					log:           log,
					Path:          fakeAssetFile,
					ConfigMapName: fakeConfigMapName,
					substitutions: map[string]string{"one": "two"},
					BlockingReadiness: ReadinessPollConfig{
						Retries: 1,
					},
				},
			}

			manager := Manager{Client: k8sClient,
				Log:    log,
				Assets: assets,
				Owner:  fakeOwner,
				Scheme: scheme.Scheme}

			getConfigMap = func(ctx context.Context, c client.Client, cmName string, ns string) (corev1.ConfigMap, error) {
				configMap := corev1.ConfigMap{
					Data: map[string]string{
						"daemonSet": "apiVersion: apps/v1\nkind: DaemonSet\nmetadata:\n  labels:\n    app: accelerator-discovery\n  name: accelerator-discovery\n  namespace: default\nspec:\n  minReadySeconds: 10\n  selector:\n    matchLabels:\n      app: accelerator-discovery\n  template:\n    metadata:\n      labels:\n        app: accelerator-discovery\n      name: accelerator-discovery\n    spec:\n      serviceAccount: accelerator-discovery\n      serviceAccountName: accelerator-discovery\n      containers:\n      - image: \"N3000_LABELER_IMAGE-123\"\n        name: accelerator-discovery",
					},
				}
				return configMap, nil
			}

			err = manager.LoadFromConfigMapAndDeploy(context.TODO())
			Expect(err).ToNot(HaveOccurred())
		})

		var _ = It("Run Manager loadFromFile (invalid retries count)", func() {
			var err error
			log = utils.NewLogger()

			assets := []Asset{
				{
					log:           log,
					Path:          fakeAssetFile,
					substitutions: map[string]string{"one": "two"},
					BlockingReadiness: ReadinessPollConfig{
						Retries: -1,
					},
				},
			}

			manager := Manager{Client: k8sClient,
				Log:    log,
				Assets: assets,
				Owner:  fakeOwner,
				Scheme: scheme.Scheme}

			getConfigMap = func(ctx context.Context, c client.Client, cmName string, ns string) (corev1.ConfigMap, error) {
				configMap := corev1.ConfigMap{
					Data: map[string]string{
						"daemonSet": "apiVersion: apps/v1\nkind: DaemonSet\nmetadata:\n  labels:\n    app: accelerator-discovery\n  name: accelerator-discovery\n  namespace: default\nspec:\n  minReadySeconds: 10\n  selector:\n    matchLabels:\n      app: accelerator-discovery\n  template:\n    metadata:\n      labels:\n        app: accelerator-discovery\n      name: accelerator-discovery\n    spec:\n      serviceAccount: accelerator-discovery\n      serviceAccountName: accelerator-discovery\n      containers:\n      - image: \"N3000_LABELER_IMAGE-123\"\n        name: accelerator-discovery",
					},
				}
				return configMap, nil
			}

			err = manager.LoadFromConfigMapAndDeploy(context.TODO())
			Expect(err).To(HaveOccurred())
		})
		var _ = It("Run Manager loadFromConfigMap (nonexistent configmap name)", func() {
			assets := []Asset{
				{
					log:           log,
					Path:          fakeAssetFile,
					ConfigMapName: fakeConfigMapName,
				},
			}

			manager := Manager{Client: k8sClient,
				Log:    log,
				Assets: assets,
				Owner:  fakeOwner,
				Scheme: scheme.Scheme}

			getConfigMap = getConfigMapData

			Expect(manager.LoadFromConfigMapAndDeploy(context.TODO())).To(HaveOccurred())
		})
		var _ = It("Run Manager loadFromConfigMap (update existing valid configmap)", func() {
			assets := []Asset{
				{
					log:           log,
					Path:          fakeAssetFile,
					ConfigMapName: fakeConfigMapName,
				},
			}

			manager := Manager{Client: k8sClient,
				Log:    log,
				Assets: assets,
				Owner:  fakeOwner,
				Scheme: scheme.Scheme}

			getConfigMap = func(ctx context.Context, c client.Client, cmName string, ns string) (corev1.ConfigMap, error) {
				configMap := corev1.ConfigMap{
					Data: map[string]string{
						"configMap":         "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: supported-clv-devices\n  namespace: default\nimmutable: false\ndata:\n  fake-key-1: fake-value-1\n  fake-key-2: fake-value-2",
						"configMap-updated": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: supported-clv-devices\n  namespace: default\nimmutable: false\ndata:\n  fake-key-1: new-fake-value-1\n  fake-key-2: fake-value-2",
					},
				}
				return configMap, nil
			}

			err := manager.LoadFromConfigMapAndDeploy(context.TODO())
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
