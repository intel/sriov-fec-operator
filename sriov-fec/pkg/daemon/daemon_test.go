// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"context"
	"fmt"
	"os"
	"sort"

	"path/filepath"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	sriovv1 "github.com/otcshare/openshift-operator/sriov-fec/api/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	fakeGetKargsErrReturn       error = nil
	fakeAppendKargsErrReturn    error = nil
	fakeModprobeErrReturn       error = nil
	fakeSystemdErrReturn        error = nil
	fakeSriovInventoryErrReturn error = nil
	fakePFConfigErrReturn       error = nil
	fakeGetVFListReturn         error = nil
	fakeGetKargsOutput          string
	fakeSriovInventoryOutput    *sriovv1.NodeInventory
	fakeGetVFconfiguredOutput   int
	fakeGetVFListOutput         []string
	lastRunExec                 string
)

func clean() {
	fakeGetKargsErrReturn = nil
	fakeAppendKargsErrReturn = nil
	fakeModprobeErrReturn = nil
	fakeSystemdErrReturn = nil
	fakeSriovInventoryErrReturn = nil
	fakeGetVFListReturn = nil
	fakeGetKargsOutput = ""
	fakeGetVFconfiguredOutput = 0
	fakeGetVFListOutput = nil
	lastRunExec = ""
}

func createFileInFolder(folderPath, fileName string) error {
	_, err := os.Stat(folderPath)
	if os.IsNotExist(err) {
		errDir := os.MkdirAll(folderPath, 0777)
		if errDir != nil {
			return err
		}
	}
	filePath := filepath.Join(folderPath, fileName)
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	f.Close()
	return nil
}

func fakeGetVFconfigured(pf string) int {
	return fakeGetVFconfiguredOutput
}

func fakeGetVFList(pf string) ([]string, error) {
	return fakeGetVFListOutput, fakeGetVFListReturn
}

func fakeRunExecCmd(args []string, log logr.Logger) (string, error) {
	sort.Strings(args)
	if i := sort.SearchStrings(args, "systemd-run"); i < len(args) {
		lastRunExec = "systemd-run"
		return "", fakeSystemdErrReturn
	}
	if i := sort.SearchStrings(args, "rpm-ostree"); i < len(args) {
		if i := sort.SearchStrings(args, "--append"); i < len(args) {
			lastRunExec = "rpm-ostree --append"
			return "", fakeAppendKargsErrReturn
		}
		lastRunExec = "rpm-ostree get"
		return fakeGetKargsOutput, fakeGetKargsErrReturn
	}
	if i := sort.SearchStrings(args, "modprobe"); i < len(args) {
		lastRunExec = "modprobe"
		return "", fakeModprobeErrReturn
	}
	if i := sort.SearchStrings(args, pfConfigAppFilepath); i < len(args) {
		lastRunExec = "pfConfigAppFile"
		return "", fakePFConfigErrReturn
	}
	lastRunExec = "Unsupported"
	return "", fmt.Errorf("Unsupported command")
}

func fakeGetSriovInventory(log logr.Logger) (*sriovv1.NodeInventory, error) {
	return fakeSriovInventoryOutput, fakeSriovInventoryErrReturn
}

var (
	node                        *corev1.Node
	nodeConfig                  *sriovv1.SriovFecNodeConfig
	request                     ctrl.Request
	reconciler                  *NodeConfigReconciler
	log                         = ctrl.Log.WithName("SriovDaemon-test")
	doDeconf                    = true
	nodeName                    = "config"
	DEFAULT_CLUSTER_CONFIG_NAME = "config"
	NAMESPACE                   = "default"
	PCIAddress                  = "0000:14:00.1"
	namespacedName              = types.NamespacedName{
		Name:      DEFAULT_CLUSTER_CONFIG_NAME,
		Namespace: NAMESPACE,
	}
)

func createCRWithNodeConfig(addNodeConfig bool) error {
	getSriovInventory = fakeGetSriovInventory
	getVFconfigured = fakeGetVFconfigured
	getVFList = fakeGetVFList

	err := k8sClient.Create(context.TODO(), node)
	Expect(err).ToNot(HaveOccurred())

	if addNodeConfig {
		// simulate creation of cluster config by the user
		err = k8sClient.Create(context.TODO(), nodeConfig)
		Expect(err).ToNot(HaveOccurred())
	}

	Expect(config).ToNot(BeNil())
	cset, err := clientset.NewForConfig(config)
	Expect(err).ToNot(HaveOccurred())

	reconciler = NewNodeConfigReconciler(k8sClient, cset,
		log, nodeName, NAMESPACE)

	request = ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: NAMESPACE,
			Name:      DEFAULT_CLUSTER_CONFIG_NAME,
		},
	}

	_, err = reconciler.Reconcile(request)
	return err
}

var _ = Describe("SriovDaemonTest", func() {
	var _ = Context("Reconciler", func() {

		BeforeEach(func() {
			workdir = testTmpFolder
			redhatReleaseFilepath = "testdata/redhat-release_test"
			procCmdlineFilePath = "testdata/cmdline_test"
			sysBusPciDevices = testTmpFolder
			sysBusPciDrivers = testTmpFolder
			clean()
			runExecCmd = fakeRunExecCmd
			doDeconf = true

			err := createFileInFolder(filepath.Join(sysBusPciDevices, PCIAddress), "driver_override")
			Expect(err).ToNot(HaveOccurred())
			err = createFileInFolder(filepath.Join(sysBusPciDevices, PCIAddress), vfNumFile)
			Expect(err).ToNot(HaveOccurred())
			err = createFileInFolder(filepath.Join(sysBusPciDrivers, "PFdriver"), "bind")
			Expect(err).ToNot(HaveOccurred())

			fakeSriovInventoryOutput = &sriovv1.NodeInventory{
				SriovAccelerators: []sriovv1.SriovAccelerator{
					{
						VendorID:   "1",
						DeviceID:   "0d8f",
						Driver:     "D3",
						MaxVFs:     1,
						PCIAddress: PCIAddress,
						VFs: []sriovv1.VF{
							{
								PCIAddress: PCIAddress,
								Driver:     "D3",
								DeviceID:   "0d8f",
							},
						},
					},
				},
			}

			node = &corev1.Node{
				ObjectMeta: v1.ObjectMeta{
					Name: nodeName,
					Labels: map[string]string{
						"fpga.intel.com/intel-accelerator-present": "",
					},
				},
			}
			nodeConfig = &sriovv1.SriovFecNodeConfig{
				ObjectMeta: v1.ObjectMeta{
					Name:      DEFAULT_CLUSTER_CONFIG_NAME,
					Namespace: NAMESPACE,
				},
				Spec: sriovv1.SriovFecNodeConfigSpec{
					PhysicalFunctions: []sriovv1.PhysicalFunctionConfig{
						{
							PCIAddress: PCIAddress,
							PFDriver:   "PFdriver",
							VFDriver:   "v",
							VFAmount:   5,
							BBDevConfig: sriovv1.BBDevConfig{
								N3000: &sriovv1.N3000BBDevConfig{
									NetworkType: "FPGA_LTE",
									PFMode:      false,
									FLRTimeOut:  10,
									Downlink: sriovv1.UplinkDownlink{
										Bandwidth:   3,
										LoadBalance: 3,
										Queues: sriovv1.UplinkDownlinkQueues{
											VF0: 0,
											VF1: 1,
											VF2: 2,
											VF3: 3,
											VF4: 4,
											VF5: 5,
											VF6: 6,
											VF7: 7,
										},
									},
									Uplink: sriovv1.UplinkDownlink{
										Bandwidth:   2,
										LoadBalance: 2,
										Queues: sriovv1.UplinkDownlinkQueues{
											VF0: 0,
											VF1: 1,
											VF2: 2,
											VF3: 3,
											VF4: 4,
											VF5: 5,
											VF6: 6,
											VF7: 7,
										},
									},
								},
							},
						},
					},
				},
			}
		})
		AfterEach(func() {
			var err error
			if doDeconf {
				err = k8sClient.Get(context.TODO(), namespacedName, nodeConfig)
				Expect(err).NotTo(HaveOccurred())
				nodeConfig.Spec = sriovv1.SriovFecNodeConfigSpec{
					PhysicalFunctions: []sriovv1.PhysicalFunctionConfig{},
				}
				err = k8sClient.Update(context.TODO(), nodeConfig)
				Expect(err).NotTo(HaveOccurred())

				_, err = reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
			}
			err = k8sClient.Delete(context.TODO(), nodeConfig)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("will create cr without node config", func() {
			err := createCRWithNodeConfig(false)
			Expect(err).ToNot(HaveOccurred())

			//Check if node config was created out of cluster config
			nodeConfigs := &sriovv1.SriovFecNodeConfigList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(lastRunExec).To(Equal(""))
		})
		var _ = It("will create cr with node config and failed reboot", func() {
			fakeSystemdErrReturn = fmt.Errorf("error")
			procCmdlineFilePath = "testdata/cmdline_test_missing_param"
			err := createCRWithNodeConfig(true)
			Expect(err).ToNot(HaveOccurred())

			//Check if node config was created out of cluster config
			nodeConfigs := &sriovv1.SriovFecNodeConfigList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(lastRunExec).To(Equal("systemd-run"))
		})
		var _ = It("will create cr with node config", func() {
			err := createCRWithNodeConfig(true)
			Expect(err).ToNot(HaveOccurred())

			//Check if node config was created out of cluster config
			nodeConfigs := &sriovv1.SriovFecNodeConfigList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(lastRunExec).To(Equal("pfConfigAppFile"))
		})
		var _ = It("will create cr with node config and failed unbind device", func() {
			err := createFileInFolder(filepath.Join(sysBusPciDevices, PCIAddress), "driver")
			Expect(err).ToNot(HaveOccurred())

			err = createCRWithNodeConfig(true)
			Expect(err).ToNot(HaveOccurred())

			err = os.Remove(filepath.Join(sysBusPciDevices, PCIAddress, "driver"))
			Expect(err).ToNot(HaveOccurred())

			//Check if node config was created out of cluster config
			nodeConfigs := &sriovv1.SriovFecNodeConfigList{}
			err = k8sClient.List(context.TODO(), nodeConfigs)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodeConfigs.Items)).To(Equal(1))
			Expect(lastRunExec).To(Equal("systemd-run"))
		})
	})
	var _ = Describe("Reconciler manager", func() {
		var _ = It("setup with invalid manager", func() {
			var m ctrl.Manager
			var reconciler NodeConfigReconciler
			err := reconciler.SetupWithManager(m)
			Expect(err).To(HaveOccurred())
		})
	})
})
