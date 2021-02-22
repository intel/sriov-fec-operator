// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	sriov "github.com/otcshare/openshift-operator/sriov-fec/api/v1"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	pciAddress = "0000:14:00.1"
)

var _ = Describe("SriovDaemonTest", func() {
	data := new(TestData)
	reconciler := new(NodeConfigReconciler)

	var _ = BeforeEach(func() {
		//configure kernel controller
		osReleaseFilepath = "testdata/rhcos_os_release"
		procCmdlineFilePath = "testdata/cmdline_test"

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
		getSriovInventory = func(_ logr.Logger) (*sriov.NodeInventory, error) {
			return &data.NodeInventory, nil
		}
	})

	var _ = Context("Reconciler", func() {
		BeforeEach(func() {
			data = new(TestData)
			Expect(readAndUnmarshall("testdata/node_config.json", data)).To(Succeed())
		})

		AfterEach(func() {
			nn := data.GetNamespacedName()
			if err := k8sClient.Get(context.TODO(), nn, &data.SriovFecNodeConfig); err == nil {
				data.SriovFecNodeConfig.Spec = sriov.SriovFecNodeConfigSpec{
					PhysicalFunctions: []sriov.PhysicalFunctionConfig{},
				}
				Expect(k8sClient.Update(context.TODO(), &data.SriovFecNodeConfig)).NotTo(HaveOccurred())
				Expect(returnLastArg(reconciler.Reconcile(context.TODO(), ctrl.Request{NamespacedName: nn}))).ToNot(HaveOccurred())
				Expect(k8sClient.Delete(context.TODO(), &data.SriovFecNodeConfig)).ToNot(HaveOccurred())
			} else if errors.IsNotFound(err) {
				log.Info("Requested NodeConfig does not exists", "NodeConfig", &data.SriovFecNodeConfig)
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(k8sClient.Delete(context.TODO(), &data.Node)).To(Succeed())
		})

		var _ = It("will create cr without node config", func() {
			Expect(k8sClient.Create(context.TODO(), &data.Node)).To(Succeed())

			Expect(initReconciler(reconciler, data.NodeConfigName(), data.NodeConfigNS())).To(Succeed())

			Expect(
				returnLastArg(
					reconciler.Reconcile(context.TODO(), ctrl.Request{NamespacedName: data.GetNamespacedName()}),
				),
			).To(Succeed())

			//Check if node config was created out of cluster config
			nodeConfigs := &sriov.SriovFecNodeConfigList{}
			Expect(k8sClient.List(context.TODO(), nodeConfigs)).To(Succeed())
			Expect(nodeConfigs.Items).To(HaveLen(1))
		})

		var _ = It("will ignore cr with wrong node name", func() {
			Expect(k8sClient.Create(context.TODO(), &data.Node)).To(Succeed())

			Expect(initReconciler(reconciler, "wrongName", data.NodeConfigNS())).To(Succeed())

			Expect(
				returnLastArg(
					reconciler.Reconcile(context.TODO(), ctrl.Request{NamespacedName: data.GetNamespacedName()}),
				),
			).To(Succeed())

			//Check if node config was created out of cluster config
			nodeConfigs := &sriov.SriovFecNodeConfigList{}
			Expect(k8sClient.List(context.TODO(), nodeConfigs)).To(Succeed())
			Expect(nodeConfigs.Items).To(BeEmpty())
		})

		var _ = It("will ignore cr with wrong namespace", func() {
			Expect(
				k8sClient.Create(context.TODO(), &data.Node),
			).To(Succeed())

			Expect(initReconciler(reconciler, data.NodeConfigName(), "wrongNamespace")).To(Succeed())

			Expect(
				returnLastArg(
					reconciler.Reconcile(context.TODO(), ctrl.Request{NamespacedName: data.GetNamespacedName()}),
				),
			).To(Succeed())

			//Check if node config was created out of cluster config
			nodeConfigs := &sriov.SriovFecNodeConfigList{}
			Expect(k8sClient.List(context.TODO(), nodeConfigs)).To(Succeed())
			Expect(nodeConfigs.Items).To(BeEmpty())
		})

		var _ = It("will fail when namespace will be not handle", func() {
			Expect(k8sClient.Create(context.TODO(), &data.Node)).To(Succeed())
			Expect(initReconciler(reconciler, data.NodeConfigName(), "wrongNamespace")).To(Succeed())
			Expect(
				returnLastArg(
					reconciler.Reconcile(context.TODO(),
						ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "wrongNamespace", Name: data.NodeConfigName()}},
					),
				),
			).To(HaveOccurred())

			//Check if node config was created out of cluster config
			nodeConfigs := &sriov.SriovFecNodeConfigList{}
			Expect(k8sClient.List(context.TODO(), nodeConfigs)).To(Succeed())
			Expect(nodeConfigs.Items).To(BeEmpty())
		})

		var _ = It("will create cr with node config and failed reboot", func() {
			execCmdMock := new(runExecCmdMock)
			execCmdMock.
				onCall([]string{"chroot", "/host/", "rpm-ostree", "kargs"}).Return("", nil).
				onCall([]string{"chroot", "/host/", "rpm-ostree", "kargs", "--append", "intel_iommu=on"}).Return("", nil).
				onCall([]string{"chroot", "/host/", "rpm-ostree", "kargs", "--append", "iommu=pt"}).Return("", nil).
				onCall([]string{
					"chroot", "/host", "systemd-run", "--unit", "sriov-fec-daemon-reboot", "--description", "sriov-fec-daemon reboot",
					"/bin/sh", "-c", "systemctl stop kubelet.service; reboot",
				}).
				Return("", fmt.Errorf("error"))

			initNodeConfiguratorRunExecCmd(execCmdMock.execute)
			procCmdlineFilePath = "testdata/cmdline_test_missing_param"

			Expect(
				k8sClient.Create(context.TODO(), &data.Node),
			).To(Succeed())

			Expect(
				k8sClient.Create(context.TODO(), &data.SriovFecNodeConfig),
			).To(Succeed())

			Expect(initReconciler(reconciler, data.NodeConfigName(), data.NodeConfigNS())).To(Succeed())
			Expect(
				returnLastArg(reconciler.Reconcile(context.TODO(), ctrl.Request{NamespacedName: data.GetNamespacedName()})),
			).To(Succeed())

			//Check if node config was created out of cluster config
			nodeConfigs := &sriov.SriovFecNodeConfigList{}
			Expect(k8sClient.List(context.TODO(), nodeConfigs)).To(Succeed())
			Expect(nodeConfigs.Items).To(HaveLen(1))
			Expect(execCmdMock.verify()).To(Succeed())
		})

		var _ = It("will create cr with node config", func() {
			osExecMock := new(runExecCmdMock)
			osExecMock.
				onCall([]string{"chroot", "/host/", "modprobe", "PFdriver"}).
				Return("", nil).
				onCall([]string{"chroot", "/host/", "modprobe", "v"}).
				Return("", nil).
				onCall([]string{"/sriov_workdir/pf_bb_config", "FPGA_5GNR", "-c", fmt.Sprintf("%s.ini", filepath.Join(workdir, pciAddress)), "-p", pciAddress}).
				Return("", nil)

			initNodeConfiguratorRunExecCmd(osExecMock.execute)

			Expect(k8sClient.Create(context.TODO(), &data.Node)).To(Succeed())
			Expect(k8sClient.Create(context.TODO(), &data.SriovFecNodeConfig)).To(Succeed())

			Expect(initReconciler(reconciler, data.NodeConfigName(), data.NodeConfigNS())).To(Succeed())
			Expect(
				returnLastArg(
					reconciler.Reconcile(context.TODO(), ctrl.Request{NamespacedName: data.GetNamespacedName()}),
				),
			).To(Succeed())

			//Check if node config was created out of cluster config
			nodeConfigs := &sriov.SriovFecNodeConfigList{}
			Expect(k8sClient.List(context.TODO(), nodeConfigs)).To(Succeed())
			Expect(nodeConfigs.Items).To(HaveLen(1))
			Expect(osExecMock.verify()).To(Succeed())
		})

		var _ = It("will create cr with node config and failed unbind device", func() {
			osExecMock := new(runExecCmdMock)
			osExecMock.
				onCall([]string{"chroot", "/host/", "modprobe", "PFdriver"}).
				Return("", nil).
				onCall([]string{"chroot", "/host/", "modprobe", "v"}).
				Return("", nil)

			initNodeConfiguratorRunExecCmd(osExecMock.execute)

			Expect(createFiles(filepath.Join(sysBusPciDevices, pciAddress), "driver")).To(Succeed())
			defer os.Remove(filepath.Join(sysBusPciDevices, pciAddress, "driver"))

			Expect(k8sClient.Create(context.TODO(), &data.Node)).To(Succeed())
			Expect(k8sClient.Create(context.TODO(), &data.SriovFecNodeConfig)).To(Succeed())

			Expect(initReconciler(reconciler, data.NodeConfigName(), data.NodeConfigNS())).To(Succeed())
			Expect(returnLastArg(reconciler.Reconcile(context.TODO(), ctrl.Request{NamespacedName: data.GetNamespacedName()}))).To(Succeed())

			//Check if node config was created out of cluster config
			nodeConfigs := &sriov.SriovFecNodeConfigList{}
			Expect(k8sClient.List(context.TODO(), nodeConfigs)).To(Succeed())
			Expect(nodeConfigs.Items).To(HaveLen(1))
			Expect(osExecMock.verify()).To(Succeed())
		})

		var _ = It("will create cr with node config and enable master bus", func() {
			driver := "pci-pf-stub"
			expectedSetpciCommandOutput := fmt.Sprintf("%s = 1", pciAddress)

			data.SriovFecNodeConfig.Spec.PhysicalFunctions[0].PFDriver = driver

			osExecMock := new(runExecCmdMock).
				onCall([]string{"chroot", "/host/", "modprobe", driver}).Return("", nil).
				onCall([]string{"chroot", "/host/", "modprobe", "v"}).Return("", nil).
				onCall(
					[]string{
						"/sriov_workdir/pf_bb_config", "FPGA_5GNR", "-c", fmt.Sprintf("%s.ini", filepath.Join(workdir, pciAddress)), "-p", pciAddress,
					},
				).Return("", nil).
				onCall([]string{"chroot", "/host/", "setpci", "-v", "-s", pciAddress, "COMMAND"}).Return(expectedSetpciCommandOutput, nil).
				onCall([]string{"chroot", "/host/", "setpci", "-v", "-s", pciAddress, "COMMAND=05"}).Return(expectedSetpciCommandOutput, nil).
				build()

			initNodeConfiguratorRunExecCmd(osExecMock.execute)

			Expect(k8sClient.Create(context.TODO(), &data.Node)).To(Succeed())
			Expect(k8sClient.Create(context.TODO(), &data.SriovFecNodeConfig)).To(Succeed())

			Expect(initReconciler(reconciler, data.NodeConfigName(), data.NodeConfigNS())).To(Succeed())

			Expect(
				returnLastArg(
					reconciler.Reconcile(context.TODO(), ctrl.Request{NamespacedName: data.GetNamespacedName()}),
				),
			).To(Succeed())

			//Check if node config was created out of cluster config
			nodeConfigs := &sriov.SriovFecNodeConfigList{}
			Expect(k8sClient.List(context.TODO(), nodeConfigs)).To(Succeed())
			Expect(nodeConfigs.Items).To(HaveLen(1))
			Expect(osExecMock.verify()).To(Succeed())
		})
	})

	var _ = Describe("Reconciler manager", func() {
		var _ = It("setup with invalid manager", func() {
			var m ctrl.Manager
			Expect(new(NodeConfigReconciler).SetupWithManager(m)).To(HaveOccurred())
		})
	})
})

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
	SriovFecNodeConfig sriov.SriovFecNodeConfig `json:"sriov_fec_node_config"`
	NodeInventory      sriov.NodeInventory      `json:"node_inventory"`
	Node               core.Node                `json:"node"`
}

func (d *TestData) SetPcieAddress(addr string) {
	for _, f := range d.SriovFecNodeConfig.Spec.PhysicalFunctions {
		f.PCIAddress = addr
	}

	for _, a := range d.NodeInventory.SriovAccelerators {
		a.PCIAddress = addr
		for _, vf := range a.VFs {
			vf.PCIAddress = addr
		}
	}
}

func (d *TestData) PcieAddress() string {
	for _, a := range d.NodeInventory.SriovAccelerators {
		return a.PCIAddress
	}
	panic("PcieAddress is not defined")
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

func initReconciler(toBeInitialized *NodeConfigReconciler, nodeName, namespace string) error {
	cset, err := clientset.NewForConfig(config)
	if err != nil {
		return err
	}

	r, err := NewNodeConfigReconciler(k8sClient, cset, log, nodeName, namespace)
	if err != nil {
		return err
	}

	*toBeInitialized = *r
	return nil
}

func initNodeConfiguratorRunExecCmd(f func([]string, logr.Logger) (string, error)) {
	runExecCmd = f
}

func returnLastArg(args ...interface{}) interface{} {
	return args[len(args)-1]
}
