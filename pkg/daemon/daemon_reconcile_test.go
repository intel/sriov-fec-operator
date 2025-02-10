// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2025 Intel Corporation

package daemon

import (
	"context"
	"fmt"

	sriovv2 "github.com/intel/sriov-fec-operator/api/sriovfec/v2"
	vrbv1 "github.com/intel/sriov-fec-operator/api/sriovvrb/v1"
	"github.com/intel/sriov-fec-operator/pkg/common/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("FecNodeConfigReconciler.Reconcile", func() {
	var scheme *runtime.Scheme

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(sriovv2.AddToScheme(scheme)).ToNot(HaveOccurred())
		Expect(vrbv1.AddToScheme(scheme)).ToNot(HaveOccurred())
	})

	_ = Describe("", func() {
		var (
			fakeClient         client.Client
			nodeNameRef        types.NamespacedName
			reconciler         FecNodeConfigReconciler
			reconcileRequestes ctrl.Request
			nodeInventory      *sriovv2.NodeInventory
		)
		BeforeEach(func() {
			procCmdlineFilePath = "testdata/cmdline_test"
			fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
			nodeNameRef = types.NamespacedName{Name: "worker", Namespace: "testNamespace"}
			nodeInventory = &sriovv2.NodeInventory{
				SriovAccelerators: []sriovv2.SriovAccelerator{
					{
						VendorID:   "vid",
						DeviceID:   "did",
						PCIAddress: pciAddress,
						PFDriver:   "pfdriver",
						MaxVFs:     10,
					},
				},
			}
			configurer := testConfigurerProto{
				configureNodeFunction: func(nodeConfig sriovv2.SriovFecNodeConfigSpec) (err error) {
					for _, pf := range nodeConfig.PhysicalFunctions {
						for i, accelerator := range nodeInventory.SriovAccelerators {
							if accelerator.PCIAddress != pf.PCIAddress {
								continue
							}
							nodeInventory.SriovAccelerators[i].VFs = []sriovv2.VF{}
							for i := 0; i < pf.VFAmount; i++ {
								nodeInventory.SriovAccelerators[i].VFs = append(nodeInventory.SriovAccelerators[i].VFs, sriovv2.VF{
									PCIAddress: fmt.Sprintf("%s%d", pf.PCIAddress[0:len(pf.PCIAddress)-1], i+1),
									Driver:     "vfDriver",
									DeviceID:   "deviceId",
								})
							}
						}
					}
					return err
				},
			}

			getSriovInventory = func(log *logrus.Logger) (*sriovv2.NodeInventory, error) {
				return nodeInventory, nil
			}

			reconciler = FecNodeConfigReconciler{
				Client:             fakeClient,
				log:                utils.NewLogger(),
				nodeNameRef:        nodeNameRef,
				sriovfecconfigurer: configurer,
				drainerAndExecute: func(configurer func(ctx context.Context) bool, drain bool) error {
					_ = configurer(context.TODO())
					return nil
				}, restartDevicePlugin: func() error {
					return nil
				}}
			reconcileRequestes = ctrl.Request{NamespacedName: nodeNameRef}
		})

		It("restores/recreates VFs removed after node reboot", func() {
			// SFNC does not exist yet
			sfnc := new(sriovv2.SriovFecNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, sfnc)).To(MatchError(ContainSubstring("not found")))

			// First reconcile creates missing sfnc
			// Created sfnc exposes node inventory: status.NodeInventory
			_, err := reconciler.Reconcile(context.TODO(), reconcileRequestes)
			Expect(err).ToNot(HaveOccurred())
			sfnc = new(sriovv2.SriovFecNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, sfnc)).ToNot(HaveOccurred())
			Expect(sfnc.Status.Inventory).To(Equal(*nodeInventory))

			// Define new spec
			// Fake client doesn't handle generation field so take care about incrementing it
			sfnc.Generation++
			sfnc.Spec = sriovv2.SriovFecNodeConfigSpec{
				PhysicalFunctions: []sriovv2.PhysicalFunctionConfigExt{
					{
						PCIAddress:  pciAddress,
						PFDriver:    "pfdriver",
						VFDriver:    "vfdriver",
						VFAmount:    1,
						BBDevConfig: sriovv2.BBDevConfig{},
					},
				},
			}
			err = fakeClient.Patch(context.TODO(), sfnc, client.Merge)
			Expect(err).ToNot(HaveOccurred())

			// Second reconcile should configure inventory to be aligned with requested spec
			_, err = reconciler.Reconcile(context.TODO(), reconcileRequestes)
			Expect(err).ToNot(HaveOccurred())
			sfnc = new(sriovv2.SriovFecNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, sfnc)).ToNot(HaveOccurred())
			Expect(sfnc.Status.Inventory).ToNot(Equal(nodeInventory))

			// Simulate node reboot - remove all existing VFs
			for accidx := range nodeInventory.SriovAccelerators {
				nodeInventory.SriovAccelerators[accidx].VFs = []sriovv2.VF{}
			}

			// Third reconcile should reconfigure VFs
			_, err = reconciler.Reconcile(context.TODO(), reconcileRequestes)
			Expect(err).ToNot(HaveOccurred())
			sfnc = new(sriovv2.SriovFecNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, sfnc)).ToNot(HaveOccurred())
			Expect(sfnc.Status.Inventory).ToNot(Equal(nodeInventory))
		})
	})
})

type testConfigurerProto struct {
	configureNodeFunction    func(nodeConfig sriovv2.SriovFecNodeConfigSpec) error
	vrbConfigureNodeFunction func(nodeConfig vrbv1.SriovVrbNodeConfigSpec) error
}

func (t testConfigurerProto) ApplySpec(nodeConfig sriovv2.SriovFecNodeConfigSpec, fecDeviceUpdateRequired map[string]bool) error {
	return t.configureNodeFunction(nodeConfig)
}

func (t testConfigurerProto) VrbApplySpec(nodeConfig vrbv1.SriovVrbNodeConfigSpec, vrbDeviceUpdateRequired map[string]bool) error {
	return t.vrbConfigureNodeFunction(nodeConfig)
}

var _ = Describe("VrbNodeConfigReconciler.Reconcile", func() {
	var scheme *runtime.Scheme

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(vrbv1.AddToScheme(scheme)).ToNot(HaveOccurred())
	})

	_ = Describe("", func() {
		var (
			fakeClient         client.Client
			nodeNameRef        types.NamespacedName
			reconciler         VrbNodeConfigReconciler
			reconcileRequestes ctrl.Request
			nodeInventory      *vrbv1.NodeInventory
		)
		BeforeEach(func() {
			procCmdlineFilePath = "testdata/cmdline_test"
			fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
			nodeNameRef = types.NamespacedName{Name: "worker", Namespace: "testNamespace"}
			nodeInventory = &vrbv1.NodeInventory{
				SriovAccelerators: []vrbv1.SriovAccelerator{
					{
						VendorID:   "vid",
						DeviceID:   "did",
						PCIAddress: pciAddress,
						PFDriver:   "pfdriver",
						MaxVFs:     10,
					},
				},
			}
			configurer := testConfigurerProto{
				vrbConfigureNodeFunction: func(nodeConfig vrbv1.SriovVrbNodeConfigSpec) (err error) {
					for _, pf := range nodeConfig.PhysicalFunctions {
						for i, accelerator := range nodeInventory.SriovAccelerators {
							if accelerator.PCIAddress != pf.PCIAddress {
								continue
							}
							nodeInventory.SriovAccelerators[i].VFs = []vrbv1.VF{}
							for i := 0; i < pf.VFAmount; i++ {
								nodeInventory.SriovAccelerators[i].VFs = append(nodeInventory.SriovAccelerators[i].VFs, vrbv1.VF{
									PCIAddress: fmt.Sprintf("%s%d", pf.PCIAddress[0:len(pf.PCIAddress)-1], i+1),
									Driver:     "vfDriver",
									DeviceID:   "deviceId",
								})
							}
						}
					}
					return err
				},
			}

			VrbgetSriovInventory = func(log *logrus.Logger) (*vrbv1.NodeInventory, error) {
				return nodeInventory, nil
			}

			reconciler = VrbNodeConfigReconciler{
				Client:        fakeClient,
				log:           utils.NewLogger(),
				nodeNameRef:   nodeNameRef,
				vrbconfigurer: configurer,
				drainerAndExecute: func(configurer func(ctx context.Context) bool, drain bool) error {
					_ = configurer(context.TODO())
					return nil
				}, restartDevicePlugin: func() error {
					return nil
				}}
			reconcileRequestes = ctrl.Request{NamespacedName: nodeNameRef}
		})

		It("restores/recreates VFs removed after node reboot", func() {
			// SVNC does not exist yet
			svnc := new(vrbv1.SriovVrbNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, svnc)).To(MatchError(ContainSubstring("not found")))

			// First reconcile creates missing svnc
			// Created svnc exposes node inventory: status.NodeInventory
			_, err := reconciler.Reconcile(context.TODO(), reconcileRequestes)
			Expect(err).ToNot(HaveOccurred())
			svnc = new(vrbv1.SriovVrbNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, svnc)).ToNot(HaveOccurred())
			Expect(svnc.Status.Inventory).To(Equal(*nodeInventory))

			// Define new spec
			// Fake client doesn't handle generation field so take care about incrementing it
			svnc.Generation++
			svnc.Spec = vrbv1.SriovVrbNodeConfigSpec{
				PhysicalFunctions: []vrbv1.PhysicalFunctionConfigExt{
					{
						PCIAddress:  pciAddress,
						PFDriver:    "pfdriver",
						VFDriver:    "vfdriver",
						VFAmount:    1,
						BBDevConfig: vrbv1.BBDevConfig{},
					},
				},
			}
			err = fakeClient.Patch(context.TODO(), svnc, client.Merge)
			Expect(err).ToNot(HaveOccurred())

			// Second reconcile should configure inventory to be aligned with requested spec
			_, err = reconciler.Reconcile(context.TODO(), reconcileRequestes)
			Expect(err).ToNot(HaveOccurred())
			svnc = new(vrbv1.SriovVrbNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, svnc)).ToNot(HaveOccurred())
			Expect(svnc.Status.Inventory).ToNot(Equal(nodeInventory))

			// Simulate node reboot - remove all existing VFs
			for accidx := range nodeInventory.SriovAccelerators {
				nodeInventory.SriovAccelerators[accidx].VFs = []vrbv1.VF{}
			}

			// Third reconcile should reconfigure VFs
			_, err = reconciler.Reconcile(context.TODO(), reconcileRequestes)
			Expect(err).ToNot(HaveOccurred())
			svnc = new(vrbv1.SriovVrbNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, svnc)).ToNot(HaveOccurred())
			Expect(svnc.Status.Inventory).ToNot(Equal(nodeInventory))
		})
	})
})

var _ = Describe("VrbResourceName", func() {
	var (
		reconciler    *VrbNodeConfigReconciler
		vrbnc         *vrbv1.SriovVrbNodeConfig
		acc           vrbv1.PhysicalFunctionConfigExt
		currentConfig map[string]interface{}
		resourceList  []interface{}
		scheme        *runtime.Scheme
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(vrbv1.AddToScheme(scheme)).ToNot(HaveOccurred())
		Expect(v1.AddToScheme(scheme)).ToNot(HaveOccurred()) // Register the core v1 types

		reconciler = &VrbNodeConfigReconciler{
			log:    utils.NewLogger(),
			Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
			drainerAndExecute: func(configurer func(ctx context.Context) bool, drain bool) error {
				_ = configurer(context.TODO())
				return nil
			},
			restartDevicePlugin: func() error {
				return nil
			},
		}

		vrbnc = &vrbv1.SriovVrbNodeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vrbnc",
				Namespace: "default",
			},
		}
		acc = vrbv1.PhysicalFunctionConfigExt{
			PCIAddress:      "0000:00:00.0",
			VrbResourceName: "test-resource",
		}
		currentConfig = make(map[string]interface{})
		resourceList = []interface{}{}
	})

	Describe("loadAndModifyDevicePluginConfig", func() {
		It("should modify the ConfigMap if needed", func() {
			// Create the SriovVrbNodeConfig object in the fake client
			err := reconciler.Client.Create(context.TODO(), vrbnc)
			Expect(err).NotTo(HaveOccurred())

			err = reconciler.loadAndModifyDevicePluginConfig(vrbnc, acc)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("matchPFPCIAddress", func() {
		It("should return true if the PF_PCI_ADDR stored in additional information matches", func() {
			resourceMap := map[string]interface{}{
				"additionalInfo": map[string]interface{}{
					"*": map[string]interface{}{
						"PF_PCI_ADDR": "0000:00:00.0",
					},
				},
			}
			matches := reconciler.matchPFPCIAddress(resourceMap, "0000:00:00.0")
			Expect(matches).To(BeTrue())
		})
	})

	Describe("matchVFDeviceID", func() {
		It("should return true if selectors[\"devices\"] match the VF device ID", func() {
			resourceMap := map[string]interface{}{
				"additionalInfo": map[string]interface{}{
					"*": map[string]interface{}{
						"PF_PCI_ADDR": "0000:00:00.0",
					},
				},
				"selectors": map[string]interface{}{
					"devices": []interface{}{"57c3"},
				},
			}
			matches := reconciler.matchVFDeviceID(resourceMap, "57c3")
			Expect(matches).To(BeTrue())
		})
	})

	Describe("handleResourceNotFound", func() {
		It("should handle the case where a resource was not found", func() {
			// Create a fake ConfigMap
			configMap := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sriovdp-config",
					Namespace: "default",
				},
				Data: map[string]string{
					"config.json": "{}",
				},
			}
			err := reconciler.Client.Create(context.TODO(), configMap)
			Expect(err).NotTo(HaveOccurred())

			// Set the nodeNameRef.Namespace to "default" to match the ConfigMap namespace
			reconciler.nodeNameRef.Namespace = "default"

			vfAddresses := []string{"0000:00:00.1"}
			err = reconciler.handleResourceNotFound(currentConfig, resourceList, "57c3", acc, vfAddresses)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("updateConfigMap", func() {
		It("should update the ConfigMap", func() {
			// Create a fake ConfigMap
			configMap := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sriovdp-config",
					Namespace: "default",
				},
				Data: map[string]string{
					"config.json": "{}",
				},
			}
			err := reconciler.Client.Create(context.TODO(), configMap)
			Expect(err).NotTo(HaveOccurred())

			// Prepare the new config
			newConfig := make(map[string]interface{})
			newConfig["resourceList"] = []interface{}{
				map[string]interface{}{
					"resourceName": "new-resource",
					"selectors": map[string]interface{}{
						"devices": []interface{}{"57c3"},
					},
				},
			}

			// Set the nodeNameRef.Namespace to "default" to match the ConfigMap namespace
			reconciler.nodeNameRef.Namespace = "default"

			// Run the updateConfigMap function
			err = reconciler.updateConfigMap(newConfig, "new-resource")
			Expect(err).NotTo(HaveOccurred())

			// Verify the ConfigMap was updated
			updatedConfigMap := &v1.ConfigMap{}
			err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: "sriovdp-config", Namespace: "default"}, updatedConfigMap)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedConfigMap.Data["config.json"]).To(ContainSubstring("new-resource"))
		})
	})

	Describe("modifyResource", func() {
		It("should modify the resource", func() {
			resourceMap := map[string]interface{}{
				"additionalInfo": map[string]interface{}{
					"*": map[string]interface{}{
						"PF_PCI_ADDR": "0000:00:00.0",
					},
				},
				"selectors": map[string]interface{}{
					"devices": []interface{}{"57c3"},
				},
			}
			vfAddresses := []string{"0000:00:00.1"}
			modified := reconciler.modifyResource(resourceMap, "new-resource", "0000:00:00.0", vfAddresses)
			Expect(modified).To(BeTrue())
		})
	})

	Describe("loadCurrentDevicePluginConfig", func() {
		It("should load the current device plugin config", func() {
			// Create a fake ConfigMap
			configMap := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sriovdp-config",
					Namespace: "default",
				},
				Data: map[string]string{
					"config.json": `{"resourceList": [{"resourceName": "test-resource"}]}`,
				},
			}
			err := reconciler.Client.Create(context.TODO(), configMap)
			Expect(err).NotTo(HaveOccurred())

			// Set the nodeNameRef.Namespace to "default" to match the ConfigMap namespace
			reconciler.nodeNameRef.Namespace = "default"

			// Run the loadCurrentDevicePluginConfig function
			ctx := context.TODO()
			config, err := reconciler.loadCurrentDevicePluginConfig(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(config).NotTo(BeNil())
			Expect(config["resourceList"]).To(HaveLen(1))
			resourceList := config["resourceList"].([]interface{})
			resourceMap := resourceList[0].(map[string]interface{})
			Expect(resourceMap["resourceName"]).To(Equal("test-resource"))
		})
	})
})
