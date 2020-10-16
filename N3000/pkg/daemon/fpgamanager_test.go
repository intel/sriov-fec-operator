// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	bmcOutput = `Board Management Controller, MAX10 NIOS FW version D.2.0.12
Board Management Controller, MAX10 Build version D.2.0.5
//****** BMC SENSORS ******//
Object Id                     : 0xEF00000
PCIe s:b:d.f                  : 0000:1b:00.0
Device Id                     : 0x0b30
Numa Node                     : 0
Ports Num                     : 01
Bitstream Id                  : 0x21000000000000
Bitstream Version             : 1.0.0
Pr Interface Id               : 12345678-abcd-efgh-ijkl-0123456789ab
( 1) Board Power              : 69.24 Watts
( 2) 12V Backplane Current    : 2.75 Amps
( 3) 12V Backplane Voltage    : 12.06 Volts
( 4) 1.2V Voltage             : 1.19 Volts
( 6) 1.8V Voltage             : 1.80 Volts
( 8) 3.3V Voltage             : 3.26 Volts
(10) FPGA Core Voltage        : 0.90 Volts
(11) FPGA Core Current        : 20.99 Amps
(12) FPGA Die Temperature     : 61.50 Celsius
(13) Board Temperature        : 30.00 Celsius
(14) QSFP0 Supply Voltage     : N/A
(15) QSFP0 Temperature        : N/A
(24) 12V AUX Current          : 3.10 Amps
(25) 12V AUX Voltage          : 11.64 Volts
(37) QSFP1 Supply Voltage     : N/A
(38) QSFP1 Temperature        : N/A
(44) PKVL0 Core Temperature   : 56.50 Celsius
(45) PKVL0 SerDes Temperature : 57.00 Celsius
(46) PKVL1 Core Temperature   : 57.00 Celsius
(47) PKVL1 SerDes Temperature : 57.50 Celsius
Board Management Controller, MAX10 NIOS FW version D.2.0.12
Board Management Controller, MAX10 Build version D.2.0.5
//****** BMC SENSORS ******//
Object Id                     : 0xEF00000
PCIe s:b:d.f                  : 0000:2b:00.0
Device Id                     : 0x0b30
Numa Node                     : 1
Ports Num                     : 01
Bitstream Id                  : 0x32000000000000
Bitstream Version             : 2.0.0
Pr Interface Id               : 87654321-abcd-efgh-ijkl-0123456789ab
( 1) Board Power              : 70.25 Watts
( 2) 12V Backplane Current    : 2.79 Amps
( 3) 12V Backplane Voltage    : 12.06 Volts
( 4) 1.2V Voltage             : 1.19 Volts
( 6) 1.8V Voltage             : 1.80 Volts
( 8) 3.3V Voltage             : 3.26 Volts
(10) FPGA Core Voltage        : 0.90 Volts
(11) FPGA Core Current        : 21.19 Amps
(12) FPGA Die Temperature     : 63.00 Celsius
(13) Board Temperature        : 31.00 Celsius
(14) QSFP0 Supply Voltage     : N/A
(15) QSFP0 Temperature        : N/A
(24) 12V AUX Current          : 3.14 Amps
(25) 12V AUX Voltage          : 11.64 Volts
(37) QSFP1 Supply Voltage     : N/A
(38) QSFP1 Temperature        : N/A
(44) PKVL0 Core Temperature   : 58.00 Celsius
(45) PKVL0 SerDes Temperature : 58.00 Celsius
(46) PKVL1 Core Temperature   : 58.50 Celsius
(47) PKVL1 SerDes Temperature : 59.00 Celsius
`
)

func fakeFpgaInfo(cmd string) (string, error) {
	if cmd == "bmc" {
		return bmcOutput, nil
	}
	return "", fmt.Errorf("Unsupported command: %s", cmd)
}

func TestMain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Main suite")
}

var _ = Describe("FPGA Manager", func() {
	var _ = Describe("getFPGAInfo", func() {
		var _ = It("will return valid []N3000FpgaStatus ", func() {
			fpgaInfoExec = fakeFpgaInfo
			result, err := getFPGAInventory()

			Expect(err).ToNot(HaveOccurred())
			Expect(len(result)).To(Equal(2))

			Expect(result[0].PciAddr).To(Equal("0000:1b:00.0"))
			Expect(result[0].DeviceID).To(Equal("0x0b30"))
			Expect(result[0].BitstreamID).To(Equal("0x21000000000000"))
			Expect(result[0].BitstreamVersion).To(Equal("1.0.0"))
			Expect(result[0].NumaNode).To(Equal(0))

			Expect(result[1].PciAddr).To(Equal("0000:2b:00.0"))
			Expect(result[1].DeviceID).To(Equal("0x0b30"))
			Expect(result[1].BitstreamID).To(Equal("0x32000000000000"))
			Expect(result[1].BitstreamVersion).To(Equal("2.0.0"))
			Expect(result[1].NumaNode).To(Equal(1))

		})
	})
})
