// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2024 Intel Corporation

package v2

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("UplinkDownlink DeepCopy", func() {
	var (
		originalConfig *UplinkDownlink
		copiedConfig   *UplinkDownlink
	)

	BeforeEach(func() {
		originalConfig = &UplinkDownlink{Bandwidth: 3, LoadBalance: 128, Queues: UplinkDownlinkQueues{}}
	})

	Describe("executing DeepCopy", func() {
		BeforeEach(func() {
			copiedConfig = originalConfig.DeepCopy() // Perform the deep copied
		})

		It("should create an exact copied of Config", func() {
			Expect(copiedConfig).To(Equal(originalConfig))
		})
	})

	Describe("executing DeepCopyInto", func() {
		BeforeEach(func() {
			copiedConfig = &UplinkDownlink{}
			originalConfig.DeepCopyInto(copiedConfig)
		})

		It("should create an exact copied of Config", func() {
			Expect(copiedConfig).To(Equal(originalConfig))
		})
	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			originalConfig = nil
			copiedConfig = originalConfig.DeepCopy()
		})

		It("should set copiedConfig as nil", func() {
			Expect(copiedConfig).To(BeNil())
		})
	})
})

var _ = Describe("UplinkDownlinkQueues  DeepCopy Tests", func() {
	var original *UplinkDownlinkQueues
	var copied *UplinkDownlinkQueues

	BeforeEach(func() {
		drain := new(bool)
		*drain = true
		original = &UplinkDownlinkQueues{VF0: 4, VF1: 4, VF2: 4, VF3: 4, VF4: 4, VF5: 4, VF6: 4, VF7: 4}
	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should return an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = &UplinkDownlinkQueues{}
			original.DeepCopyInto(copied)
		})

		It("should create an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})

	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			original = nil
			copied = original.DeepCopy()
		})

		It("should set copied as nil", func() {
			Expect(copied).To(BeNil())
		})
	})
})

var _ = Describe("SriovFecClusterConfigList DeepCopy Tests", func() {
	var original *SriovFecClusterConfigList
	var copied *SriovFecClusterConfigList

	BeforeEach(func() {
		drain := new(bool)
		*drain = true
		original = &SriovFecClusterConfigList{
			Items: []SriovFecClusterConfig{
				{Spec: SriovFecClusterConfigSpec{Priority: 5, DrainSkip: drain}, Status: SriovFecClusterConfigStatus{SyncStatus: "", LastSyncError: ""}},
				{Spec: SriovFecClusterConfigSpec{Priority: 2, DrainSkip: drain}, Status: SriovFecClusterConfigStatus{SyncStatus: "", LastSyncError: ""}},
			},
		}
	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should return an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = &SriovFecClusterConfigList{}
			original.DeepCopyInto(copied)
		})

		It("should create an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})

	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			original = nil
			copied = original.DeepCopy()
		})

		It("should set copied as nil", func() {
			Expect(copied).To(BeNil())
		})
	})
})

var _ = Describe("N3000BBDevConfig DeepCopy", func() {
	var (
		originalConfig *N3000BBDevConfig
		copiedConfig   *N3000BBDevConfig
	)

	BeforeEach(func() {
		originalConfig = &N3000BBDevConfig{
			NetworkType: "FPGA_5GNR",
			FLRTimeOut:  4,
			Uplink:      UplinkDownlink{Queues: UplinkDownlinkQueues{VF0: 1, VF1: 2}},
			Downlink:    UplinkDownlink{Queues: UplinkDownlinkQueues{VF0: 3, VF1: 4}},
		}
	})

	Describe("executing DeepCopy", func() {
		BeforeEach(func() {
			copiedConfig = originalConfig.DeepCopy() // Perform the deep copied
		})

		It("should create an exact copied of N3000BBDevConfig", func() {
			Expect(copiedConfig).To(Equal(originalConfig))
		})

		Context("when the copied config is modified", func() {
			BeforeEach(func() {
				copiedConfig.Uplink.Queues.VF0 = 10
			})

			It("should not affect the original config", func() {
				Expect(originalConfig.Uplink.Queues.VF0).To(Equal(1))
			})
		})

		Context("when the original config is nil", func() {
			BeforeEach(func() {
				originalConfig = nil
				copiedConfig = originalConfig.DeepCopy()
			})

			It("should set copiedConfig as nil", func() {
				Expect(copiedConfig).To(BeNil())
			})
		})
	})

	Describe("executing DeepCopyInto", func() {
		BeforeEach(func() {
			copiedConfig = &N3000BBDevConfig{}
			originalConfig.DeepCopyInto(copiedConfig)
		})

		It("should create an exact copied of N3000BBDevConfig", func() {
			Expect(copiedConfig).To(Equal(originalConfig))
		})

		Context("when the copied config is modified", func() {
			BeforeEach(func() {
				// Modify the copied struct
				copiedConfig.Uplink.Queues.VF0 = 10
			})

			It("should not affect the original config", func() {
				// Ensure the original struct is unaffected by changes to the copied
				Expect(originalConfig.Uplink.Queues.VF0).To(Equal(1))
			})
		})
	})
})

var _ = Describe("ACC100BBDevConfig DeepCopy", func() {
	var (
		originalConfig *ACC100BBDevConfig
		copiedConfig   *ACC100BBDevConfig
	)

	BeforeEach(func() {
		originalConfig = &ACC100BBDevConfig{
			NumVfBundles: 2,
			MaxQueueSize: 1024,
			Uplink4G:     QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
			Downlink4G:   QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
			Uplink5G:     QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
			Downlink5G:   QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
		}
	})

	Describe("executing DeepCopy", func() {
		BeforeEach(func() {
			copiedConfig = originalConfig.DeepCopy() // Perform the deep copied
		})

		It("should create an exact copied of ACC100BBDevConfig", func() {
			Expect(copiedConfig).To(Equal(originalConfig))
		})

		Context("when the copied config is modified", func() {
			BeforeEach(func() {
				copiedConfig.Uplink4G.NumQueueGroups = 0
				copiedConfig.Downlink4G.NumQueueGroups = 0
			})

			It("should not affect the original config", func() {
				Expect(originalConfig.Uplink4G.NumQueueGroups).To(Equal(2))
			})
		})

		Context("when the original config is nil", func() {
			BeforeEach(func() {
				originalConfig = nil
				copiedConfig = originalConfig.DeepCopy()
			})

			It("should set copiedConfig as nil", func() {
				Expect(copiedConfig).To(BeNil())
			})
		})
	})

	Describe("executing DeepCopyInto", func() {
		BeforeEach(func() {
			copiedConfig = &ACC100BBDevConfig{}
			originalConfig.DeepCopyInto(copiedConfig)
		})

		It("should create an exact copied of ACC100BBDevConfig", func() {
			Expect(copiedConfig).To(Equal(originalConfig))
		})

		Context("when the copied config is modified", func() {
			BeforeEach(func() {
				// Modify the copied struct
				copiedConfig.Uplink4G.NumQueueGroups = 0
			})

			It("should not affect the original config", func() {
				// Ensure the original struct is unaffected by changes to the copied
				Expect(originalConfig.Uplink4G.NumQueueGroups).To(Equal(2))
			})
		})
	})
})

var _ = Describe("ACC200BBDevConfig DeepCopy", func() {
	var (
		originalConfig *ACC200BBDevConfig
		copiedConfig   *ACC200BBDevConfig
	)

	BeforeEach(func() {
		originalConfig = &ACC200BBDevConfig{
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

	Describe("executing DeepCopy", func() {
		BeforeEach(func() {
			copiedConfig = originalConfig.DeepCopy() // Perform the deep copied
		})

		It("should create an exact copied of ACC200BBDevConfig", func() {
			Expect(copiedConfig).To(Equal(originalConfig))
		})

		Context("when the copied config is modified", func() {
			BeforeEach(func() {
				copiedConfig.Uplink4G.NumQueueGroups = 0
				copiedConfig.Downlink4G.NumQueueGroups = 0
			})

			It("should not affect the original config", func() {
				Expect(originalConfig.Uplink4G.NumQueueGroups).To(Equal(2))
			})
		})

		Context("when the original config is nil", func() {
			BeforeEach(func() {
				originalConfig = nil
				copiedConfig = originalConfig.DeepCopy()
			})

			It("should set copiedConfig as nil", func() {
				Expect(copiedConfig).To(BeNil())
			})
		})
	})

	Describe("executing DeepCopyInto", func() {
		BeforeEach(func() {
			copiedConfig = &ACC200BBDevConfig{}
			originalConfig.DeepCopyInto(copiedConfig)
		})

		It("should create an exact copied of ACC200BBDevConfig", func() {
			Expect(copiedConfig).To(Equal(originalConfig))
		})

		Context("when the copied config is modified", func() {
			BeforeEach(func() {
				// Modify the copied struct
				copiedConfig.Uplink4G.NumQueueGroups = 0
			})

			It("should not affect the original config", func() {
				// Ensure the original struct is unaffected by changes to the copied
				Expect(originalConfig.Uplink4G.NumQueueGroups).To(Equal(2))
			})
		})
	})
})

var _ = Describe("AcceleratorSelector DeepCopy Tests", func() {
	var original *AcceleratorSelector
	var copied *AcceleratorSelector

	BeforeEach(func() {
		original = &AcceleratorSelector{
			VendorID:   "8086",
			DeviceID:   "1234",
			PCIAddress: "0000:00:02.0",
			PFDriver:   "vfio-pci",
			MaxVFs:     16,
		}
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = &AcceleratorSelector{}
			original.DeepCopyInto(copied)
		})

		It("should create an exact copied of AcceleratorSelector", func() {
			Expect(copied).To(Equal(original))
		})

	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should return an exact copied of AcceleratorSelector", func() {
			Expect(copied).To(Equal(original))
		})
	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			original = nil
			copied = original.DeepCopy()
		})

		It("should set copied as nil", func() {
			Expect(copied).To(BeNil())
		})
	})
})

var _ = Describe("BBDevConfig DeepCopy Tests", func() {
	var original *BBDevConfig
	var copied *BBDevConfig

	BeforeEach(func() {
		original = &BBDevConfig{
			N3000: &N3000BBDevConfig{
				NetworkType: "FPGA_5GNR",
				PFMode:      false,
				FLRTimeOut:  60,
				Downlink:    UplinkDownlink{Bandwidth: 3, LoadBalance: 128, Queues: UplinkDownlinkQueues{VF0: 1, VF1: 1}},
				Uplink:      UplinkDownlink{Bandwidth: 3, LoadBalance: 128, Queues: UplinkDownlinkQueues{VF0: 1, VF1: 1}},
			},
			ACC100: &ACC100BBDevConfig{NumVfBundles: 1,
				Uplink4G:   QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
				Downlink4G: QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
				Uplink5G:   QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
				Downlink5G: QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
			},
			ACC200: &ACC200BBDevConfig{
				ACC100BBDevConfig: ACC100BBDevConfig{
					NumVfBundles: 1,
					Uplink4G:     QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
					Downlink4G:   QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
					Uplink5G:     QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
					Downlink5G:   QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2},
				},
				QFFT: QueueGroupConfig{NumQueueGroups: 4, NumAqsPerGroups: 2, AqDepthLog2: 2},
			},
		}
	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should create a deep copied, not a shallow copied", func() {
			Expect(copied).ToNot(BeIdenticalTo(original))
		})

		It("should deep copied N3000 field correctly", func() {
			Expect(copied.N3000).ToNot(BeIdenticalTo(original.N3000))
			Expect(copied.N3000).To(Equal(original.N3000))
		})

		It("should deep copied ACC100 field correctly", func() {
			Expect(copied.ACC100).ToNot(BeIdenticalTo(original.ACC100))
			Expect(copied.ACC100).To(Equal(original.ACC100))
		})

		It("should deep copied ACC200 field correctly", func() {
			Expect(copied.ACC200).ToNot(BeIdenticalTo(original.ACC200))
			Expect(copied.ACC200).To(Equal(original.ACC200))
		})

		It("should handle nil fields correctly", func() {
			// Set one of the fields to nil and test DeepCopy
			original.N3000 = nil
			copied = original.DeepCopy()
			Expect(copied.N3000).To(BeNil())
			Expect(copied.ACC100).ToNot(BeNil()) // Ensure other fields are still copied correctly
		})
	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			original = nil
			copied = original.DeepCopy()
		})

		It("should set copied as nil", func() {
			Expect(copied).To(BeNil())
		})
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = &BBDevConfig{}
			original.DeepCopyInto(copied)
		})
		It("should create a deep copied, not a shallow copied, using DeepCopyInto", func() {
			Expect(copied).ToNot(BeIdenticalTo(original))
		})

		It("should deep copied N3000 field correctly using DeepCopyInto", func() {
			Expect(copied.N3000).ToNot(BeIdenticalTo(original.N3000))
			Expect(copied.N3000).To(Equal(original.N3000))
		})

		It("should deep copied ACC100 field correctly using DeepCopyInto", func() {
			Expect(copied.ACC100).ToNot(BeIdenticalTo(original.ACC100))
			Expect(copied.ACC100).To(Equal(original.ACC100))
		})

		It("should deep copied ACC200 field correctly using DeepCopyInto", func() {
			Expect(copied.ACC200).ToNot(BeIdenticalTo(original.ACC200))
			Expect(copied.ACC200).To(Equal(original.ACC200))
		})

		It("should handle nil fields correctly using DeepCopyInto", func() {
			original = &BBDevConfig{
				N3000:  nil,
				ACC100: nil,
				ACC200: nil,
			}
			copied = new(BBDevConfig)     // Create a new instance for the copied
			original.DeepCopyInto(copied) // Use DeepCopyInto to copied the data

			Expect(copied.N3000).To(BeNil())
			Expect(copied.ACC100).To(BeNil())
			Expect(copied.ACC200).To(BeNil())
		})
	})
})

var _ = Describe("ByPriority DeepCopy Tests", func() {
	var original ByPriority
	var copied ByPriority

	BeforeEach(func() {
		original = ByPriority{
			{Spec: SriovFecClusterConfigSpec{Priority: 3}},
			{Spec: SriovFecClusterConfigSpec{Priority: 1}},
			{Spec: SriovFecClusterConfigSpec{Priority: 4}},
			{Spec: SriovFecClusterConfigSpec{Priority: 2}},
		}
	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should create a deep copied, not a shallow copied", func() {
			Expect(&copied).ToNot(BeIdenticalTo(&original))
		})

		It("should deep copied all elements correctly", func() {
			for i := range original {
				Expect(copied[i]).To(Equal(original[i]))
				Expect(&copied[i]).ToNot(BeIdenticalTo(&original[i]))
			}
		})

		It("should handle nil slices correctly", func() {
			original = nil
			copied = original.DeepCopy()
			Expect(copied).To(BeNil())
		})
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = make(ByPriority, len(original))
			original.DeepCopyInto(&copied)
		})

		It("should create a deep copied, not a shallow copied, using DeepCopyInto", func() {
			Expect(&copied).ToNot(BeIdenticalTo(&original))
		})

		It("should deep copied all elements correctly using DeepCopyInto", func() {
			for i := range original {
				Expect(copied[i]).To(Equal(original[i]))
				Expect(&copied[i]).ToNot(BeIdenticalTo(&original[i]))
			}
		})

	})
})

var _ = Describe("FFTLutParam DeepCopy Tests", func() {
	var original *FFTLutParam
	var copied *FFTLutParam

	BeforeEach(func() {
		original = &FFTLutParam{
			FftUrl:      "http://abc123/xyz.tar.gz",
			FftChecksum: "387bfc0ee19860ffd4ad852b26f8469e8a93032e",
		}
	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should create a new struct that is a deep copy of the original", func() {
			Expect(copied).To(BeEquivalentTo(original))
			Expect(copied).NotTo(BeIdenticalTo(original))
		})
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = &FFTLutParam{}
			original.DeepCopyInto(copied)
		})

		It("should copy all fields from the original to the copy", func() {
			Expect(copied.FftChecksum).To(Equal(original.FftChecksum))
			Expect(copied.FftUrl).To(Equal(original.FftUrl))
		})
	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			original = nil
			copied = original.DeepCopy()
		})

		It("should set copied as nil", func() {
			Expect(copied).To(BeNil())
		})
	})
})

var _ = Describe("NodeInventory DeepCopy Tests", func() {
	var original *NodeInventory
	var copied *NodeInventory

	BeforeEach(func() {
		original = &NodeInventory{
			SriovAccelerators: []SriovAccelerator{
				{VendorID: "8086", DeviceID: "1234", PCIAddress: "0000:00:02.0", PFDriver: "vfio-pci", MaxVFs: 16, VFs: []VF{}},
				{VendorID: "8086", DeviceID: "1234", PCIAddress: "0000:1a:00.1", PFDriver: "vfio-pci", MaxVFs: 32},
			},
		}
	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should create a new struct that is a deep copy of the original", func() {
			Expect(copied).To(BeEquivalentTo(original))
			Expect(copied).NotTo(BeIdenticalTo(original))
		})

		It("should deep copied all elements correctly", func() {
			for i := range original.SriovAccelerators {
				Expect(copied.SriovAccelerators[i]).To(Equal(original.SriovAccelerators[i]))
				Expect(&copied.SriovAccelerators[i]).ToNot(BeIdenticalTo(&original.SriovAccelerators[i]))
			}
		})

		It("should not link the copied slice to the original", func() {
			copied.SriovAccelerators[0].MaxVFs = 8
			Expect(original.SriovAccelerators[0].MaxVFs).NotTo(Equal(copied.SriovAccelerators[0].MaxVFs))
		})

		It("should handle nil slices correctly", func() {
			original = nil
			copied = original.DeepCopy()
			Expect(copied).To(BeNil())
		})
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = &NodeInventory{
				SriovAccelerators: []SriovAccelerator{},
			}
			original.DeepCopyInto(copied)
		})

		It("should copy all fields from the original to the copy", func() {
			Expect(copied.SriovAccelerators).To(Equal(original.SriovAccelerators))
		})

		It("should deep copied all elements correctly using DeepCopyInto", func() {
			for i := range original.SriovAccelerators {
				Expect(copied.SriovAccelerators[i]).To(Equal(original.SriovAccelerators[i]))
				Expect(&copied.SriovAccelerators[i]).ToNot(BeIdenticalTo(&original.SriovAccelerators[i]))
			}
		})

		It("should not link the copied slice to the original", func() {
			copied.SriovAccelerators[0].MaxVFs = 8
			Expect(original.SriovAccelerators[0].MaxVFs).NotTo(Equal(copied.SriovAccelerators[0].MaxVFs))
		})
	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			original = nil
			copied = original.DeepCopy()
		})

		It("should set copied as nil", func() {
			Expect(copied).To(BeNil())
		})
	})
})

var _ = Describe("PhysicalFunctionConfig DeepCopy Tests", func() {
	var original *PhysicalFunctionConfig
	var copied *PhysicalFunctionConfig

	BeforeEach(func() {
		original = &PhysicalFunctionConfig{
			PFDriver:    "vfio-pci",
			VFDriver:    "vfio-pci",
			VFAmount:    2,
			BBDevConfig: BBDevConfig{},
		}
	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should return an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = &PhysicalFunctionConfig{}
			original.DeepCopyInto(copied)
		})

		It("should create an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})

	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			original = nil
			copied = original.DeepCopy()
		})

		It("should set copied as nil", func() {
			Expect(copied).To(BeNil())
		})
	})
})

var _ = Describe("PhysicalFunctionConfigExt DeepCopy Tests", func() {
	var original *PhysicalFunctionConfigExt
	var copied *PhysicalFunctionConfigExt

	BeforeEach(func() {
		original = &PhysicalFunctionConfigExt{
			PCIAddress:  "8a:00.0",
			PFDriver:    "vfio-pci",
			VFDriver:    "vfio-pci",
			VFAmount:    2,
			BBDevConfig: BBDevConfig{},
		}
	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should return an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = &PhysicalFunctionConfigExt{}
			original.DeepCopyInto(copied)
		})

		It("should create an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})

	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			original = nil
			copied = original.DeepCopy()
		})

		It("should set copied as nil", func() {
			Expect(copied).To(BeNil())
		})
	})
})

var _ = Describe("QueueGroupConfig DeepCopy Tests", func() {
	var original *QueueGroupConfig
	var copied *QueueGroupConfig

	BeforeEach(func() {
		original = &QueueGroupConfig{NumQueueGroups: 2, NumAqsPerGroups: 2, AqDepthLog2: 2}
	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should return an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = &QueueGroupConfig{}
			original.DeepCopyInto(copied)
		})

		It("should create an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})

	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			original = nil
			copied = original.DeepCopy()
		})

		It("should set copied as nil", func() {
			Expect(copied).To(BeNil())
		})
	})
})

var _ = Describe("SriovFecClusterConfigList DeepCopy Tests", func() {
	var original *SriovFecClusterConfigList
	var copied *SriovFecClusterConfigList

	BeforeEach(func() {
		drain := new(bool)
		*drain = true
		original = &SriovFecClusterConfigList{
			Items: []SriovFecClusterConfig{
				{Spec: SriovFecClusterConfigSpec{NodeSelector: map[string]string{"k1": "v1", "k2": "v2"}, Priority: 5, DrainSkip: drain}, Status: SriovFecClusterConfigStatus{SyncStatus: "", LastSyncError: ""}},
				{Spec: SriovFecClusterConfigSpec{Priority: 2, DrainSkip: drain}, Status: SriovFecClusterConfigStatus{SyncStatus: "", LastSyncError: ""}},
			},
		}
	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should return an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = &SriovFecClusterConfigList{}
			original.DeepCopyInto(copied)
		})

		It("should create an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})

	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			original = nil
			copied = original.DeepCopy()
		})

		It("should set copied as nil", func() {
			Expect(copied).To(BeNil())
		})
	})
})

var _ = Describe("SriovFecClusterConfigStatus DeepCopy Tests", func() {
	var original *SriovFecClusterConfigStatus
	var copied *SriovFecClusterConfigStatus

	BeforeEach(func() {
		original = &SriovFecClusterConfigStatus{}
	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should return an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = &SriovFecClusterConfigStatus{}
			original.DeepCopyInto(copied)
		})

		It("should create an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})

	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			original = nil
			copied = original.DeepCopy()
		})

		It("should set copied as nil", func() {
			Expect(copied).To(BeNil())
		})
	})
})

var _ = Describe("VF DeepCopy Tests", func() {
	var original *VF
	var copied *VF

	BeforeEach(func() {
		original = &VF{PCIAddress: "32:00.0", Driver: "vfio-pci", DeviceID: "1234"}
	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should return an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = &VF{}
			original.DeepCopyInto(copied)
		})

		It("should create an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})

	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			original = nil
			copied = original.DeepCopy()
		})

		It("should set copied as nil", func() {
			Expect(copied).To(BeNil())
		})
	})
})

var _ = Describe("SriovFecNodeConfig DeepCopy Tests", func() {
	var original *SriovFecNodeConfig
	var copied *SriovFecNodeConfig

	BeforeEach(func() {
		original = &SriovFecNodeConfig{Spec: SriovFecNodeConfigSpec{}, Status: SriovFecNodeConfigStatus{}}
	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should return an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = &SriovFecNodeConfig{}
			original.DeepCopyInto(copied)
		})

		It("should create an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})

	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			original = nil
			copied = original.DeepCopy()
		})

		It("should set copied as nil", func() {
			Expect(copied).To(BeNil())
		})
	})
})

var _ = Describe("SriovFecNodeConfig DeepCopy Tests", func() {
	var original *SriovFecNodeConfigList
	var copied *SriovFecNodeConfigList

	BeforeEach(func() {
		original = &SriovFecNodeConfigList{Items: []SriovFecNodeConfig{}}
	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should return an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = &SriovFecNodeConfigList{}
			original.DeepCopyInto(copied)
		})

		It("should create an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})

	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			original = nil
			copied = original.DeepCopy()
		})

		It("should set copied as nil", func() {
			Expect(copied).To(BeNil())
		})
	})
})

var _ = Describe("SriovFecNodeConfigSpec  DeepCopy Tests", func() {
	var original *SriovFecNodeConfigSpec
	var copied *SriovFecNodeConfigSpec

	BeforeEach(func() {
		original = &SriovFecNodeConfigSpec{PhysicalFunctions: []PhysicalFunctionConfigExt{}, DrainSkip: true}
	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should return an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = &SriovFecNodeConfigSpec{}
			original.DeepCopyInto(copied)
		})

		It("should create an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})

	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			original = nil
			copied = original.DeepCopy()
		})

		It("should set copied as nil", func() {
			Expect(copied).To(BeNil())
		})
	})
})

var _ = Describe("SriovFecNodeConfigStatus DeepCopy Tests", func() {
	var original *SriovFecNodeConfigStatus
	var copied *SriovFecNodeConfigStatus

	BeforeEach(func() {
		original = &SriovFecNodeConfigStatus{PfBbConfVersion: "23.1", Conditions: []v1.Condition{}, Inventory: NodeInventory{}}
	})

	Describe("DeepCopy", func() {
		BeforeEach(func() {
			copied = original.DeepCopy()
		})

		It("should return an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})
	})

	Describe("DeepCopyInto", func() {
		BeforeEach(func() {
			copied = &SriovFecNodeConfigStatus{}
			original.DeepCopyInto(copied)
		})

		It("should create an exact copy of ", func() {
			Expect(copied).To(Equal(original))
		})

	})

	Context("when the original config is nil", func() {
		BeforeEach(func() {
			original = nil
			copied = original.DeepCopy()
		})

		It("should set copied as nil", func() {
			Expect(copied).To(BeNil())
		})
	})
})
