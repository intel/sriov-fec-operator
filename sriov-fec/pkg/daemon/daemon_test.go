// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	sriovv2 "github.com/smart-edge-open/openshift-operator/sriov-fec/api/v2"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

var (
	pciAddress = "0000:14:00.1"
)

var _ = Describe("NodeConfigReconciler", func() {
	const (
		_THIS_NODE_NAME      = "worker"
		_SUPPORTED_NAMESPACE = "default"
	)

	BeforeEach(func() {
		Expect(sriovv2.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	})

	Describe("NodeConfigReconciler.Reconcile(...)", func() {
		var (
			data    *TestData
			testEnv *envtest.Environment
		)

		BeforeEach(func() {
			//configure kernel controller
			osReleaseFilepath = "testdata/rhcos_os_release"
			procCmdlineFilePath = "testdata/cmdline_test"
			configPath = "testdata/accelerators.json"

			//configure node configurator
			workdir = testTmpFolder
			sysBusPciDevices = testTmpFolder
			sysBusPciDrivers = testTmpFolder
			Expect(createFiles(filepath.Join(sysBusPciDevices, pciAddress), "driver_override", vfNumFile)).To(Succeed())
			Expect(createFiles(filepath.Join(sysBusPciDrivers, "PFdriver"), "bind")).To(Succeed())
			Expect(createFiles(filepath.Join(sysBusPciDrivers, "pci-pf-stub"), "bind")).To(Succeed())

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

			var reconciler *NodeConfigReconciler

			BeforeEach(func() {
				By("bootstrapping test environment")

				onGetErrorReturningClient := testClient{
					Client: fake.NewClientBuilder().Build(),
					get: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						return errors.NewNotFound(sriovv2.GroupVersion.WithResource("SriovFecNodeConfig").GroupResource(), "cannot get")
					},
					create: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
						return fmt.Errorf("cannot create")
					},
				}

				nodeNameRef := types.NamespacedName{Namespace: _SUPPORTED_NAMESPACE, Name: _THIS_NODE_NAME}
				configurer, _ := NewNodeConfigurer(func(operation func(ctx context.Context) bool, drain bool) error { return nil }, &onGetErrorReturningClient, nodeNameRef)
				var err error
				reconciler, err = NewNodeConfigReconciler(&onGetErrorReturningClient, configurer, nodeNameRef)
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

					fakeDrainer := func(configure func(ctx context.Context) bool, drain bool) error {
						configure(context.TODO())
						return nil
					}

					nodeNameRef := types.NamespacedName{Namespace: _SUPPORTED_NAMESPACE, Name: _THIS_NODE_NAME}
					configurer, _ := NewNodeConfigurer(fakeDrainer, k8sClient, nodeNameRef)

					reconciler, err := NewNodeConfigReconciler(k8sClient, configurer, nodeNameRef)
					Expect(err).ToNot(HaveOccurred())

					k8sManager, err := CreateManager(config, _SUPPORTED_NAMESPACE, scheme.Scheme)
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

				AfterEach(func() {
					By("tearing down the test environment")
					gexec.KillAndWait("5s")
					Expect(testEnv.Stop()).ToNot(HaveOccurred())
				})

				Context("Requested spec/config is correct and refers to existing accelerators", func() {
					It("spec/config should be applied", func() {
						osExecMock := new(runExecCmdMock).
							onCall([]string{"chroot", "/host/", "modprobe", "PFdriver"}).
							Return("", nil).
							onCall([]string{"chroot", "/host/", "modprobe", "v"}).
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

					When("Kernel has missing parameters", func() {
						Specify("kernel has to be reconfigured and spec/config should be applied", func() {

							expectedCallsRelatedWithReboot := new(runExecCmdMock).
								//mocking calls for nodeConfigurator.rebootNode(...)
								onCall([]string{"chroot", "--userspec", "0", "/host/", "rpm-ostree", "kargs"}).Return("", nil).
								onCall([]string{"chroot", "--userspec", "0", "/host/", "rpm-ostree", "kargs", "--append", "intel_iommu=on"}).Return("", nil).
								onCall([]string{"chroot", "--userspec", "0", "/host/", "rpm-ostree", "kargs", "--append", "iommu=pt"}).Return("", nil).
								onCall([]string{
									"chroot", "--userspec", "0", "/host", "systemd-run", "--unit", "sriov-fec-daemon-reboot", "--description", "sriov-fec-daemon reboot",
									"/bin/sh", "-c", "systemctl stop kubelet.service; reboot",
								}).Return("", nil)

							//init daemon.runExecCmd
							runExecCmd = expectedCallsRelatedWithReboot.execute
							//manifest missing kernel params
							procCmdlineFilePath = "testdata/cmdline_test_missing_param"

							//Reconcile(...) function will be executed automatically after node reboot
							//I have no idea how to simulate reboot/restart controller
							//I am making resyncPeriod tiny to force immediate requeue of reconciliation process
							resyncPeriod = time.Second

							//apply nodeconfig
							Expect(k8sClient.Patch(context.TODO(), &data.SriovFecNodeConfig, client.Merge, client.FieldOwner("test"))).To(Succeed())

							//node config processing should be in progress
							Eventually(func() (nc sriovv2.SriovFecNodeConfig, err error) {
								return nc, k8sClient.Get(context.TODO(), data.GetNamespacedName(), &nc)
							}, "10s").
								Should(
									WithTransform(
										func(nc sriovv2.SriovFecNodeConfig) *metav1.Condition {
											return nc.FindCondition(ConditionConfigured)
										}, SatisfyAll(
											Not(BeNil()),
											WithTransform(func(c *metav1.Condition) string { return c.Reason }, Equal(string(ConfigurationInProgress))),
										),
									),
								)

							Eventually(func() error {
								return expectedCallsRelatedWithReboot.verify()
							}, "10s").Should(Succeed())

							//manifest expected kernel configuration
							procCmdlineFilePath = "testdata/cmdline_test"

							expectedApplyConfigRelatedCalls := new(runExecCmdMock).
								onCall([]string{"chroot", "/host/", "modprobe", "PFdriver"}).Return("", nil).
								onCall([]string{"chroot", "/host/", "modprobe", "v"}).Return("", nil).
								onCall([]string{"/sriov_workdir/pf_bb_config", "FPGA_5GNR", "-c", fmt.Sprintf("%s.ini", filepath.Join(workdir, pciAddress)), "-p", pciAddress}).Return("", nil)

							//init daemon.runExecCmd
							runExecCmd = expectedApplyConfigRelatedCalls.execute

							Eventually(func() error {
								return expectedApplyConfigRelatedCalls.verify()
							}, "10s").Should(Succeed())

							resyncPeriod = time.Minute

							Eventually(func() (nc sriovv2.SriovFecNodeConfig, err error) {
								return nc, k8sClient.Get(context.TODO(), data.GetNamespacedName(), &nc)
							}, "10s", "0.5s").
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
						})
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
					reconciler := NodeConfigReconciler{Client: fakeClient, log: log}

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

			Context("Modification of SriovFecNodeConfig not related with this NodeConfigReconciler instance", func() {
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
					configurer, _ := NewNodeConfigurer(drainer, k8sClient, nodeNameRef)

					nodeReconciler, err := NewNodeConfigReconciler(k8sClient, configurer, nodeNameRef)
					Expect(err).ToNot(HaveOccurred())

					reconciler := nodeRecocnilerWrapper{
						NodeConfigReconciler: nodeReconciler,
						reconcilingFunc: func(context.Context, reconcile.Request) (reconcile.Result, error) {
							Fail("reconcile function should not be called")
							return reconcile.Result{}, nil
						},
					}

					k8sManager, err := CreateManager(config, _SUPPORTED_NAMESPACE, scheme.Scheme)
					Expect(err).ToNot(HaveOccurred())

					Expect(reconciler.SetupWithManager(k8sManager)).ToNot(HaveOccurred())

					go func() {
						Expect(k8sManager.Start(context.TODO())).ToNot(HaveOccurred())
					}()
				})

				AfterEach(func() {
					By("tearing down the test environment")
					gexec.KillAndWait("5s")
					Expect(testEnv.Stop()).ToNot(HaveOccurred())
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

			When("NodeConfigReconciler cannot read inventory", func() {
				It("SriovFecNodeConfig.Status.Condition should expose appropriate info", func() {
					fakeClient := fake.NewClientBuilder().WithObjects(&data.SriovFecNodeConfig).Build()
					reconciler := NodeConfigReconciler{Client: fakeClient, log: log}
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
		reconciler := NodeConfigReconciler{Client: fakeClient, log: log}

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
	*NodeConfigReconciler
	reconcilingFunc func(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
}

func (n *nodeRecocnilerWrapper) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return n.reconcilingFunc(ctx, req)
}

type testClient struct {
	client.Client
	get    func(ctx context.Context, key client.ObjectKey, obj client.Object) error
	create func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
}

func (e *testClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
	bytes, err := ioutil.ReadFile(filepath)
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
