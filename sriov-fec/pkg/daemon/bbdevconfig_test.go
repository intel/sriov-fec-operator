// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2021 Intel Corporation

package daemon

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	sriovv2 "github.com/otcshare/openshift-operator/sriov-fec/api/v2"
)

func compareFiles(firstFilepath, secondFilepath string) error {
	first, err := ioutil.ReadFile(firstFilepath)
	if err != nil {
		return fmt.Errorf("Unable to open file: %s", firstFilepath)
	}
	second, err2 := ioutil.ReadFile(secondFilepath)
	if err2 != nil {
		return fmt.Errorf("Unable to open file: %s", secondFilepath)
	}
	if !bytes.Equal(first, second) {
		return fmt.Errorf("Different files: %s and %s", firstFilepath, secondFilepath)
	}
	return nil
}

var _ = Describe("bbdevconfig", func() {
	sampleBBDevConfig0 := sriovv2.N3000BBDevConfig{
		PFMode: true,
		Uplink: sriovv2.UplinkDownlink{
			Bandwidth:   8,
			LoadBalance: 128,
			Queues: sriovv2.UplinkDownlinkQueues{
				VF0: 15,
				VF1: 13,
				VF2: 11,
				VF3: 9,
				VF4: 14,
				VF5: 3,
				VF6: 5,
				VF7: 7,
			},
		},
		Downlink: sriovv2.UplinkDownlink{
			Bandwidth:   6,
			LoadBalance: 64,
			Queues: sriovv2.UplinkDownlinkQueues{
				VF0: 16,
				VF1: 8,
				VF2: 4,
				VF3: 2,
				VF4: 6,
				VF5: 1,
				VF6: 0,
				VF7: 0,
			},
		},
		FLRTimeOut: 21,
	}
	sampleBBDevConfig1 := sriovv2.ACC100BBDevConfig{
		PFMode:       true,
		NumVfBundles: 16,
		MaxQueueSize: 1024,
		Uplink4G: sriovv2.QueueGroupConfig{
			NumQueueGroups:  2,
			NumAqsPerGroups: 16,
			AqDepthLog2:     4,
		},
		Downlink4G: sriovv2.QueueGroupConfig{
			NumQueueGroups:  2,
			NumAqsPerGroups: 16,
			AqDepthLog2:     4,
		},
		Uplink5G: sriovv2.QueueGroupConfig{
			NumQueueGroups:  2,
			NumAqsPerGroups: 16,
			AqDepthLog2:     4,
		},
		Downlink5G: sriovv2.QueueGroupConfig{
			NumQueueGroups:  2,
			NumAqsPerGroups: 16,
			AqDepthLog2:     4,
		},
	}
	sampleBBDevConfig2 := sriovv2.ACC100BBDevConfig{
		PFMode:       true,
		NumVfBundles: 16,
		MaxQueueSize: 1024,
		Uplink4G: sriovv2.QueueGroupConfig{
			NumQueueGroups:  4,
			NumAqsPerGroups: 16,
			AqDepthLog2:     4,
		},
		Downlink4G: sriovv2.QueueGroupConfig{
			NumQueueGroups:  4,
			NumAqsPerGroups: 16,
			AqDepthLog2:     4,
		},
		Uplink5G: sriovv2.QueueGroupConfig{
			NumQueueGroups:  4,
			NumAqsPerGroups: 16,
			AqDepthLog2:     4,
		},
		Downlink5G: sriovv2.QueueGroupConfig{
			NumQueueGroups:  4,
			NumAqsPerGroups: 16,
			AqDepthLog2:     4,
		},
	}
	sampleBBDevConfig3 := sriovv2.BBDevConfig{
		N3000: &sampleBBDevConfig0,
	}
	sampleBBDevConfig4 := sriovv2.BBDevConfig{
		ACC100: &sampleBBDevConfig1,
	}
	sampleBBDevConfig5 := sriovv2.BBDevConfig{}
	var _ = Context("generateBBDevConfigFile", func() {
		var _ = It("will create valid config ", func() {
			filename := "config.cfg"
			err := generateN3000BBDevConfigFile(&sampleBBDevConfig0, filepath.Join(testTmpFolder, filename))
			Expect(err).ToNot(HaveOccurred())
			err = compareFiles(filepath.Join(testTmpFolder, filename), "testdata/bbdevconfig_test1.cfg")
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("will return error when config is nil ", func() {
			filename := "config.cfg"
			err := generateN3000BBDevConfigFile(nil, filepath.Join(testTmpFolder, filename))
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will create valid ACC100 config ", func() {
			filename := "config.cfg"
			err := generateACC100BBDevConfigFile(&sampleBBDevConfig1, filepath.Join(testTmpFolder, filename))
			Expect(err).ToNot(HaveOccurred())
			err = compareFiles(filepath.Join(testTmpFolder, filename), "testdata/bbdevconfig_test2.cfg")
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("will return error when ACC100 config is nil ", func() {
			filename := "config.cfg"
			err := generateACC100BBDevConfigFile(nil, filepath.Join(testTmpFolder, filename))
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will return error when total number of queue groups for ACC100 exceeds 8 ", func() {
			filename := "config.cfg"
			err := generateACC100BBDevConfigFile(&sampleBBDevConfig2, filepath.Join(testTmpFolder, filename))
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will create valid N3000 config ", func() {
			filename := "config.cfg"
			err := generateBBDevConfigFile(sampleBBDevConfig3, filepath.Join(testTmpFolder, filename))
			Expect(err).ToNot(HaveOccurred())
			err = compareFiles(filepath.Join(testTmpFolder, filename), "testdata/bbdevconfig_test1.cfg")
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("will create valid ACC100 config ", func() {
			filename := "config.cfg"
			err := generateBBDevConfigFile(sampleBBDevConfig4, filepath.Join(testTmpFolder, filename))
			Expect(err).ToNot(HaveOccurred())
			err = compareFiles(filepath.Join(testTmpFolder, filename), "testdata/bbdevconfig_test2.cfg")
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("will return an error when N3000 and ACC100 configs are nil ", func() {
			filename := "config.cfg"
			err := generateBBDevConfigFile(sampleBBDevConfig5, filepath.Join(testTmpFolder, filename))
			Expect(err).To(HaveOccurred())
		})
	})
})
