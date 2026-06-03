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
			/* Reset package-level globals so each test starts from a clean state. */
			for k := range fecPreviousConfig {
				delete(fecPreviousConfig, k)
			}
			for k := range fecCurrentConfig {
				delete(fecCurrentConfig, k)
			}
			for k := range fecDeviceUpdateRequired {
				delete(fecDeviceUpdateRequired, k)
			}
			getSriovInventory = GetSriovInventory
			procCmdlineFilePath = "testdata/cmdline_test"
			sysLockdownFilePath = "testdata/lockdown_none"
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

		AfterEach(func() {
			for k := range fecPreviousConfig {
				delete(fecPreviousConfig, k)
			}
			for k := range fecCurrentConfig {
				delete(fecCurrentConfig, k)
			}
			for k := range fecDeviceUpdateRequired {
				delete(fecDeviceUpdateRequired, k)
			}
			getSriovInventory = GetSriovInventory
			procCmdlineFilePath = "/proc/cmdline"
			sysLockdownFilePath = "/sys/kernel/security/lockdown"
		})

		It("restores/recreates VFs when sriov_numvfs is reset externally", func() {
			// Verifies that when VFs disappear from sysfs without any Kubernetes
			// spec change, the reconciler still calls configureAccelerator to
			// restore them.  Uses a gated configurer that honours the
			// fecDeviceUpdateRequired flag (filters spec, not live inventory).

			configureCallCount := 0
			gatedInventory := nodeInventory
			gatedConfigurer := testGatedConfigurerProto{
				configureNodeFunction: func(nodeConfig sriovv2.SriovFecNodeConfigSpec) error {
					configureCallCount++
					for _, pf := range nodeConfig.PhysicalFunctions {
						for i, acc := range gatedInventory.SriovAccelerators {
							if acc.PCIAddress != pf.PCIAddress {
								continue
							}
							gatedInventory.SriovAccelerators[i].VFs = []sriovv2.VF{}
							for j := 0; j < pf.VFAmount; j++ {
								gatedInventory.SriovAccelerators[i].VFs = append(
									gatedInventory.SriovAccelerators[i].VFs,
									sriovv2.VF{
										PCIAddress: fmt.Sprintf("%s%d",
											pf.PCIAddress[0:len(pf.PCIAddress)-1], j+1),
										Driver:   "vfDriver",
										DeviceID: "deviceId",
									})
							}
						}
					}
					return nil
				},
			}
			reconciler.sriovfecconfigurer = gatedConfigurer
			getSriovInventory = func(log *logrus.Logger) (*sriovv2.NodeInventory, error) {
				return gatedInventory, nil
			}

			// Reconcile 1: creates empty SriovFecNodeConfig (no spec yet)
			_, err := reconciler.Reconcile(context.TODO(), reconcileRequestes)
			Expect(err).ToNot(HaveOccurred())

			// Set spec requesting 1 VF — bump generation so reconciler sees a change
			sfnc := new(sriovv2.SriovFecNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, sfnc)).ToNot(HaveOccurred())
			sfnc.Generation++
			sfnc.Spec = sriovv2.SriovFecNodeConfigSpec{
				PhysicalFunctions: []sriovv2.PhysicalFunctionConfigExt{
					{
						PCIAddress: pciAddress,
						/* igb_uio is used here instead of the production default
						 * vfio-pci to avoid validateNodeConfig reading the real
						 * /sys/module/vfio_pci/parameters/enable_sriov, which
						 * may not be set on CI hosts.  The driver value does not
						 * affect the VF-restore logic under test.
						 */
						PFDriver:    utils.IgbUio,
						VFDriver:    utils.IgbUio,
						VFAmount:    1,
						BBDevConfig: sriovv2.BBDevConfig{},
					},
				},
			}
			Expect(fakeClient.Patch(context.TODO(), sfnc, client.Merge)).ToNot(HaveOccurred())

			// Reconcile 2: spec changed (generation bump) → configureAccelerator called,
			// VFs created, fecPreviousConfig populated with current spec
			configureCallCount = 0
			_, err = reconciler.Reconcile(context.TODO(), reconcileRequestes)
			Expect(err).ToNot(HaveOccurred())
			Expect(configureCallCount).To(Equal(1), "reconcile 2 must call configureAccelerator")

			sfnc = new(sriovv2.SriovFecNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, sfnc)).ToNot(HaveOccurred())
			Expect(sfnc.Status.Inventory.SriovAccelerators[0].VFs).To(HaveLen(1),
				"VFs should be present after reconcile 2")

			// Simulate external sriov_numvfs reset (e.g. StarlingX sysinv-agent or
			// pf_bb_config daemon exit) — VFs disappear from sysfs without any
			// Kubernetes spec change
			for i := range gatedInventory.SriovAccelerators {
				gatedInventory.SriovAccelerators[i].VFs = []sriovv2.VF{}
			}

			// Reconcile 3: spec UNCHANGED, VFs missing.
			// isCardUpdateRequired returns true (exposedVfs=0 != requestedVfs=1).
			// checkIfDeviceUpdateNeeded sees prev==curr so it clears
			// fecDeviceUpdateRequired; the hardware mismatch override must
			// re-set the flag so configureAccelerator is called.
			configureCallCount = 0
			_, err = reconciler.Reconcile(context.TODO(), reconcileRequestes)
			Expect(err).ToNot(HaveOccurred())
			Expect(configureCallCount).To(Equal(1),
				"reconcile 3 must call configureAccelerator to restore externally-cleared VFs")

			sfnc = new(sriovv2.SriovFecNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, sfnc)).ToNot(HaveOccurred())
			Expect(sfnc.Status.Inventory.SriovAccelerators[0].VFs).To(HaveLen(1),
				"VFs must be restored after reconcile 3")
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

/* testGatedConfigurerProto honours the fecDeviceUpdateRequired gate by
 * skipping PCI addresses where the flag is false and only invoking the
 * configure function for those where it is true.  Unlike testConfigurerProto
 * it does not bypass the gate, so tests can verify that
 * fecDeviceUpdateRequired is set before configureAccelerator is called.
 * Note: it filters the spec rather than iterating the live inventory as
 * the production NodeConfigurator.ApplySpec does.
 */
type testGatedConfigurerProto struct {
	configureNodeFunction func(nodeConfig sriovv2.SriovFecNodeConfigSpec) error
}

func (t testGatedConfigurerProto) ApplySpec(nodeConfig sriovv2.SriovFecNodeConfigSpec, fecDeviceUpdateRequired map[string]bool) error {
	filtered := sriovv2.SriovFecNodeConfigSpec{}
	for _, pf := range nodeConfig.PhysicalFunctions {
		if fecDeviceUpdateRequired[pf.PCIAddress] {
			filtered.PhysicalFunctions = append(filtered.PhysicalFunctions, pf)
		}
	}
	if len(filtered.PhysicalFunctions) == 0 {
		return nil
	}
	return t.configureNodeFunction(filtered)
}

func (t testGatedConfigurerProto) VrbApplySpec(nodeConfig vrbv1.SriovVrbNodeConfigSpec, vrbDeviceUpdateRequired map[string]bool) error {
	return nil
}

/* testGatedVrbConfigurerProto honours the vrbDeviceUpdateRequired gate by
 * skipping PCI addresses where the flag is false and only invoking the
 * configure function for those where it is true.  Unlike testConfigurerProto
 * it does not bypass the gate, so tests can verify that
 * vrbDeviceUpdateRequired is set before VrbApplySpec is called.
 * Note: it filters the spec rather than iterating the live inventory as
 * the production NodeConfigurator.VrbApplySpec does.
 */
type testGatedVrbConfigurerProto struct {
	vrbConfigureNodeFunction func(nodeConfig vrbv1.SriovVrbNodeConfigSpec) error
}

func (t testGatedVrbConfigurerProto) VrbApplySpec(nodeConfig vrbv1.SriovVrbNodeConfigSpec, vrbDeviceUpdateRequired map[string]bool) error {
	filtered := vrbv1.SriovVrbNodeConfigSpec{}
	for _, pf := range nodeConfig.PhysicalFunctions {
		if vrbDeviceUpdateRequired[pf.PCIAddress] {
			filtered.PhysicalFunctions = append(filtered.PhysicalFunctions, pf)
		}
	}
	if len(filtered.PhysicalFunctions) == 0 {
		return nil
	}
	return t.vrbConfigureNodeFunction(filtered)
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
			/* Reset package-level globals so each test starts from a clean state. */
			for k := range vrbPreviousConfig {
				delete(vrbPreviousConfig, k)
			}
			for k := range vrbCurrentConfig {
				delete(vrbCurrentConfig, k)
			}
			for k := range vrbDeviceUpdateRequired {
				delete(vrbDeviceUpdateRequired, k)
			}
			VrbgetSriovInventory = VrbGetSriovInventory
			procCmdlineFilePath = "testdata/cmdline_test"
			sysLockdownFilePath = "testdata/lockdown_none"
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

		AfterEach(func() {
			for k := range vrbPreviousConfig {
				delete(vrbPreviousConfig, k)
			}
			for k := range vrbCurrentConfig {
				delete(vrbCurrentConfig, k)
			}
			for k := range vrbDeviceUpdateRequired {
				delete(vrbDeviceUpdateRequired, k)
			}
			VrbgetSriovInventory = VrbGetSriovInventory
			procCmdlineFilePath = "/proc/cmdline"
			sysLockdownFilePath = "/sys/kernel/security/lockdown"
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

		It("restores/recreates VFs when sriov_numvfs is reset externally", func() {
			// Verifies that when VFs disappear from sysfs without any Kubernetes
			// spec change, the reconciler still calls VrbApplySpec to restore
			// them.  Uses a gated configurer that honours the
			// vrbDeviceUpdateRequired flag (filters spec, not live inventory).

			configureCallCount := 0
			gatedInventory := nodeInventory
			gatedConfigurer := testGatedVrbConfigurerProto{
				vrbConfigureNodeFunction: func(nodeConfig vrbv1.SriovVrbNodeConfigSpec) error {
					configureCallCount++
					for _, pf := range nodeConfig.PhysicalFunctions {
						for i, acc := range gatedInventory.SriovAccelerators {
							if acc.PCIAddress != pf.PCIAddress {
								continue
							}
							gatedInventory.SriovAccelerators[i].VFs = []vrbv1.VF{}
							for j := 0; j < pf.VFAmount; j++ {
								gatedInventory.SriovAccelerators[i].VFs = append(
									gatedInventory.SriovAccelerators[i].VFs,
									vrbv1.VF{
										PCIAddress: fmt.Sprintf("%s%d",
											pf.PCIAddress[0:len(pf.PCIAddress)-1], j+1),
										Driver:   "vfDriver",
										DeviceID: "deviceId",
									})
							}
						}
					}
					return nil
				},
			}
			reconciler.vrbconfigurer = gatedConfigurer
			VrbgetSriovInventory = func(log *logrus.Logger) (*vrbv1.NodeInventory, error) {
				return gatedInventory, nil
			}

			// Reconcile 1: creates empty SriovVrbNodeConfig (no spec yet)
			_, err := reconciler.Reconcile(context.TODO(), reconcileRequestes)
			Expect(err).ToNot(HaveOccurred())

			// Set spec requesting 1 VF — bump generation so reconciler sees a change
			svnc := new(vrbv1.SriovVrbNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, svnc)).ToNot(HaveOccurred())
			svnc.Generation++
			svnc.Spec = vrbv1.SriovVrbNodeConfigSpec{
				PhysicalFunctions: []vrbv1.PhysicalFunctionConfigExt{
					{
						PCIAddress: pciAddress,
						/* igb_uio is used here instead of the production default
						 * vfio-pci to avoid validateVrbNodeConfig reading the
						 * real /sys/module/vfio_pci/parameters/enable_sriov,
						 * which may not be set on CI hosts.  The driver value
						 * does not affect the VF-restore logic under test.
						 */
						PFDriver:    utils.IgbUio,
						VFDriver:    utils.IgbUio,
						VFAmount:    1,
						BBDevConfig: vrbv1.BBDevConfig{},
					},
				},
			}
			Expect(fakeClient.Patch(context.TODO(), svnc, client.Merge)).ToNot(HaveOccurred())

			// Reconcile 2: spec changed (generation bump) → VrbApplySpec called,
			// VFs created, vrbPreviousConfig populated with current spec
			configureCallCount = 0
			_, err = reconciler.Reconcile(context.TODO(), reconcileRequestes)
			Expect(err).ToNot(HaveOccurred())
			Expect(configureCallCount).To(Equal(1), "reconcile 2 must call VrbApplySpec")

			svnc = new(vrbv1.SriovVrbNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, svnc)).ToNot(HaveOccurred())
			Expect(svnc.Status.Inventory.SriovAccelerators[0].VFs).To(HaveLen(1),
				"VFs should be present after reconcile 2")

			// Simulate external sriov_numvfs reset (e.g. StarlingX sysinv-agent or
			// pf_bb_config daemon exit) — VFs disappear from sysfs without any
			// Kubernetes spec change
			for i := range gatedInventory.SriovAccelerators {
				gatedInventory.SriovAccelerators[i].VFs = []vrbv1.VF{}
			}

			// Reconcile 3: spec UNCHANGED, VFs missing.
			// isCardUpdateRequired returns true (exposedVfs=0 != requestedVfs=1).
			// checkIfDeviceUpdateNeeded sees prev==curr so it clears
			// vrbDeviceUpdateRequired; the hardware mismatch override must
			// re-set the flag so VrbApplySpec is called.
			configureCallCount = 0
			_, err = reconciler.Reconcile(context.TODO(), reconcileRequestes)
			Expect(err).ToNot(HaveOccurred())
			Expect(configureCallCount).To(Equal(1),
				"reconcile 3 must call VrbApplySpec to restore externally-cleared VFs")

			svnc = new(vrbv1.SriovVrbNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, svnc)).ToNot(HaveOccurred())
			Expect(svnc.Status.Inventory.SriovAccelerators[0].VFs).To(HaveLen(1),
				"VFs must be restored after reconcile 3")
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
			config, err := reconciler.loadCurrentDevicePluginConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(config).NotTo(BeNil())
			Expect(config["resourceList"]).To(HaveLen(1))
			resourceList := config["resourceList"].([]interface{})
			resourceMap := resourceList[0].(map[string]interface{})
			Expect(resourceMap["resourceName"]).To(Equal("test-resource"))
		})
	})
})
