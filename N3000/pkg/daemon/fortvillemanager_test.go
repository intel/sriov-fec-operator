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
	fpgav1 "github.com/otcshare/openshift-operator/N3000/api/v1"
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
	nvmupdateOutputFile = "test/nvmupdate.xml"
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

func fakeNvmupdate(cmd *exec.Cmd, log logr.Logger, dryRun bool) (string, error) {
	if strings.Contains(cmd.String(), "nvmupdate64e -i") {
		return "", fakeNvmupdateFirstErrReturn
	} else if strings.Contains(cmd.String(), "nvmupdate64e -u -m") {
		return "", fakeNvmupdateSecondErrReturn
	}
	return "", fmt.Errorf("Unsupported command: %s", cmd)
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

func usersFortvilleMock(w http.ResponseWriter, r *http.Request) {
}

var _ = Describe("Fortville Manager", func() {
	f := FortvilleManager{Log: ctrl.Log.WithName("daemon-test")}
	sampleOneFortville := fpgav1.N3000Node{
		Spec: fpgav1.N3000NodeSpec{
			Fortville: fpgav1.N3000Fortville{
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
			Fortville: fpgav1.N3000Fortville{
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
			Fortville: fpgav1.N3000Fortville{
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
			Fortville: fpgav1.N3000Fortville{
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
			Fortville: fpgav1.N3000Fortville{
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

			// Second flash with the same data
			err = f.flash(&sampleOneFortville)
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
			rsuExec = runExec

			err := f.flash(&sampleOneFortville)
			Expect(err).ToNot(HaveOccurred())
		})

		var _ = It("will call runExec with DryRun flag", func() {
			cleanFortville()
			nvmupdateExec = fakeNvmupdate
			fpgaInfoExec = fakeFpgaInfo
			fpgadiagExec = fakeFpgadiag
			rsuExec = runExec

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
