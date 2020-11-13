// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"io/ioutil"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	nvmupdateOutput = `<?xml version="1.0" encoding="UTF-8"?>
<DeviceUpdate lang="en">
        <Instance vendor="8086" device="1572" subdevice="0" subvendor="8086" bus="7" dev="0" func="1" PBA="H58362-002" port_id="Port 2 of 2" display="Intel(R) Ethernet Converged Network Adapter X710">
                <Module type="PXE" version="1.0.2" display="">
                </Module>
                <Module type="EFI" version="1.0.5" display="">
                </Module>
                <Module type="NVM" version="8000191B" previous_version="8000143F" display="">
                        <Status result="Success" id="0">All operations completed successfully.</Status>
                </Module>
                <VPD>
                        <VPDField type="String">XL710 40GbE Controller</VPDField>
                        <VPDField type="Readable" key="PN"></VPDField>
                        <VPDField type="Readable" key="EC"></VPDField>
                        <VPDField type="Readable" key="FG"></VPDField>
                        <VPDField type="Readable" key="LC"></VPDField>
                        <VPDField type="Readable" key="MN"></VPDField>
                        <VPDField type="Readable" key="PG"></VPDField>
                        <VPDField type="Readable" key="SN"></VPDField>
                        <VPDField type="Readable" key="V0"></VPDField>
                        <VPDField type="Checksum" key="RV">86</VPDField>
                        <VPDField type="Writable" key="V1"></VPDField>
                </VPD>
                <MACAddresses>
                        <MAC address="6805CA3AA725">
                        </MAC>
                        <SAN address="6805CA3AA727">
                        </SAN>
                </MACAddresses>
        </Instance>
        <NextUpdateAvailable> 1 </NextUpdateAvailable>
        <RebootRequired> 0 </RebootRequired>
        <PowerCycleRequired> 1 </PowerCycleRequired>
</DeviceUpdate>`
)

var _ = Describe("getDeviceUpdateFromFile", func() {
	var _ = It("will return valid DeviceUpdate ", func() {

		tmpfile, err := ioutil.TempFile(".", "update")
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte(nvmupdateOutput))
		Expect(err).ToNot(HaveOccurred())

		result, err := getDeviceUpdateFromFile(tmpfile.Name())

		Expect(err).ToNot(HaveOccurred())
		Expect(len(result.Modules)).To(Equal(3))

		Expect(result.Modules[0].Type).To(Equal("PXE"))
		Expect(result.Modules[0].Version).To(Equal("1.0.2"))
		Expect(result.Modules[0].Status).To(Equal(moduleStatus{}))

		Expect(result.Modules[1].Type).To(Equal("EFI"))
		Expect(result.Modules[1].Version).To(Equal("1.0.5"))
		Expect(result.Modules[1].Status).To(Equal(moduleStatus{}))

		Expect(result.Modules[2].Type).To(Equal("NVM"))
		Expect(result.Modules[2].Version).To(Equal("8000191B"))
		Expect(result.Modules[2].Status).To(Equal(moduleStatus{Result: "Success"}))

		Expect(result.NextUpdateAvailable).To(Equal(1))
	})
})

var _ = Describe("getDeviceUpdateFromFile", func() {
	var _ = It("will return error if too large file is provided", func() {

		tmpfile, err := ioutil.TempFile(".", "update")
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(tmpfile.Name())

		blob := make([]byte, 50000)
		_, err = tmpfile.Write([]byte(blob))

		Expect(err).ToNot(HaveOccurred())

		_, err = getDeviceUpdateFromFile(tmpfile.Name())

		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("getDeviceUpdateFromFile", func() {
	var _ = It("will return error if parsing xml exceeds timeout value", func() {

		tmpfile, err := ioutil.TempFile(".", "update")
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte(nvmupdateOutput))
		Expect(err).ToNot(HaveOccurred())

		updateXMLParseTimeout = 1 * time.Nanosecond
		_, err = getDeviceUpdateFromFile(tmpfile.Name())
		updateXMLParseTimeout = 100 * time.Millisecond

		Expect(err).To(HaveOccurred())
	})
})
