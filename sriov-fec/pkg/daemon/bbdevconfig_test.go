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
	sriovv1 "github.com/otcshare/openshift-operator/sriov-fec/api/v1"
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
	sampleBBDevConfig0 := sriovv1.N3000BBDevConfig{
		PFMode: true,
		Uplink: sriovv1.UplinkDownlink{
			Bandwidth:   8,
			LoadBalance: 128,
			Queues: sriovv1.UplinkDownlinkQueues{
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
		Downlink: sriovv1.UplinkDownlink{
			Bandwidth:   6,
			LoadBalance: 64,
			Queues: sriovv1.UplinkDownlinkQueues{
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
	})
})
