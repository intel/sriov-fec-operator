// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	fpgav1 "github.com/open-ness/openshift-operator/N3000/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	fpgdiagOutput = `Found 1 ethernet interfaces:
 ens785f0   64:4c:36:11:1b:a8
***********************----
Read 3 mac addresses from sysfs:
ff:ff:ff:ff:ff:ff
ff:ff:ff:00:00:00
ff:ff:ff:08:00:01
`
	nvmupdateOutputFile              = "test/nvmupdate.xml"
	nvmupdateOutputFile_bad          = "test/nvmupdate-bad.xml"
	nvmupdateOutputFile_nonextupdate = "test/nvmupdate-nonextupdate.xml"

	invalidBmcOutput = `Board Management Controller, MAX10 NIOS FW version D.2.0.12
Board Management Controller, MAX10 Build version D.2.0.5
//****** BMC SENSORS ******//
Object Id                     : 0xEF00000
PCIe s:b:d.f                  : 0000:1:00.0
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
`

	bmcOutputDoublePCI = `Board Management Controller, MAX10 NIOS FW version D.2.0.12
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
PCIe s:b:d.f                  : 0000:1b:00.0
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

	ethtoolOutput = `ethtoolOutput := driver: virtio_net
version: 1.0.0
firmware-version: 1
expansion-rom-version:
bus-info: 0000:00:03.0
supports-statistics: no
supports-test: no
supports-eeprom-access: no
supports-register-dump: no
supports-priv-flags: no
`
)

var (
	fakeNvmupdateFirstErrReturn  error = nil
	fakeNvmupdateSecondErrReturn error = nil
	fakeFpgadiagErrReturn        error = nil
	fakeEthtoolErrReturn         error = nil
	fakeTarErrReturn             error = nil
)

func cleanFortville() {
	fakeNvmupdateFirstErrReturn = nil
	fakeNvmupdateSecondErrReturn = nil
	fakeFpgadiagErrReturn = nil
	fakeEthtoolErrReturn = nil
	fakeTarErrReturn = nil
}

func copyFile(from, to string) (err error) {
	_, err = os.Stat(from)
	if err != nil {
		return
	}

	f, err := os.Open(from)
	if err != nil {
		return
	}
	defer f.Close()
	t, err := os.Create(to)
	if err != nil {
		return
	}
	defer func() {
		cerr := t.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(t, f); err != nil {
		return err
	}
	err = t.Sync()
	return err
}

func mockFortvilleEnv() {
	nvmInstallDest = testTmpFolder
	updateOutFile = nvmInstallDest + "/update.xml"
	nvmPackageDestination = nvmInstallDest + "/nvmupdate.tar.gz"
	nvmupdate64ePath = nvmInstallDest
	configFile = nvmInstallDest + "/nvmupdate.cfg"
	err := copyFile(nvmupdateOutputFile, updateOutFile)
	Expect(err).ToNot(HaveOccurred())
}

func fakeNvmupdate(cmd *exec.Cmd, log logr.Logger, dryRun bool) error {
	if strings.Contains(cmd.String(), "nvmupdate64e -i") {
		return fakeNvmupdateFirstErrReturn
	} else if strings.Contains(cmd.String(), "nvmupdate64e -u -m") {
		return fakeNvmupdateSecondErrReturn
	}
	return fmt.Errorf("Unsupported command: %s", cmd)
}

func fakeFpgadiag(cmd *exec.Cmd, log logr.Logger, dryRun bool) (string, error) {
	if strings.Contains(cmd.String(), "fpgadiag") {
		return fpgdiagOutput, fakeFpgadiagErrReturn
	}
	return "", fmt.Errorf("Unsupported command: %s", cmd)
}

func fakeEthtool(cmd *exec.Cmd, log logr.Logger, dryRun bool) (string, error) {
	if strings.Contains(cmd.String(), "ethtool") {
		return "", fakeEthtoolErrReturn
	}
	return "", fmt.Errorf("Unsupported command: %s", cmd)
}

func fakeTar(cmd *exec.Cmd, log logr.Logger, dryRun bool) (string, error) {
	if strings.Contains(cmd.String(), "tar xzfv") {
		return "", fakeTarErrReturn
	}
	return "", fmt.Errorf("Unsupported command: %s", cmd)
}

func serverFortvilleMock() *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/fortville", usersFortvilleMock)

	srv := httptest.NewServer(handler)

	return srv
}

func fakeFpgaInfoEmptyBCM(cmd *exec.Cmd, log logr.Logger, dryRun bool) (string, error) {
	return "", nil
}

func fakeFpgaInfoInvalidBCM(cmd *exec.Cmd, log logr.Logger, dryRun bool) (string, error) {
	return invalidBmcOutput, nil
}

func fakeFpgaInfoDoubleBMC(cmd *exec.Cmd, log logr.Logger, dryRun bool) (string, error) {
	return bmcOutputDoublePCI, nil
}

func fakeEthtoolInvalidMac(cmd *exec.Cmd, log logr.Logger, dryRun bool) (string, error) {
	return ethtoolOutput, nil
}

func usersFortvilleMock(w http.ResponseWriter, r *http.Request) {
}

var _ = Describe("Fortville Manager", func() {
	f := FortvilleManager{Log: ctrl.Log.WithName("daemon-test")}
	sampleOneFortville := fpgav1.N3000Node{
		Spec: fpgav1.N3000NodeSpec{
			Fortville: &fpgav1.N3000Fortville{
				MACs: []fpgav1.FortvilleMAC{
					{
						MAC: "64:4c:36:11:1b:a8",
					},
				},
				FirmwareURL: "http://www.test.com/fortville/nvmPackage.tag.gz",
			},
		},
	}
	sampleWrongMACFortville := fpgav1.N3000Node{
		Spec: fpgav1.N3000NodeSpec{
			Fortville: &fpgav1.N3000Fortville{
				MACs: []fpgav1.FortvilleMAC{
					{
						MAC: "ff:ff:ff:ff:ff:aa",
					},
				},
				FirmwareURL: "http://www.test.com/fpga/image/1.bin",
			},
		},
	}
	sampleOneFortvilleDryRun := fpgav1.N3000Node{
		Spec: fpgav1.N3000NodeSpec{
			Fortville: &fpgav1.N3000Fortville{
				MACs: []fpgav1.FortvilleMAC{
					{
						MAC: "64:4c:36:11:1b:a8",
					},
				},
				FirmwareURL: "http://www.test.com/fortville/nvmPackage.tag.gz",
			},
			DryRun: true,
		},
	}
	sampleOneFortvilleNoURL := fpgav1.N3000Node{
		Spec: fpgav1.N3000NodeSpec{
			Fortville: &fpgav1.N3000Fortville{
				MACs: []fpgav1.FortvilleMAC{
					{
						MAC: "64:4c:36:11:1b:a8",
					},
				},
			},
		},
	}
	sampleOneFortvilleInvalidChecksum := fpgav1.N3000Node{
		Spec: fpgav1.N3000NodeSpec{
			Fortville: &fpgav1.N3000Fortville{
				MACs: []fpgav1.FortvilleMAC{
					{
						MAC: "64:4c:36:11:1b:a8",
					},
				},
				FirmwareURL: "http://www.test.com/fortville/nvmPackage.tag.gz",
				CheckSum:    "0xbad",
			},
		},
	}

	var _ = Describe("flash", func() {
		var _ = It("will return nil in successfully scenario ", func() {
			cleanFortville()
			ethtoolExec = fakeEthtool
			nvmupdateExec = fakeNvmupdate
			fpgaInfoExec = fakeFpgaInfo
			fpgadiagExec = fakeFpgadiag

			err := f.flash(&sampleOneFortville)
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("will fail because of invalid MAC", func() {
			cleanFortville()
			ethtoolExec = fakeEthtool
			nvmupdateExec = fakeNvmupdate
			fpgaInfoExec = fakeFpgaInfo
			fpgadiagExec = fakeFpgadiag
			fakeNvmupdateSecondErrReturn = fmt.Errorf("error")

			err := f.flash(&sampleOneFortville)
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will fail because of invalid outfile", func() {
			cleanFortville()
			ethtoolExec = fakeEthtool
			nvmupdateExec = fakeNvmupdate
			fpgaInfoExec = fakeFpgaInfo
			fpgadiagExec = fakeFpgadiag

			tmpUpdateOutFile := updateOutFile
			updateOutFile = testTmpFolder + "/invalidOutFile"

			err := f.flash(&sampleOneFortville)
			updateOutFile = tmpUpdateOutFile
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will fail because of wrong status field", func() {
			cleanFortville()
			ethtoolExec = fakeEthtool
			nvmupdateExec = fakeNvmupdate
			fpgaInfoExec = fakeFpgaInfo
			fpgadiagExec = fakeFpgadiag

			tmpUpdateOutFile := updateOutFile
			updateOutFile = nvmupdateOutputFile_bad

			err := f.flash(&sampleOneFortville)
			updateOutFile = tmpUpdateOutFile
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will pass with noNextUpdate", func() {
			cleanFortville()
			ethtoolExec = fakeEthtool
			nvmupdateExec = fakeNvmupdate
			fpgaInfoExec = fakeFpgaInfo
			fpgadiagExec = fakeFpgadiag

			tmpUpdateOutFile := updateOutFile
			updateOutFile = nvmupdateOutputFile_nonextupdate

			err := f.flash(&sampleOneFortville)
			updateOutFile = tmpUpdateOutFile
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("will return nil in successfully scenario (PCI address doubled in BMC)", func() {
			cleanFortville()
			ethtoolExec = fakeEthtool
			nvmupdateExec = fakeNvmupdate
			fpgadiagExec = fakeFpgadiag
			fpgaInfoExec = fakeFpgaInfoDoubleBMC

			err := f.flash(&sampleOneFortville)
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("will return error when nvmupdate failed", func() {
			cleanFortville()
			fakeNvmupdateFirstErrReturn = fmt.Errorf("error")
			nvmupdateExec = fakeNvmupdate
			fpgaInfoExec = fakeFpgaInfo
			fpgadiagExec = fakeFpgadiag
			err := f.flash(&sampleOneFortville)
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will return error when fpgadiag failed", func() {
			cleanFortville()
			fakeFpgadiagErrReturn = fmt.Errorf("error")
			nvmupdateExec = fakeNvmupdate
			fpgaInfoExec = fakeFpgaInfo
			fpgadiagExec = fakeFpgadiag
			err := f.flash(&sampleOneFortville)
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will call runExc", func() {
			cleanFortville()
			nvmupdateExec = fakeNvmupdate
			fpgaInfoExec = fakeFpgaInfo
			fpgadiagExec = fakeFpgadiag
			rsuExec = runExecWithLog

			err := f.flash(&sampleOneFortville)
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("will call runExec with DryRun flag", func() {
			cleanFortville()
			nvmupdateExec = fakeNvmupdate
			fpgaInfoExec = fakeFpgaInfo
			fpgadiagExec = fakeFpgadiag
			rsuExec = runExecWithLog

			err := f.flash(&sampleOneFortvilleDryRun)
			Expect(err).ToNot(HaveOccurred())
		})
	})
	var _ = Describe("verifyImagePaths", func() {
		var _ = It("will return error when a path does not exist", func() {
			orig := nvmupdate64ePath
			nvmupdate64ePath = "nonexistent"
			err := verifyImagePaths()
			Expect(err).To(HaveOccurred())
			nvmupdate64ePath = orig
		})
		var _ = It("will return error when a path is a symlink", func() {
			p := path.Join(nvmupdate64ePath, nvmupdate64e)

			err := os.Remove(p)
			Expect(err).ShouldNot(HaveOccurred())
			err = os.Symlink("/dev/null", p)
			Expect(err).ShouldNot(HaveOccurred())

			err = verifyImagePaths()
			Expect(err).To(HaveOccurred())

			err = os.Remove(p)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = os.Create(path.Join(testTmpFolder, "nvmupdate64e"))
			Expect(err).ShouldNot(HaveOccurred())
		})
		var _ = It("will return nil if paths are valid", func() {
			err := verifyImagePaths()
			Expect(err).ToNot(HaveOccurred())
		})
	})
	var _ = Describe("verifyPreconditions", func() {
		var _ = It("will return error when MAC in CR does not exist ", func() {
			cleanFortville()
			fpgaInfoExec = fakeFpgaInfo
			fpgadiagExec = fakeFpgadiag
			err := f.verifyPreconditions(&sampleWrongMACFortville)
			Expect(err).To(HaveOccurred())

		})
		var _ = It("will return error when no MAC found", func() {
			cleanFortville()
			fpgaInfoExec = fakeFpgaInfoEmptyBCM
			fpgadiagExec = fakeFpgadiag
			err := f.verifyPreconditions(&sampleWrongMACFortville)
			Expect(err).To(HaveOccurred())

		})
		var _ = It("will return error when PCI address is invalid", func() {
			cleanFortville()
			fpgaInfoExec = fakeFpgaInfoInvalidBCM
			fpgadiagExec = fakeFpgadiag
			err := f.verifyPreconditions(&sampleWrongMACFortville)
			Expect(err).To(HaveOccurred())

		})
		var _ = It("will return error when PCI address is invalid (valid MAC)", func() {
			cleanFortville()
			fpgaInfoExec = fakeFpgaInfoInvalidBCM
			fpgadiagExec = fakeFpgadiag
			ethtoolExec = fakeEthtoolInvalidMac
			err := f.verifyPreconditions(&sampleWrongMACFortville)
			Expect(err).To(HaveOccurred())

		})
		var _ = It("will return error when extract nvm package failed ", func() {
			cleanFortville()
			fpgaInfoExec = fakeFpgaInfo
			fpgadiagExec = fakeFpgadiag
			fakeTarErrReturn = fmt.Errorf("error")
			tarExec = fakeTar
			srv := serverFortvilleMock()
			defer srv.Close()
			err := f.verifyPreconditions(&sampleOneFortville)
			fakeTarErrReturn = nil
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will return nil in successfully scenario ", func() {
			cleanFortville()
			fpgaInfoExec = fakeFpgaInfo
			fpgadiagExec = fakeFpgadiag
			tarExec = fakeTar
			srv := serverFortvilleMock()
			defer srv.Close()
			err := f.verifyPreconditions(&sampleOneFortville)
			Expect(err).ToNot(HaveOccurred())
		})
		var _ = It("will fail because of no FirmwareURL ", func() {
			cleanFortville()
			fpgaInfoExec = fakeFpgaInfo
			fpgadiagExec = fakeFpgadiag
			tarExec = fakeTar
			srv := serverFortvilleMock()
			defer srv.Close()
			err := f.verifyPreconditions(&sampleOneFortvilleNoURL)
			Expect(err).To(HaveOccurred())

			err = f.getNVMUpdate(&sampleOneFortvilleNoURL)
			Expect(err).To(HaveOccurred())

			fakeFpgaInfoErrReturn = fmt.Errorf("error")
			_, err = f.getN3000Devices()
			Expect(err).To(HaveOccurred())
			fakeFpgaInfoErrReturn = nil
		})
		var _ = It("will fail because of wrong checksum ", func() {
			cleanFortville()
			ethtoolExec = fakeEthtool
			nvmupdateExec = fakeNvmupdate
			fpgaInfoExec = fakeFpgaInfo
			fpgadiagExec = fakeFpgadiag
			err := f.verifyPreconditions(&sampleOneFortvilleInvalidChecksum)
			Expect(err).To(HaveOccurred())
		})
	})
})
