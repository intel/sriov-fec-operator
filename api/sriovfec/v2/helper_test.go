// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2024 Intel Corporation

package v2

import (
	"sort"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("helperFunctionsTest", func() {
	var _ = Describe("Sorting Cluster Configs by Priority", func() {
		It("should sort the cluster configs by priority in descending order and then by name in ascending order", func() {
			clusterConfig := []SriovFecClusterConfig{
				{Spec: SriovFecClusterConfigSpec{Priority: 3}},
				{Spec: SriovFecClusterConfigSpec{Priority: 1}},
				{Spec: SriovFecClusterConfigSpec{Priority: 3}},
				{Spec: SriovFecClusterConfigSpec{Priority: 2}},
			}

			// Assuming GetName() method exists and returns the Name field
			sort.Sort(ByPriority(clusterConfig))

			// Assert that the elements are sorted by Priority in descending order
			Expect(clusterConfig[0].Spec.Priority).To(Equal(3))
			Expect(clusterConfig[1].Spec.Priority).To(Equal(3))
			Expect(clusterConfig[2].Spec.Priority).To(Equal(2))
			Expect(clusterConfig[3].Spec.Priority).To(Equal(1))
		})
	})

	var _ = Describe("AcceleratorSelector", func() {
		Describe("Matches function", func() {
			It("should match an accelerator when all criteria are met", func() {
				selector := AcceleratorSelector{
					VendorID:   "8086",
					PCIAddress: "0000:00:02.0",
					PFDriver:   "i40e",
					MaxVFs:     32,
					DeviceID:   "154c",
				}

				accelerator := SriovAccelerator{
					VendorID:   "8086",
					PCIAddress: "0000:00:02.0",
					PFDriver:   "i40e",
					MaxVFs:     32,
					DeviceID:   "154c",
				}

				Expect(selector.Matches(accelerator)).To(BeTrue())
			})

			It("should not match an accelerator when one criterion does not match", func() {
				selector := AcceleratorSelector{
					VendorID:   "8086",
					PCIAddress: "0000:00:02.0",
					PFDriver:   "i40e",
					MaxVFs:     32,
					DeviceID:   "154c",
				}

				accelerator := SriovAccelerator{
					VendorID:   "8086",
					PCIAddress: "0000:00:02.0",
					PFDriver:   "ixgbe",
					MaxVFs:     32,
					DeviceID:   "154c",
				}

				Expect(selector.Matches(accelerator)).To(BeFalse())
			})

			Context("when optional fields are empty", func() {
				It("should match an accelerator if only mandatory criteria are met", func() {
					selector := AcceleratorSelector{
						VendorID: "8086",
					}

					accelerator := SriovAccelerator{
						VendorID:   "8086",
						PCIAddress: "0000:00:02.0",
						PFDriver:   "i40e",
						MaxVFs:     32,
						DeviceID:   "154c",
					}

					Expect(selector.Matches(accelerator)).To(BeTrue())
				})
			})
		})
	})

	var _ = Describe("SriovFecNodeConfig", func() {
		Describe("FindCondition function", func() {
			var nodeConfig *SriovFecNodeConfig

			BeforeEach(func() {
				nodeConfig = &SriovFecNodeConfig{
					Status: SriovFecNodeConfigStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Ready",
								Status: metav1.ConditionTrue,
								Reason: "NodeIsReady",
							},
							{
								Type:   "Degraded",
								Status: metav1.ConditionFalse,
								Reason: "NodeIsHealthy",
							},
						},
					},
				}
			})

			It("should find the condition by type", func() {
				condition := nodeConfig.FindCondition("Ready")
				Expect(condition).NotTo(BeNil())
				Expect(condition.Type).To(Equal("Ready"))
				Expect(condition.Status).To(Equal(metav1.ConditionTrue))
				Expect(condition.Reason).To(Equal("NodeIsReady"))
			})

			It("should return nil if the condition is not found", func() {
				condition := nodeConfig.FindCondition("NonExistent")
				Expect(condition).To(BeNil())
			})
		})
	})
})
