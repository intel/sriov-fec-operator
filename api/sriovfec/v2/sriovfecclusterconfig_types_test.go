// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2024 Intel Corporation

package v2

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ = Describe("UplinkDownlinkQueues", func() {
	Describe("String method", func() {
		It("should correctly format the struct's fields into a string", func() {
			udq := UplinkDownlinkQueues{
				VF0: 1, VF1: 2, VF2: 3, VF3: 4,
				VF4: 5, VF5: 6, VF6: 7, VF7: 8,
			}

			expectedString := "1,2,3,4,5,6,7,8"
			Expect(udq.String()).To(Equal(expectedString))
		})

		It("should correctly format the struct's fields into a string with zeros", func() {
			udq := UplinkDownlinkQueues{
				VF0: 0, VF1: 0, VF2: 0, VF3: 0,
				VF4: 0, VF5: 0, VF6: 0, VF7: 0,
			}

			expectedString := "0,0,0,0,0,0,0,0"
			Expect(udq.String()).To(Equal(expectedString))
		})

		It("should correctly format the struct's fields into a string with negative values", func() {
			udq := UplinkDownlinkQueues{
				VF0: -1, VF1: -2, VF2: -3, VF3: -4,
				VF4: -5, VF5: -6, VF6: -7, VF7: -8,
			}

			expectedString := "-1,-2,-3,-4,-5,-6,-7,-8"
			Expect(udq.String()).To(Equal(expectedString))
		})
	})
})

var _ = Describe("ACC100BBDevConfig Validation", func() {
	var (
		config ACC100BBDevConfig
	)

	BeforeEach(func() {
		config = ACC100BBDevConfig{
			NumVfBundles: 1, // Valid value within the expected range.
			Uplink4G:     QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
			Downlink4G:   QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
			Uplink5G:     QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
			Downlink5G:   QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
		}
	})

	Context("when the total number of queue groups is within the maximum limit", func() {
		It("should return an error", func() {
			// Adjusting to not exceed the max limit
			config.Uplink4G.NumQueueGroups = 0
			config.Downlink4G.NumQueueGroups = 0
			err := config.Validate()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when the total number of queue groups is equal to the maximum limit", func() {
		It("should return an error", func() {
			err := config.Validate()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when the total number of queue groups exceeds the maximum limit", func() {
		It("should return an error", func() {
			// Adjusting to exceed the max limit
			config.Uplink4G.NumQueueGroups = 4
			err := config.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprintf("total number of requested queue groups (4G/5G) %v exceeds the maximum (%d)", 10, acc100maxQueueGroups)))
		})
	})
})

var _ = Describe("ACC200BBDevConfig Validation", func() {
	var (
		config ACC200BBDevConfig
	)

	BeforeEach(func() {
		config = ACC200BBDevConfig{
			ACC100BBDevConfig: ACC100BBDevConfig{
				NumVfBundles: 1,
				Uplink4G:     QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
				Downlink4G:   QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
				Uplink5G:     QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
				Downlink5G:   QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
			},
			QFFT: QueueGroupConfig{NumQueueGroups: 4, NumAqsPerGroups: 2, AqDepthLog2: 2},
		}
	})

	Context("when the total number of queue groups is within the maximum limit", func() {
		It("should return an error", func() {
			err := config.Validate()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when the total number of queue groups is equal to the maximum limit", func() {
		It("should return an error", func() {
			config.Uplink5G.NumQueueGroups = 4
			config.Downlink5G.NumQueueGroups = 4
			config.Uplink4G.NumQueueGroups = 2
			config.Downlink4G.NumQueueGroups = 2
			err := config.Validate()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when the total number of queue groups exceeds the maximum limit", func() {
		It("should return an error", func() {
			config.Uplink5G.NumQueueGroups = 4
			config.Downlink5G.NumQueueGroups = 4
			config.Uplink4G.NumQueueGroups = 4
			config.Downlink4G.NumQueueGroups = 4
			err := config.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprintf("total number of requested queue groups (4G/5G/QFFT) %v exceeds the maximum (%d)", 20, acc200maxQueueGroups)))
		})
	})
})

var _ = Describe("BBDevConfig Validation", func() {
	Context("when there are ambiguous configurations", func() {
		It("should return a field forbidden error", func() {
			config := BBDevConfig{
				ACC100: &ACC100BBDevConfig{},
				ACC200: &ACC200BBDevConfig{},
			}
			err := config.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&field.Error{}))
			fieldErr := err.(*field.Error)
			Expect(fieldErr.Type).To(Equal(field.ErrorTypeForbidden))
			Expect(fieldErr.Field).To(Equal("spec.physicalFunction.bbDevConfig"))
			Expect(fieldErr.Detail).To(Equal("specified bbDevConfig cannot contain multiple configurations"))
		})
	})

	Context("when there is a single valid configuration", func() {
		It("should not return an error", func() {
			config := BBDevConfig{
				ACC100: &ACC100BBDevConfig{},
			}
			err := config.Validate()
			Expect(err).ToNot(HaveOccurred())
			Expect(err).To(BeNil())
		})
	})
})
