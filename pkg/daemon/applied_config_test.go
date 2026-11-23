// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2026 Intel Corporation

package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	sriovv2 "github.com/intel/sriov-fec-operator/api/sriovfec/v2"
	vrbv1 "github.com/intel/sriov-fec-operator/api/sriovvrb/v1"
	"github.com/intel/sriov-fec-operator/pkg/common/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("planPFAction", func() {
	alive := func() procState { return procAlive }
	dead := func() procState { return procDead }
	req := &pfRequest{pfDriver: utils.VfioPci, vfDriver: utils.VfioPci, vfAmount: 2, hash: "h1"}
	configured := pfDeviceState{pfDriver: "vfio-pci", vfDrivers: []string{"vfio-pci", "vfio-pci"}}

	DescribeTable("decides from the record, the spec and the device",
		func(r *pfRequest, applied string, state pfDeviceState, stale bool, proc func() procState, want pfAction) {
			got, why := planPFAction(r, applied, state, stale, proc)
			Expect(got).To(Equal(want), why)
		},
		Entry("configured and running: nothing to do", req, "h1", configured, false, alive, pfNoAction),
		Entry("daemon restarted: only pf_bb_config", req, "h1", configured, false, dead, pfRestartPfBBConfig),
		Entry("pf_bb_config state unknown: nothing to do", req, "h1", configured, false, func() procState { return procUnknown }, pfNoAction),
		Entry("no record", req, "", configured, false, alive, pfReconfigure),
		Entry("spec changed", req, "h0", configured, false, alive, pfReconfigure),
		Entry("VFs missing", req, "h1", pfDeviceState{pfDriver: "vfio-pci"}, false, alive, pfReconfigure),
		Entry("VF unbind pending (no driver)", req, "h1", pfDeviceState{pfDriver: "vfio-pci", vfDrivers: []string{"", "vfio-pci"}}, false, alive, pfReconfigure),
		Entry("PF driver differs", req, "h1", pfDeviceState{pfDriver: "igb_uio", vfDrivers: []string{"vfio-pci", "vfio-pci"}}, false, alive, pfReconfigure),
		Entry("stale write from a previous instance", req, "h1", configured, true, alive, pfReconfigure),
		Entry("not in the spec, with VFs", nil, "", configured, false, alive, pfReconfigure),
		Entry("not in the spec, no VFs", nil, "h1", pfDeviceState{pfDriver: "vfio-pci"}, false, alive, pfNoAction),
		Entry("pci-pf-stub and pci_pf_stub are the same driver",
			&pfRequest{pfDriver: utils.PciPfStubUnderscore, vfDriver: utils.IgbUio, vfAmount: 1, hash: "h1"}, "h1",
			pfDeviceState{pfDriver: "pci-pf-stub", vfDrivers: []string{"igb_uio"}}, false, dead, pfNoAction),
	)

	It("does not look at pf_bb_config when the PF needs reconfiguring anyway", func() {
		asked := false
		action, _ := planPFAction(req, "", configured, false, func() procState { asked = true; return procDead })
		Expect(action).To(Equal(pfReconfigure))
		Expect(asked).To(BeFalse())
	})
})

var _ = Describe("applied configuration records", func() {
	It("round-trips through an annotation and keeps other annotations", func() {
		scheme := runtime.NewScheme()
		Expect(sriovv2.AddToScheme(scheme)).To(Succeed())
		nc := &sriovv2.SriovFecNodeConfig{ObjectMeta: metav1.ObjectMeta{
			Name: "worker", Namespace: "ns", Annotations: map[string]string{"other": "kept"},
		}}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nc).Build()

		applied, present := readAppliedConfigs(nc, fecAppliedConfigAnnotation)
		Expect(present).To(BeFalse())
		Expect(applied).To(BeEmpty())

		Expect(storeAppliedConfigs(context.TODO(), c, nc, fecAppliedConfigAnnotation, appliedConfigs{"0000:b8:00.0": "h1"})).To(Succeed())

		stored := &sriovv2.SriovFecNodeConfig{}
		Expect(c.Get(context.TODO(), types.NamespacedName{Name: "worker", Namespace: "ns"}, stored)).To(Succeed())
		applied, present = readAppliedConfigs(stored, fecAppliedConfigAnnotation)
		Expect(present).To(BeTrue())
		Expect(applied).To(Equal(appliedConfigs{"0000:b8:00.0": "h1"}))
		Expect(stored.Annotations["other"]).To(Equal("kept"))
	})

	It("treats a malformed record as present and empty", func() {
		nc := &sriovv2.SriovFecNodeConfig{ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{fecAppliedConfigAnnotation: "{"},
		}}
		applied, present := readAppliedConfigs(nc, fecAppliedConfigAnnotation)
		Expect(present).To(BeTrue())
		Expect(applied).To(BeEmpty())
	})

	It("hashes configurations by content", func() {
		a := sriovv2.PhysicalFunctionConfigExt{PCIAddress: "0000:b8:00.0", PFDriver: utils.VfioPci, VFAmount: 2}
		b := a
		Expect(configHash(a)).To(Equal(configHash(b)))
		b.VFAmount = 3
		Expect(configHash(a)).ToNot(Equal(configHash(b)))
	})
})

var _ = Describe("FecNodeConfigReconciler across daemon restarts", func() {
	const pf = "0000:b8:00.0"

	var (
		scheme      *runtime.Scheme
		fakeClient  client.Client
		nodeNameRef types.NamespacedName
		request     ctrl.Request
		inventory   *sriovv2.NodeInventory
		applyCalls  int
		restarts    int
		applyErr    error
		pgrepOut    string

		tmp                                                   string
		origInventory                                         func(*logrus.Logger) (*sriovv2.NodeInventory, error)
		origPgrep                                             func(*logrus.Logger, string) (string, error)
		origCmdline, origLockdown, origModuleDir, origWorkdir string
	)

	spec := sriovv2.SriovFecNodeConfigSpec{
		DrainSkip: true,
		PhysicalFunctions: []sriovv2.PhysicalFunctionConfigExt{
			{PCIAddress: pf, PFDriver: utils.VfioPci, VFDriver: utils.VfioPci, VFAmount: 2},
		},
	}

	configuredInventory := func() *sriovv2.NodeInventory {
		return &sriovv2.NodeInventory{SriovAccelerators: []sriovv2.SriovAccelerator{{
			VendorID: "8086", DeviceID: "0d8f", PCIAddress: pf, PFDriver: "vfio-pci", MaxVFs: 8,
			VFs: []sriovv2.VF{
				{PCIAddress: "0000:b8:00.1", Driver: "vfio-pci", DeviceID: "0d90"},
				{PCIAddress: "0000:b8:00.2", Driver: "vfio-pci", DeviceID: "0d90"},
			},
		}}}
	}

	newReconciler := func() *FecNodeConfigReconciler {
		return &FecNodeConfigReconciler{
			Client:      fakeClient,
			log:         utils.NewLogger(),
			nodeNameRef: nodeNameRef,
			sriovfecconfigurer: countingConfigurer{
				apply: func() error {
					applyCalls++
					if applyErr != nil {
						return applyErr
					}
					inventory = configuredInventory()
					return nil
				},
				restart: func() error { restarts++; return nil },
			},
			drainerAndExecute: func(configurer func(ctx context.Context) bool, drain bool) error {
				_ = configurer(context.TODO())
				return nil
			},
			restartDevicePlugin: func() error { return nil },
		}
	}

	getNodeConfig := func() *sriovv2.SriovFecNodeConfig {
		nc := &sriovv2.SriovFecNodeConfig{}
		Expect(fakeClient.Get(context.TODO(), nodeNameRef, nc)).To(Succeed())
		return nc
	}

	condition := func() metav1.Condition {
		return findOrCreateConfigurationStatusCondition(getNodeConfig())
	}

	setSpec := func(s sriovv2.SriovFecNodeConfigSpec) {
		nc := getNodeConfig()
		nc.Generation++
		nc.Spec = s
		Expect(fakeClient.Patch(context.TODO(), nc, client.Merge)).To(Succeed())
	}

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(sriovv2.AddToScheme(scheme)).To(Succeed())
		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		nodeNameRef = types.NamespacedName{Name: "worker", Namespace: "testNamespace"}
		request = ctrl.Request{NamespacedName: nodeNameRef}
		applyCalls, restarts, applyErr, pgrepOut = 0, 0, nil, "1"

		var err error
		tmp, err = os.MkdirTemp("", "applied-config")
		Expect(err).ToNot(HaveOccurred())
		params := filepath.Join(tmp, "module", utils.VfioPciUnderscore, "parameters")
		Expect(os.MkdirAll(params, 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(params, "enable_sriov"), []byte("Y\n"), 0644)).To(Succeed())

		origInventory, origPgrep = getSriovInventory, runPgrepCount
		origCmdline, origLockdown, origModuleDir, origWorkdir = procCmdlineFilePath, sysLockdownFilePath, sysModuleDir, workdir
		procCmdlineFilePath = "testdata/cmdline_test"
		sysLockdownFilePath = "testdata/lockdown_none"
		sysModuleDir = filepath.Join(tmp, "module")
		workdir = tmp

		inventory = &sriovv2.NodeInventory{SriovAccelerators: []sriovv2.SriovAccelerator{{
			VendorID: "8086", DeviceID: "0d8f", PCIAddress: pf, PFDriver: "vfio-pci", MaxVFs: 8,
		}}}
		getSriovInventory = func(*logrus.Logger) (*sriovv2.NodeInventory, error) { return inventory, nil }
		runPgrepCount = func(*logrus.Logger, string) (string, error) { return pgrepOut, nil }

		// first start: create the node config and apply the spec
		_, err = newReconciler().Reconcile(context.TODO(), request)
		Expect(err).ToNot(HaveOccurred())
		setSpec(spec)
		_, err = newReconciler().Reconcile(context.TODO(), request)
		Expect(err).ToNot(HaveOccurred())
		Expect(applyCalls).To(Equal(1))
		Expect(condition().Reason).To(Equal(string(ConfigurationSucceeded)))
		applyCalls = 0
	})

	AfterEach(func() {
		getSriovInventory, runPgrepCount = origInventory, origPgrep
		procCmdlineFilePath, sysLockdownFilePath, sysModuleDir, workdir = origCmdline, origLockdown, origModuleDir, origWorkdir
		Expect(os.RemoveAll(tmp)).To(Succeed())
	})

	It("only starts pf_bb_config again after a restart, and leaves the PF alone otherwise", func() {
		// the daemon restarts: pf_bb_config is gone, the PF and VFs are configured
		pgrepOut = "0"
		_, err := newReconciler().Reconcile(context.TODO(), request)
		Expect(err).ToNot(HaveOccurred())
		Expect(applyCalls).To(Equal(0), "the PF must not be torn down")
		Expect(restarts).To(Equal(1))
		Expect(condition().Reason).To(Equal(string(ConfigurationSucceeded)))

		pgrepOut = "1"
		_, err = newReconciler().Reconcile(context.TODO(), request)
		Expect(err).ToNot(HaveOccurred())
		Expect(applyCalls).To(Equal(0))
		Expect(restarts).To(Equal(1))
	})

	It("adopts the configuration reported by the status when there is no record (upgrade)", func() {
		nc := getNodeConfig()
		delete(nc.Annotations, fecAppliedConfigAnnotation)
		Expect(fakeClient.Update(context.TODO(), nc)).To(Succeed())

		_, err := newReconciler().Reconcile(context.TODO(), request)
		Expect(err).ToNot(HaveOccurred())
		Expect(applyCalls).To(Equal(0))
		applied, present := readAppliedConfigs(getNodeConfig(), fecAppliedConfigAnnotation)
		Expect(present).To(BeTrue())
		Expect(applied).To(HaveKey(pf))
	})

	It("reconfigures a PF whose teardown failed, even when the spec goes back to the applied one", func() {
		// remove the PF from the spec; the teardown starts and fails
		applyErr = errors.New("failed to set new amount of VFs")
		setSpec(sriovv2.SriovFecNodeConfigSpec{DrainSkip: true})
		result, err := newReconciler().Reconcile(context.TODO(), request)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeTrue())
		Expect(applyCalls).To(Equal(1))
		Expect(condition().Reason).To(Equal(string(ConfigurationFailed)))
		applied, _ := readAppliedConfigs(getNodeConfig(), fecAppliedConfigAnnotation)
		Expect(applied).ToNot(HaveKey(pf))

		// the same spec as before is applied again: the PF is not trusted
		applyErr, applyCalls = nil, 0
		setSpec(spec)
		_, err = newReconciler().Reconcile(context.TODO(), request)
		Expect(err).ToNot(HaveOccurred())
		Expect(applyCalls).To(Equal(1))
		Expect(condition().Reason).To(Equal(string(ConfigurationSucceeded)))
	})

	It("keeps the record when the reconfiguration is refused because a VF is in use", func() {
		applyErr = &DeviceInUseError{PF: pf, Users: map[string][]string{"0000:b8:00.1": {"4242 (dpdk-app)"}}}
		changed := spec
		changed.PhysicalFunctions = []sriovv2.PhysicalFunctionConfigExt{spec.PhysicalFunctions[0]}
		changed.PhysicalFunctions[0].VFAmount = 4
		setSpec(changed)
		_, err := newReconciler().Reconcile(context.TODO(), request)
		Expect(err).ToNot(HaveOccurred())
		Expect(condition().Reason).To(Equal(string(ConfigurationDeviceInUse)))

		// going back to the applied spec needs nothing: the PF was never touched
		applyErr, applyCalls = nil, 0
		setSpec(spec)
		_, err = newReconciler().Reconcile(context.TODO(), request)
		Expect(err).ToNot(HaveOccurred())
		Expect(applyCalls).To(Equal(0))
		Expect(condition().Reason).To(Equal(string(ConfigurationSucceeded)))
		Expect(condition().ObservedGeneration).To(Equal(getNodeConfig().Generation))
	})
})

// countingConfigurer counts calls to the Configurer methods.
type countingConfigurer struct {
	apply   func() error
	restart func() error
}

func (c countingConfigurer) ApplySpec(sriovv2.SriovFecNodeConfigSpec, map[string]bool) error {
	return c.apply()
}

func (c countingConfigurer) RestartPfBBConfig(sriovv2.SriovAccelerator, *sriovv2.PhysicalFunctionConfigExt) error {
	return c.restart()
}

var _ = Describe("VrbNodeConfigReconciler across daemon restarts", func() {
	It("only starts pf_bb_config again after a restart", func() {
		const pf = "0000:07:00.0"
		scheme := runtime.NewScheme()
		Expect(vrbv1.AddToScheme(scheme)).To(Succeed())
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		nodeNameRef := types.NamespacedName{Name: "worker", Namespace: "testNamespace"}
		request := ctrl.Request{NamespacedName: nodeNameRef}

		tmp, err := os.MkdirTemp("", "applied-config-vrb")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(tmp)
		params := filepath.Join(tmp, "module", utils.VfioPciUnderscore, "parameters")
		Expect(os.MkdirAll(params, 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(params, "enable_sriov"), []byte("Y\n"), 0644)).To(Succeed())

		origInventory, origPgrep := VrbgetSriovInventory, runPgrepCount
		origCmdline, origLockdown, origModuleDir, origWorkdir := procCmdlineFilePath, sysLockdownFilePath, sysModuleDir, workdir
		defer func() {
			VrbgetSriovInventory, runPgrepCount = origInventory, origPgrep
			procCmdlineFilePath, sysLockdownFilePath, sysModuleDir, workdir = origCmdline, origLockdown, origModuleDir, origWorkdir
		}()
		procCmdlineFilePath = "testdata/cmdline_test"
		sysLockdownFilePath = "testdata/lockdown_none"
		sysModuleDir = filepath.Join(tmp, "module")
		workdir = tmp

		inventory := &vrbv1.NodeInventory{SriovAccelerators: []vrbv1.SriovAccelerator{{
			VendorID: "8086", DeviceID: "57c2", PCIAddress: pf, PFDriver: "vfio-pci", MaxVFs: 64,
		}}}
		VrbgetSriovInventory = func(*logrus.Logger) (*vrbv1.NodeInventory, error) { return inventory, nil }
		pgrepOut := "1"
		runPgrepCount = func(*logrus.Logger, string) (string, error) { return pgrepOut, nil }

		applyCalls, restarts := 0, 0
		newReconciler := func() *VrbNodeConfigReconciler {
			return &VrbNodeConfigReconciler{
				Client:      fakeClient,
				log:         utils.NewLogger(),
				nodeNameRef: nodeNameRef,
				vrbconfigurer: countingVrbConfigurer{
					apply: func() error {
						applyCalls++
						inventory = &vrbv1.NodeInventory{SriovAccelerators: []vrbv1.SriovAccelerator{{
							VendorID: "8086", DeviceID: "57c2", PCIAddress: pf, PFDriver: "vfio-pci", MaxVFs: 64,
							VFs: []vrbv1.VF{
								{PCIAddress: "0000:07:00.1", Driver: "vfio-pci", DeviceID: "57c3"},
								{PCIAddress: "0000:07:00.2", Driver: "vfio-pci", DeviceID: "57c3"},
							},
						}}}
						return nil
					},
					restart: func() error { restarts++; return nil },
				},
				drainerAndExecute: func(configurer func(ctx context.Context) bool, drain bool) error {
					_ = configurer(context.TODO())
					return nil
				},
				restartDevicePlugin: func() error { return nil },
			}
		}

		_, err = newReconciler().Reconcile(context.TODO(), request)
		Expect(err).ToNot(HaveOccurred())
		nc := &vrbv1.SriovVrbNodeConfig{}
		Expect(fakeClient.Get(context.TODO(), nodeNameRef, nc)).To(Succeed())
		nc.Generation++
		nc.Spec = vrbv1.SriovVrbNodeConfigSpec{DrainSkip: true, PhysicalFunctions: []vrbv1.PhysicalFunctionConfigExt{
			{PCIAddress: pf, PFDriver: utils.VfioPci, VFDriver: utils.VfioPci, VFAmount: 2},
		}}
		Expect(fakeClient.Patch(context.TODO(), nc, client.Merge)).To(Succeed())

		_, err = newReconciler().Reconcile(context.TODO(), request)
		Expect(err).ToNot(HaveOccurred())
		Expect(applyCalls).To(Equal(1))

		// the daemon restarts: pf_bb_config is gone, the PF and VFs are configured
		pgrepOut = "0"
		_, err = newReconciler().Reconcile(context.TODO(), request)
		Expect(err).ToNot(HaveOccurred())
		Expect(applyCalls).To(Equal(1), "the PF must not be torn down")
		Expect(restarts).To(Equal(1))

		Expect(fakeClient.Get(context.TODO(), nodeNameRef, nc)).To(Succeed())
		Expect(VrbfindOrCreateConfigurationStatusCondition(nc).Reason).To(Equal(string(ConfigurationSucceeded)))
	})
})

// countingVrbConfigurer counts calls to the VrbConfigurer methods.
type countingVrbConfigurer struct {
	apply   func() error
	restart func() error
}

func (c countingVrbConfigurer) VrbApplySpec(vrbv1.SriovVrbNodeConfigSpec, map[string]bool) error {
	return c.apply()
}

func (c countingVrbConfigurer) VrbRestartPfBBConfig(vrbv1.SriovAccelerator, *vrbv1.PhysicalFunctionConfigExt) error {
	return c.restart()
}
