// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"fmt"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	fpgav1 "github.com/smart-edge-open/openshift-operator/N3000/api/v1"
	"k8s.io/klog/klogr"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"
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
(12) FPGA Die Temperature     : 73.00 Celsius
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
(12) FPGA Die Temperature     : 98.50 Celsius
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

var (
	fakeFpgaInfoErrReturn    error = nil
	fakeFpgasUpdateErrReturn error = nil
	fakeRsuUpdateErrReturn   error = nil
)

func mockFPGAEnv() {
	fpgaUserImageSubfolderPath = testTmpFolder
	fpgaUserImageFile = filepath.Join(testTmpFolder, "fpga")
}

func fakeFpgaInfo(cmd *exec.Cmd, log logr.Logger, dryRun bool) (string, error) {
	if strings.Contains(cmd.String(), "fpgainfo bmc") {
		return bmcOutput, fakeFpgaInfoErrReturn
	}
	return "", fmt.Errorf("Unsupported command: %s", cmd)
}

func fakeFpgasUpdate(cmd *exec.Cmd, log logr.Logger, dryRun bool) error {
	if strings.Contains(cmd.String(), "fpgasupdate") {
		return fakeFpgasUpdateErrReturn
	}
	return fmt.Errorf("Unsupported command: %s", cmd)
}

func fakeRsu(cmd *exec.Cmd, log logr.Logger, dryRun bool) error {
	if strings.Contains(cmd.String(), "rsu") && strings.Contains(cmd.String(), "bmcimg") {
		return fakeRsuUpdateErrReturn
	}
	return fmt.Errorf("Unsupported command: %s", cmd)
}

func cleanFPGA() {
	fakeFpgaInfoErrReturn = nil
	fakeFpgasUpdateErrReturn = nil
	fakeRsuUpdateErrReturn = nil

	err := os.Setenv(envTemperatureLimitName, fmt.Sprintf("%f", fpgaTemperatureDefaultLimit))
	Expect(err).ToNot(HaveOccurred())
}

var _ = Describe("FPGA Manager", func() {
	log := klogr.New().WithName("fpgamanager-Test")
	f := FPGAManager{Log: ctrl.Log.WithName("daemon-test")}
	sampleOneFPGA := fpgav1.N3000Node{
		Spec: fpgav1.N3000NodeSpec{
			FPGA: []fpgav1.N3000Fpga{
				{
					PCIAddr: "0000:1b:00.0",
				},
			},
		},
	}

	var _ = Describe("getFPGAInfo", func() {
		var _ = It("will return valid []N3000FpgaStatus ", func() {
			fpgaInfoExec = fakeFpgaInfo
			result, err := getFPGAInventory(log)

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
		var _ = It("will return error when fpgaInfo failed", func() {
			fakeFpgaInfoErrReturn = fmt.Errorf("error")
			fpgaInfoExec = fakeFpgaInfo
			_, err := getFPGAInventory(log)
			cleanFPGA()
			Expect(err).To(HaveOccurred())
		})
	})
	var _ = Describe("checkFPGADieTemperature", func() {
		var _ = It("will return nil in successfully scenario", func() {
			err := os.Setenv(envTemperatureLimitName, "")
			Expect(err).ToNot(HaveOccurred())

			fpgaInfoExec = fakeFpgaInfo
			err = checkFPGADieTemperature("0000:1b:00.0", log)

			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("will return error when FPGA temperature exceeded limit", func() {
			fpgaInfoExec = fakeFpgaInfo
			err := checkFPGADieTemperature("0000:2b:00.0", log)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(
				Equal("FPGA temperature: 98.500000, exceeded limit: 85.000000, on PCIAddr: 0000:2b:00.0"))
		})
		var _ = It("will return error when PCIAddr does not exist", func() {
			fpgaInfoExec = fakeFpgaInfo
			err := checkFPGADieTemperature("0000:xx:00.0", log)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Not found PCIAddr: 0000:xx:00.0"))
		})
		var _ = It("will return error when fpgaInfo failed", func() {
			fakeFpgaInfoErrReturn = fmt.Errorf("error")
			fpgaInfoExec = fakeFpgaInfo
			err := checkFPGADieTemperature("0000:xx:00.0", log)
			cleanFPGA()
			Expect(err).To(HaveOccurred())
		})
	})
	var _ = Describe("getFPGATemperatureLimit", func() {
		var _ = It("will return temperature limit from env variable", func() {
			fpgaTemperature := 75.0 //in Celsius degrees
			err := os.Setenv(envTemperatureLimitName, fmt.Sprintf("%f", fpgaTemperature))
			Expect(err).ToNot(HaveOccurred())
			t := getFPGATemperatureLimit()
			Expect(t).To(Equal(fpgaTemperature))
		})
		var _ = It("will return default temperature limit, incorrect env variable", func() {
			err := os.Setenv(envTemperatureLimitName, "incorrect_float_value")
			Expect(err).ToNot(HaveOccurred())
			t := getFPGATemperatureLimit()
			Expect(t).To(Equal(fpgaTemperatureDefaultLimit))
		})
		var _ = It("will return default temperature limit, env variable exceeded limit", func() {
			fpgaTemperature := 10.0 //in Celsius degrees
			err := os.Setenv(envTemperatureLimitName, fmt.Sprintf("%f", fpgaTemperature))
			Expect(err).ToNot(HaveOccurred())
			t := getFPGATemperatureLimit()
			Expect(t).To(Equal(fpgaTemperatureDefaultLimit))
		})
	})
	var _ = Describe("programFPGAs", func() {
		var _ = It("will return nil in successfully scenario", func() {
			fpgaInfoExec = fakeFpgaInfo
			fpgasUpdateExec = fakeFpgasUpdate
			rsuExec = fakeRsu
			err := f.ProgramFPGAs(&sampleOneFPGA)
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("will return error when fpgasUpdate failed", func() {
			fpgaInfoExec = fakeFpgaInfo
			fakeFpgasUpdateErrReturn = fmt.Errorf("error")
			fpgasUpdateExec = fakeFpgasUpdate
			rsuExec = fakeRsu
			err := f.ProgramFPGAs(&sampleOneFPGA)
			cleanFPGA()
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will return error when rsuExec failed", func() {
			fpgaInfoExec = fakeFpgaInfo
			fpgasUpdateExec = fakeFpgasUpdate
			fakeRsuUpdateErrReturn = fmt.Errorf("error")
			rsuExec = fakeRsu
			err := f.ProgramFPGAs(&sampleOneFPGA)
			cleanFPGA()
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will return error when one PCIAddr in CR does not exist", func() {
			fpgaInfoExec = fakeFpgaInfo
			fpgasUpdateExec = fakeFpgasUpdate
			rsuExec = fakeRsu
			err := f.ProgramFPGAs(&fpgav1.N3000Node{
				Spec: fpgav1.N3000NodeSpec{
					FPGA: []fpgav1.N3000Fpga{
						{
							PCIAddr: "0000:1x:00.0", //not existing one
						},
					},
				},
			})
			Expect(err).To(HaveOccurred())
		})
	})
	var _ = Describe("verifyPreconditions", func() {
		var testServer *httptest.Server

		BeforeEach(func() {
			fpgaInfoExec = fakeFpgaInfo
			testServer = CreateTestServer("/fpga/image/")
		})
		AfterEach(func() {
			testServer.Close()
		})
		var _ = It("will return nil in successfully scenario", func() {
			err := f.verifyPreconditions(&fpgav1.N3000Node{
				Spec: fpgav1.N3000NodeSpec{
					FPGA: []fpgav1.N3000Fpga{
						{
							PCIAddr:      "0000:1b:00.0",
							UserImageURL: testServer.URL + "/fpga/image/1.bin",
						},
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("will return error when http get failed", func() {
			err := f.verifyPreconditions(&fpgav1.N3000Node{
				Spec: fpgav1.N3000NodeSpec{
					FPGA: []fpgav1.N3000Fpga{
						{
							PCIAddr:      "0000:1b:00.0",
							UserImageURL: "*?1.bin",
						},
					},
				},
			})
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will return error when fpga temperature exceeded limit", func() {
			fpgaTemperature := 70.0 //in Celsius degrees
			err := os.Setenv(envTemperatureLimitName, fmt.Sprintf("%f", fpgaTemperature))
			Expect(err).ToNot(HaveOccurred())
			err = f.verifyPreconditions(&sampleOneFPGA)
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will return error when one PCIAddr in CR does not exist", func() {
			Expect(f.verifyPreconditions(&fpgav1.N3000Node{
				Spec: fpgav1.N3000NodeSpec{
					FPGA: []fpgav1.N3000Fpga{
						{
							PCIAddr: "0000:1b:00.0",
						},
						{
							PCIAddr: "0000:1x:00.0", //not existing one
						},
					},
				},
			})).To(HaveOccurred())
		})
		var _ = It("will return error when fpgaInfo failed", func() {
			fakeFpgaInfoErrReturn = fmt.Errorf("error")
			err := f.verifyPreconditions(&sampleOneFPGA)
			cleanFPGA()
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will succeed with non-existing directory", func() {
			tmpPathHolder := fpgaUserImageSubfolderPath
			fpgaUserImageSubfolderPath = testTmpFolder + "/fakeFPGApath"

			err := f.verifyPreconditions(&fpgav1.N3000Node{
				Spec: fpgav1.N3000NodeSpec{
					FPGA: []fpgav1.N3000Fpga{
						{
							PCIAddr:      "0000:1b:00.0",
							UserImageURL: testServer.URL + "/fpga/image/1.bin",
						},
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())

			os.Remove(fpgaUserImageSubfolderPath)
			fpgaUserImageSubfolderPath = tmpPathHolder
		})
	})
})
