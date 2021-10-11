// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package daemon

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/otcshare/openshift-operator/common/pkg/utils"
	sriovv2 "github.com/otcshare/openshift-operator/sriov-fec/api/v2"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("NodeConfigReconciler.Reconcile", func() {
	var scheme *runtime.Scheme

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(sriovv2.AddToScheme(scheme)).ToNot(HaveOccurred())
	})

	_ = Describe("", func() {
		var (
			fakeClient         client.Client
			nodeNameRef        types.NamespacedName
			reconciler         NodeConfigReconciler
			reconcileRequestes ctrl.Request
			nodeInventory      *sriovv2.NodeInventory
		)
		BeforeEach(func() {
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
				configureNodeFunction: func(nodeConfig *sriovv2.SriovFecNodeConfig) (isRebootRequested bool, err error) {
					for _, pf := range nodeConfig.Spec.PhysicalFunctions {
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
					return false, err
				},
			}

			getSriovInventory = func(log *logrus.Logger) (*sriovv2.NodeInventory, error) {
				return nodeInventory, nil
			}

			reconciler = NodeConfigReconciler{Client: fakeClient, log: utils.NewLogger(), nodeNameRef: nodeNameRef, configurer: configurer}
			reconcileRequestes = ctrl.Request{NamespacedName: nodeNameRef}
		})

		It("restores/recreates VFs removed after node reboot", func() {
			//sfnc does not exist yet
			sfnc := new(sriovv2.SriovFecNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, sfnc)).To(MatchError(ContainSubstring("not found")))

			//first reconcile creates missing sfnc
			//created sfnc exposes node inventory: status.NodeInventory
			_, err := reconciler.Reconcile(context.TODO(), reconcileRequestes)
			Expect(err).ToNot(HaveOccurred())
			sfnc = new(sriovv2.SriovFecNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, sfnc)).ToNot(HaveOccurred())
			Expect(sfnc.Status.Inventory).To(Equal(*nodeInventory))

			//define new spec
			//fake client doesn't handle generation field so take care about incrementing it
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

			//second reconcile should configure inventory to be aligned with requested spec
			_, err = reconciler.Reconcile(context.TODO(), reconcileRequestes)
			Expect(err).ToNot(HaveOccurred())
			sfnc = new(sriovv2.SriovFecNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, sfnc)).ToNot(HaveOccurred())
			Expect(sfnc.Status.Inventory).ToNot(Equal(nodeInventory))

			//simulate node reboot - remove all existing VFs
			for accidx := range nodeInventory.SriovAccelerators {
				nodeInventory.SriovAccelerators[accidx].VFs = []sriovv2.VF{}
			}

			//third reconcile should reconfigure VFs
			_, err = reconciler.Reconcile(context.TODO(), reconcileRequestes)
			Expect(err).ToNot(HaveOccurred())
			sfnc = new(sriovv2.SriovFecNodeConfig)
			Expect(fakeClient.Get(context.TODO(), nodeNameRef, sfnc)).ToNot(HaveOccurred())
			Expect(sfnc.Status.Inventory).ToNot(Equal(nodeInventory))
		})
	})
})

type testConfigurerProto struct {
	configureNodeFunction func(nodeConfig *sriovv2.SriovFecNodeConfig) (isRebootRequested bool, err error)
}

func (t testConfigurerProto) configureNode(nodeConfig *sriovv2.SriovFecNodeConfig) (isRebootRequested bool, err error) {
	return t.configureNodeFunction(nodeConfig)
}
