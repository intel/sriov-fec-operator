// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2024 Intel Corporation

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	fuzz "github.com/google/gofuzz"
	"github.com/google/uuid"
	"github.com/intel/sriov-fec-operator/pkg/common/utils"

	sriovv2 "github.com/intel/sriov-fec-operator/api/sriovfec/v2"
	vrbv1 "github.com/intel/sriov-fec-operator/api/sriovvrb/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	pciAddress = "0000:14:00.1"
)

var _ = Describe("FecNodeConfigReconciler", func() {
	const (
		_THIS_NODE_NAME      = "worker"
		_SUPPORTED_NAMESPACE = "default"
	)

	BeforeEach(func() {
		Expect(sriovv2.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
		Expect(vrbv1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	})

	Describe("FecNodeConfigReconciler.Reconcile(...)", func() {
		var (
			data    *TestData
			testEnv *envtest.Environment
		)

		BeforeEach(func() {
			//configure kernel controller
			procCmdlineFilePath = "testdata/cmdline_test"
			FecConfigPath = "testdata/accelerators.json"
			VrbConfigPath = "testdata/accelerators_vrb.json"

			//configure node configurator
			workdir = testTmpFolder
			sysBusPciDevices = testTmpFolder
			sysBusPciDrivers = testTmpFolder
			Expect(createFiles(filepath.Join(sysBusPciDevices, pciAddress), "driver_override", "reset", vfNumFileDefault, vfNumFileIgbUio)).To(Succeed())
			Expect(createFiles(filepath.Join(sysBusPciDrivers, utils.IGB_UIO), "bind")).To(Succeed())
			Expect(createFiles(filepath.Join(sysBusPciDrivers, utils.PCI_PF_STUB_DASH), "bind")).To(Succeed())

			getVFconfigured = func(string) int {
				return 0
			}

			getVFList = func(string) ([]string, error) {
				return nil, nil
			}

			getSriovInventory = func(_ *logrus.Logger) (*sriovv2.NodeInventory, error) {
				return &data.SriovFecNodeConfig.Status.Inventory, nil
			}

			data = new(TestData)
			Expect(readAndUnmarshall("testdata/node_config.json", data)).To(Succeed())
		})

		When("Required SriovFecNodeConfig does not exist and cannot be created", func() {

			var reconciler *FecNodeConfigReconciler

			BeforeEach(func() {
				By("bootstrapping test environment")

				onGetErrorReturningClient := testClient{
					Client: fake.NewClientBuilder().Build(),
					get: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						return errors.NewNotFound(sriovv2.GroupVersion.WithResource("SriovFecNodeConfig").GroupResource(), "cannot get")
					},
					create: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
						return fmt.Errorf("cannot create")
					},
				}

				nodeNameRef := types.NamespacedName{Namespace: _SUPPORTED_NAMESPACE, Name: _THIS_NODE_NAME}

				drainer := func(operation func(ctx context.Context) bool, drain bool) error { return nil }

				var err error
				reconciler, err = FecNewNodeConfigReconciler(&onGetErrorReturningClient, drainer, nodeNameRef, nil, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(reconciler).ToNot(BeNil())
			})

			It("Reconcile(...) returns error", func() {
				_, err := reconciler.Reconcile(context.TODO(), ctrl.Request{NamespacedName: types.NamespacedName{Name: _THIS_NODE_NAME, Namespace: _SUPPORTED_NAMESPACE}})
				Expect(err).To(MatchError(ContainSubstring("cannot create")))
			})
		})

		Context("Positive scenarios", func() {

			Context("Initial/zero SriovFecNodeConfig is existing", func() {
				var k8sClient client.Client

				JustBeforeEach(func() {
					By("bootstrapping test environment")

					testEnv = &envtest.Environment{
						CRDDirectoryPaths:       []string{filepath.Join("..", "..", "config", "crd", "bases")},
						ControlPlaneStopTimeout: time.Second,
					}

					config, err := testEnv.Start()
					Expect(err).To(Succeed())
					Expect(config).ToNot(BeNil())

					k8sClient, err = client.New(config, client.Options{Scheme: scheme.Scheme})
					Expect(err).ToNot(HaveOccurred())
					Expect(k8sClient).ToNot(BeNil())

					nodeNameRef := types.NamespacedName{Namespace: _SUPPORTED_NAMESPACE, Name: _THIS_NODE_NAME}
					pfBBConfigController := NewPfBBConfigController(log, uuid.New().String())
					configurer := NewNodeConfigurator(logrus.New(), pfBBConfigController, k8sClient, nodeNameRef)

					reconciler, err := FecNewNodeConfigReconciler(
						k8sClient,
						func(configure func(ctx context.Context) bool, drain bool) error {
							configure(context.TODO())
							return nil
						},
						nodeNameRef,
						configurer,
						func() error {
							return nil
						})

					Expect(err).ToNot(HaveOccurred())

					k8sManager, err := CreateManager(config, scheme.Scheme, _SUPPORTED_NAMESPACE, 0, 0, log)
					Expect(err).ToNot(HaveOccurred())

					Expect(reconciler.SetupWithManager(k8sManager)).ToNot(HaveOccurred())

					//Required during cordoning & draining
					Expect(k8sClient.Create(context.TODO(), &data.Node)).To(Succeed())

					//initialize empty SriovFecNodeConfig
					Expect(reconciler.CreateEmptyNodeConfigIfNeeded(k8sClient)).To(Succeed())
					go func() {
						Expect(k8sManager.Start(context.TODO())).ToNot(HaveOccurred())
					}()
				})

				JustAfterEach(func() {
					By("tearing down the test environment")
					_ = testEnv.Stop()
				})

				Context("Requested spec/config is correct and refers to existing accelerators", func() {
					It("spec/config should be applied", func() {
						osExecMock := new(runExecCmdMock).
							onCall([]string{"pkill -9 -f pf_bb_config.*0000:14:00.1"}).
							Return("", nil).
							onCall([]string{"modprobe", utils.IGB_UIO}).
							Return("", nil).
							onCall([]string{"modprobe", "v"}).
							Return("", nil).
							onCall([]string{"setpci", "-v", "-s", "0000:14:00.1", "COMMAND=06"}).
							Return("", nil).
							onCall([]string{"/sriov_workdir/pf_bb_config", "FPGA_5GNR", "-c", fmt.Sprintf("%s.ini", filepath.Join(workdir, pciAddress)), "-p", pciAddress}).
							Return("", nil)

						initNodeConfiguratorRunExecCmd(osExecMock.execute)

						nc := new(sriovv2.SriovFecNodeConfig)
						Expect(k8sClient.Get(context.TODO(), data.GetNamespacedName(), nc)).ToNot(HaveOccurred())

						nc.Spec = data.SriovFecNodeConfig.Spec
						Expect(k8sClient.Update(context.TODO(), nc)).To(Succeed())

						Eventually(func() (nc sriovv2.SriovFecNodeConfig, err error) {
							return nc, k8sClient.Get(context.TODO(), data.GetNamespacedName(), &nc)
						}, "1m", "0.5s").
							Should(
								WithTransform(
									func(nc sriovv2.SriovFecNodeConfig) *metav1.Condition {
										return nc.FindCondition(ConditionConfigured)
									}, SatisfyAll(
										Not(BeNil()),
										WithTransform(func(c *metav1.Condition) string { return c.Reason }, Equal(string(ConfigurationSucceeded))),
									),
								),
							)

						Expect(osExecMock.verify()).To(Succeed())
					})
				})
			})
		})

		Context("Negative scenarios", func() {

			Context("Requested config/spec refers to non existing accelerator", func() {
				It("spec/config should not be applied, error info should be exposed over configuration ccondition", func() {

					//existing inventory
					getSriovInventory = func(log *logrus.Logger) (*sriovv2.NodeInventory, error) {
						inventory := data.SriovFecNodeConfig.Status.Inventory
						inventory.SriovAccelerators[0].PCIAddress = "0000:99:00.1"
						return &inventory, nil
					}

					fakeClient := fake.NewClientBuilder().WithObjects(&data.SriovFecNodeConfig).Build()
					nodeNameRef := types.NamespacedName{Namespace: _SUPPORTED_NAMESPACE, Name: _THIS_NODE_NAME}
					reconciler := FecNodeConfigReconciler{Client: fakeClient, log: log, nodeNameRef: nodeNameRef}

					_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{
						Name:      _THIS_NODE_NAME,
						Namespace: _SUPPORTED_NAMESPACE,
					}})
					Expect(err).To(Succeed())

					res := new(sriovv2.SriovFecNodeConfig)
					Expect(fakeClient.Get(context.TODO(), client.ObjectKeyFromObject(&data.SriovFecNodeConfig), res)).To(Succeed())
					Expect(res).To(Not(BeNil()))
					Expect(res.FindCondition(ConditionConfigured)).To(Not(BeNil()))
					Expect(res.FindCondition(ConditionConfigured).Reason).To(Equal(string(ConfigurationFailed)))
					Expect(res.FindCondition(ConditionConfigured).Status).To(Equal(metav1.ConditionFalse))
					Expect(res.FindCondition(ConditionConfigured).Message).To(ContainSubstring("not existing accelerator"))
				})
			})

			Context("Modification of SriovFecNodeConfig not related with this FecNodeConfigReconciler instance", func() {
				var k8sClient client.Client

				BeforeEach(func() {

					By("bootstrapping test environment")

					testEnv = &envtest.Environment{
						CRDDirectoryPaths:       []string{filepath.Join("..", "..", "config", "crd", "bases")},
						ControlPlaneStopTimeout: time.Second,
					}

					config, err := testEnv.Start()
					Expect(err).To(Succeed())
					Expect(config).ToNot(BeNil())

					k8sClient, err = client.New(config, client.Options{Scheme: scheme.Scheme})
					Expect(err).ToNot(HaveOccurred())
					Expect(k8sClient).ToNot(BeNil())

					drainer := func(configure func(ctx context.Context) bool, drain bool) error {
						configure(context.TODO())
						return nil
					}

					nodeNameRef := types.NamespacedName{Namespace: _SUPPORTED_NAMESPACE, Name: _THIS_NODE_NAME}

					nodeReconciler, err := FecNewNodeConfigReconciler(k8sClient, drainer, nodeNameRef, nil, nil)
					Expect(err).ToNot(HaveOccurred())

					reconciler := nodeRecocnilerWrapper{
						FecNodeConfigReconciler: nodeReconciler,
						reconcilingFunc: func(context.Context, reconcile.Request) (reconcile.Result, error) {
							Fail("reconcile function should not be called")
							return reconcile.Result{}, nil
						},
					}

					k8sManager, err := CreateManager(config, scheme.Scheme, _SUPPORTED_NAMESPACE, 0, 0, log)
					Expect(err).ToNot(HaveOccurred())

					Expect(reconciler.SetupWithManager(k8sManager)).ToNot(HaveOccurred())

					go func() {
						Expect(k8sManager.Start(context.TODO())).ToNot(HaveOccurred())
					}()
				})

				When("Modified SriovFecNodeConfig has foreign name(....)", func() {
					It("should not be considered for reconciliation", func() {
						nodeConfigRelatedWithForeignNode := &sriovv2.SriovFecNodeConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "foreign-node",
								Namespace: _SUPPORTED_NAMESPACE,
							},
							Spec: sriovv2.SriovFecNodeConfigSpec{
								PhysicalFunctions: []sriovv2.PhysicalFunctionConfigExt{},
							},
						}

						Expect(k8sClient.Create(context.TODO(), nodeConfigRelatedWithForeignNode)).ToNot(HaveOccurred())

						Consistently(
							func() error {
								return k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(nodeConfigRelatedWithForeignNode), &sriovv2.SriovFecNodeConfig{})
							}, "10s", "1s").ShouldNot(HaveOccurred())
					})
				})

				When("Modified SriovFecNodeConfig has foreign namespace", func() {
					It("should not be considered for reconciliation", func() {

						const _FOREIGN_NAMESPACE = "kube-system"
						nodeConfigRelatedWithForeignNode := &sriovv2.SriovFecNodeConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name:      _THIS_NODE_NAME,
								Namespace: _FOREIGN_NAMESPACE,
							},
							Spec: sriovv2.SriovFecNodeConfigSpec{
								PhysicalFunctions: []sriovv2.PhysicalFunctionConfigExt{},
							},
						}

						Expect(k8sClient.Create(context.TODO(), nodeConfigRelatedWithForeignNode)).ToNot(HaveOccurred())

						Consistently(
							func() error {
								return k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(nodeConfigRelatedWithForeignNode), &sriovv2.SriovFecNodeConfig{})
							}, "10s", "1s").ShouldNot(HaveOccurred())
					})
				})
			})

			When("FecNodeConfigReconciler cannot read inventory", func() {
				It("SriovFecNodeConfig.Status.Condition should expose appropriate info", func() {
					fakeClient := fake.NewClientBuilder().WithObjects(&data.SriovFecNodeConfig).Build()
					nodeNameRef := types.NamespacedName{Namespace: _SUPPORTED_NAMESPACE, Name: _THIS_NODE_NAME}
					reconciler := FecNodeConfigReconciler{Client: fakeClient, log: log, nodeNameRef: nodeNameRef}
					getSriovInventory = func(log *logrus.Logger) (*sriovv2.NodeInventory, error) {
						return nil, fmt.Errorf("cannot read inventory")
					}

					_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{
						Name:      _THIS_NODE_NAME,
						Namespace: _SUPPORTED_NAMESPACE,
					}})
					Expect(err).To(MatchError("cannot read inventory"))
				})
			})
		})
	})

	It("updateStatus() updates ConditionConfigured condition", func() {
		nodeConfig := sriovv2.SriovFecNodeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      _THIS_NODE_NAME,
				Namespace: _SUPPORTED_NAMESPACE,
			},
			Status: sriovv2.SriovFecNodeConfigStatus{},
			Spec: sriovv2.SriovFecNodeConfigSpec{
				PhysicalFunctions: []sriovv2.PhysicalFunctionConfigExt{},
			},
		}

		fakeClient := fake.NewClientBuilder().WithObjects(&nodeConfig).Build()
		reconciler := FecNodeConfigReconciler{Client: fakeClient, log: log}

		Expect(nodeConfig.Status.Conditions).To(BeEmpty())

		Expect(reconciler.updateStatus(&nodeConfig, metav1.ConditionUnknown, ConfigurationNotRequested, "Unknown")).To(Succeed())

		res := new(sriovv2.SriovFecNodeConfig)
		Expect(fakeClient.Get(context.TODO(), client.ObjectKeyFromObject(&nodeConfig), res)).To(Succeed())
		Expect(res.Status.Conditions).To(HaveLen(1))
		Expect(res.FindCondition(ConditionConfigured)).ToNot(BeNil())
		Expect(res.FindCondition(ConditionConfigured).Reason).To(ContainSubstring("NotRequested"), "Condition.Reason")
		Expect(res.FindCondition(ConditionConfigured).Message).To(ContainSubstring("Unknown"), "Condition.Message")
		Expect(res.FindCondition(ConditionConfigured).Status).To(BeEquivalentTo(metav1.ConditionUnknown), "Condition.Status")

		Expect(reconciler.updateStatus(&nodeConfig, metav1.ConditionTrue, ConfigurationSucceeded, string(ConfigurationSucceeded))).To(Succeed())
		res = new(sriovv2.SriovFecNodeConfig)
		Expect(fakeClient.Get(context.TODO(), client.ObjectKeyFromObject(&nodeConfig), res)).To(Succeed())
		Expect(res.Status.Conditions).To(HaveLen(1))
		Expect(res.FindCondition(ConditionConfigured)).ToNot(BeNil())
		Expect(res.FindCondition(ConditionConfigured).Status).To(BeEquivalentTo(metav1.ConditionTrue), "Condition.Status")
		Expect(res.FindCondition(ConditionConfigured).Message).To(ContainSubstring("Succeeded"), "Condition.Message")
		Expect(res.FindCondition(ConditionConfigured).Reason).To(ContainSubstring("Succeeded"), "Condition.Reason")
	})

	Describe("isConfigurationOfNonExistingInventoryRequested()", func() {
		When("requested config refers only to exiting inventory", func() {
			It("error should not be returned", func() {
				requestedConfig := []sriovv2.PhysicalFunctionConfigExt{
					{PCIAddress: "1"}, {PCIAddress: "2"}, {PCIAddress: "3"},
				}

				inventory := sriovv2.NodeInventory{
					SriovAccelerators: []sriovv2.SriovAccelerator{
						{PCIAddress: "1"}, {PCIAddress: "2"}, {PCIAddress: "3"}, {PCIAddress: "4"},
					},
				}
				Expect(isConfigurationOfNonExistingInventoryRequested(requestedConfig, &inventory)).To(BeFalse())
			})
		})

		When("requested config refers to not exiting inventory", func() {
			It("error should be returned", func() {
				requestedConfig := []sriovv2.PhysicalFunctionConfigExt{
					{PCIAddress: "1"}, {PCIAddress: "99"}, {PCIAddress: "3"},
				}

				inventory := sriovv2.NodeInventory{
					SriovAccelerators: []sriovv2.SriovAccelerator{
						{PCIAddress: "1"}, {PCIAddress: "2"}, {PCIAddress: "3"}, {PCIAddress: "4"},
					},
				}
				Expect(isConfigurationOfNonExistingInventoryRequested(requestedConfig, &inventory)).To(BeTrue())
			})
		})

		When("empty config requested", func() {
			It("error should not be returned", func() {
				requestedConfig := []sriovv2.PhysicalFunctionConfigExt{}

				inventory := sriovv2.NodeInventory{
					SriovAccelerators: []sriovv2.SriovAccelerator{
						{PCIAddress: "1"},
						{PCIAddress: "2"},
						{PCIAddress: "3"},
						{PCIAddress: "4"},
					},
				}
				Expect(isConfigurationOfNonExistingInventoryRequested(requestedConfig, &inventory)).To(BeFalse())
			})
		})
	})

})

type nodeRecocnilerWrapper struct {
	*FecNodeConfigReconciler
	reconcilingFunc func(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
}

func (n *nodeRecocnilerWrapper) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return n.reconcilingFunc(ctx, req)
}

type testClient struct {
	client.Client
	get    func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
	create func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
}

func (e *testClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return e.get(ctx, key, obj)
}

func (e *testClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return e.create(ctx, obj, opts...)
}

func createFiles(folderPath string, filesToBeCreated ...string) error {
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		errDir := os.MkdirAll(folderPath, 0777)
		if errDir != nil {
			return err
		}
	}
	for _, name := range filesToBeCreated {
		filePath := filepath.Join(folderPath, name)
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		if err = f.Close(); err != nil {
			return err
		}
	}
	return nil
}

func readAndUnmarshall(filepath string, target interface{}) error {
	bytes, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, target)
}

type TestData struct {
	SriovFecNodeConfig   sriovv2.SriovFecNodeConfig `json:"sriov_fec_node_config"`
	Node                 core.Node                  `json:"node"`
	SriovDevicePluginPod core.Pod                   `json:"sriov_device_plugin_pod"`
}

func (d *TestData) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: d.SriovFecNodeConfig.Namespace,
		Name:      d.SriovFecNodeConfig.Name,
	}
}

func (d *TestData) NodeConfigName() string {
	return d.SriovFecNodeConfig.Name
}

func (d *TestData) NodeConfigNS() string {
	return d.SriovFecNodeConfig.Namespace
}

func initNodeConfiguratorRunExecCmd(f func([]string, *logrus.Logger) (string, error)) {
	runExecCmd = f
}

type runExecCmdMock struct {
	executions []struct {
		expected     []string
		toBeReturned *[]interface{}
	}
	executionCount int
}

func (r *runExecCmdMock) onCall(expected []string) *resultCatcher {
	var tbr []interface{}
	r.executions = append(r.executions, struct {
		expected     []string
		toBeReturned *[]interface{}
	}{expected: expected, toBeReturned: &tbr})

	return &resultCatcher{toBeReturned: &tbr, mock: r}
}

func (r *runExecCmdMock) execute(args []string, l *logrus.Logger) (string, error) {
	l.Info("runExecCmdMock:", "command", args)
	defer func() { r.executionCount++ }()

	if len(r.executions) <= r.executionCount {
		return "", fmt.Errorf("runExecCmdMock has been called too many times")
	}

	if reflect.DeepEqual(args, r.executions[r.executionCount].expected) {
		execution := r.executions[r.executionCount]
		toBeReturned := *execution.toBeReturned

		return toBeReturned[0].(string),
			func() error {
				v := toBeReturned[1]
				if v == nil {
					return nil
				}
				return v.(error)
			}()
	}

	return "", fmt.Errorf(
		"runExecCmdMock has been called with arguments other than expected, expected: %v, actual: %v",
		r.executions[r.executionCount],
		args,
	)
}

func (r *runExecCmdMock) verify() error {
	if r.executionCount != len(r.executions) {
		return fmt.Errorf("runExecCmdMock: exec command was not requested, expected executions(%d), actual executions(%d); expected executions: %v", len(r.executions), r.executionCount, r.executions)
	}
	return nil
}

type resultCatcher struct {
	toBeReturned *[]interface{}
	mock         *runExecCmdMock
}

func (r *resultCatcher) Return(toBeReturned ...interface{}) *runExecCmdMock {
	*r.toBeReturned = toBeReturned
	return r.mock
}

func FuzzIsCardUpdateRequired(f *testing.F) {
	icur := FecNodeConfigReconciler{
		Client:      nil,
		log:         &logrus.Logger{},
		nodeNameRef: types.NamespacedName{},
		drainerAndExecute: func(configurer func(ctx context.Context) bool, drain bool) error {
			return nil
		},
		sriovfecconfigurer: nil,
		restartDevicePlugin: func() error {
			return nil
		},
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		sfnc := sriovv2.SriovFecNodeConfig{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       sriovv2.SriovFecNodeConfigSpec{},
			Status:     sriovv2.SriovFecNodeConfigStatus{},
		}

		detectedInventory := sriovv2.NodeInventory{
			SriovAccelerators: []sriovv2.SriovAccelerator{},
		}

		fuzz.NewFromGoFuzz(data).Fuzz(&sfnc)
		fuzz.NewFromGoFuzz(data).Fuzz(&detectedInventory)

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Error: %v", icur)
			}
		}()
		_ = icur.isCardUpdateRequired(&sfnc, &detectedInventory)
	})
}

func FuzzVrbIsCardUpdateRequired(f *testing.F) {
	vicur := VrbNodeConfigReconciler{
		Client:      nil,
		log:         &logrus.Logger{},
		nodeNameRef: types.NamespacedName{},
		drainerAndExecute: func(configurer func(ctx context.Context) bool, drain bool) error {
			return nil
		},
		vrbconfigurer: nil,
		restartDevicePlugin: func() error {
			return nil
		},
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		svnc := vrbv1.SriovVrbNodeConfig{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       vrbv1.SriovVrbNodeConfigSpec{},
			Status:     vrbv1.SriovVrbNodeConfigStatus{},
		}

		detectedInventory := vrbv1.NodeInventory{
			SriovAccelerators: []vrbv1.SriovAccelerator{},
		}

		fuzz.NewFromGoFuzz(data).Fuzz(&svnc)
		fuzz.NewFromGoFuzz(data).Fuzz(&detectedInventory)

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Error: %v", vicur)
			}
		}()
		_ = vicur.isCardUpdateRequired(&svnc, &detectedInventory)
	})
}

func FuzzValidateNodeConfig(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		nc := sriovv2.SriovFecNodeConfigSpec{
			PhysicalFunctions: []sriovv2.PhysicalFunctionConfigExt{},
			DrainSkip:         false,
		}
		fuzz.NewFromGoFuzz(data).Fuzz(&nc)
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Error: %v", nc)
			}
		}()
		_ = validateNodeConfig(nc)
	})
}

func FuzzVrbValidateNodeConfig(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		nc := vrbv1.SriovVrbNodeConfigSpec{
			PhysicalFunctions: []vrbv1.PhysicalFunctionConfigExt{},
			DrainSkip:         false,
		}
		fuzz.NewFromGoFuzz(data).Fuzz(&nc)
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Error: %v", nc)
			}
		}()
		_ = validateVrbNodeConfig(nc)
	})
}
