// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2023 Intel Corporation

package daemon

import (
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	sriovv2 "github.com/intel/sriov-fec-operator/api/sriovfec/v2"
	vrbv1 "github.com/intel/sriov-fec-operator/api/sriovvrb/v1"
	"github.com/intel/sriov-fec-operator/pkg/common/utils"
	"github.com/sirupsen/logrus"
)

var (
	downloadFile    = DownloadFile
	untarFile       = Untar
	artifactsFolder = "/tmp"

	pfConfigAppFilepath              string
	srsFftWindowsCoefficientFilepath string
)

type fftUpdater struct {
	log        *logrus.Logger
	httpClient *http.Client
}

func NewPfBBConfigController(log *logrus.Logger, sharedVfioToken string) *pfBBConfigController {
	var err error
	cert := getTlsCert(log)

	httpClient := http.DefaultClient
	if cert != nil {
		log.Info("found certificate - using HTTPS client")
		httpClient, err = NewSecureHttpsClient(cert)
		if err != nil {
			log.WithError(err)
			return nil
		}
	}

	return &pfBBConfigController{
		log:             log,
		sharedVfioToken: sharedVfioToken,
		fftUpdater: &fftUpdater{
			log:        log,
			httpClient: httpClient,
		},
	}
}

type pfBBConfigController struct {
	log               *logrus.Logger
	sharedVfioToken   string
	fftUpdater        *fftUpdater
}

func getTlsCert(log *logrus.Logger) *x509.Certificate {
	derBytes, err := os.ReadFile("/etc/certificate/tls.crt")
	if err != nil {
		log.Error(err, "failed to read mounted certificate")
		return nil
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		log.Error(err, "failed to parse certificate")
		return nil
	}
	return cert
}

func (p *pfBBConfigController) initializePfBBConfig(acc sriovv2.SriovAccelerator, pf *sriovv2.PhysicalFunctionConfigExt) error {
	if pf.BBDevConfig.N3000 != nil || pf.BBDevConfig.ACC100 != nil || pf.BBDevConfig.ACC200 != nil {
		bbdevConfigFilepath := filepath.Join(workdir, fmt.Sprintf("%s.ini", pf.PCIAddress))
		if err := generateBBDevConfigFile(pf.BBDevConfig, bbdevConfigFilepath); err != nil {
			p.log.WithError(err).WithField("pci", pf.PCIAddress).Error("failed to create bbdev config file")
			return err
		}

		deviceName := supportedAccelerators.Devices[acc.DeviceID]
		var err error
		if deviceName == "ACC200" {
			srsFftWindowsCoefficientFilepath, err = p.fftUpdater.getFftFilePath(p, &pf.BBDevConfig.ACC200.FFTLut)
			if err != nil {
				p.log.WithError(err)
				return err
			}
			if srsFftWindowsCoefficientFilepath == "" {
				srsFftWindowsCoefficientFilepath = "/sriov_workdir/vrb1/srs_fft_windows_coefficient.bin"
			}
			p.log.Infof("SRS FFT file path is : %s", srsFftWindowsCoefficientFilepath)
		}
		if pfConfigAppFilepath == "" {
			pfConfigAppFilepath = "/sriov_workdir/pf_bb_config"
		}
		p.log.Infof("pf-bb-config file path is : %s", pfConfigAppFilepath)
		var token *string
		if strings.EqualFold(pf.PFDriver, utils.VFIO_PCI) {
			token = &p.sharedVfioToken
		}

		if err := p.runPFConfig(deviceName, bbdevConfigFilepath, pf.PCIAddress, token); err != nil {
			p.log.WithError(err).WithField("pci", pf.PCIAddress).Error("failed to configure device's queues")
			return err
		}
	} else {
		p.log.Info("All sections of 'BBDevConfig' are nil - queues will not be (re)configured")
	}

	return nil
}

func (p *pfBBConfigController) VrbinitializePfBBConfig(acc vrbv1.SriovAccelerator, pf *vrbv1.PhysicalFunctionConfigExt) error {
	if pf.BBDevConfig.VRB1 != nil || pf.BBDevConfig.VRB2 != nil {
		bbdevConfigFilepath := filepath.Join(workdir, fmt.Sprintf("%s.ini", pf.PCIAddress))
		if err := generateVrbBBDevConfigFile(pf.BBDevConfig, bbdevConfigFilepath); err != nil {
			p.log.WithError(err).WithField("pci", pf.PCIAddress).Error("failed to create bbdev config file")
			return err
		}

		deviceName := VrbsupportedAccelerators.Devices[acc.DeviceID]
		var err error
		if deviceName == "VRB1" {
			srsFftWindowsCoefficientFilepath, err = p.fftUpdater.VrbgetFftFilePath(p, &pf.BBDevConfig.VRB1.FFTLut)
			if err != nil {
				p.log.WithError(err)
				return err
			}
			if srsFftWindowsCoefficientFilepath == "" {
				srsFftWindowsCoefficientFilepath = "/sriov_workdir/vrb1/srs_fft_windows_coefficient.bin"
			}
			p.log.Infof("SRS FFT file path is : %s", srsFftWindowsCoefficientFilepath)
		}
		if deviceName == "VRB2" {
			srsFftWindowsCoefficientFilepath, err = p.fftUpdater.VrbgetFftFilePath(p, &pf.BBDevConfig.VRB2.FFTLut)
			if err != nil {
				p.log.WithError(err)
				return err
			}
			if srsFftWindowsCoefficientFilepath == "" {
				srsFftWindowsCoefficientFilepath = "/sriov_workdir/vrb2/srs_fft_windows_coefficient.bin"
			}
			p.log.Infof("SRS FFT file path is : %s", srsFftWindowsCoefficientFilepath)
		}

		if pfConfigAppFilepath == "" {
			pfConfigAppFilepath = "/sriov_workdir/pf_bb_config"
		}

		var token *string
		if strings.EqualFold(pf.PFDriver, utils.VFIO_PCI) {
			token = &p.sharedVfioToken
		}

		if err := p.runPFConfig(deviceName, bbdevConfigFilepath, pf.PCIAddress, token); err != nil {
			p.log.WithError(err).WithField("pci", pf.PCIAddress).Error("failed to configure device's queues")
			return err
		}
	} else {
		p.log.Info("All sections of 'BBDevConfig' are nil - queues will not be (re)configured")
	}

	return nil
}

// runPFConfig executes a pf-bb-config tool
// deviceName is one of: FPGA_LTE or FPGA_5GNR or ACC100
// cfgFilepath is a filepath to the config
// pciAddress points to a specific PF device
func (p *pfBBConfigController) runPFConfig(deviceName, cfgFilepath, pciAddress string, token *string) error {
	switch deviceName {
	case "FPGA_LTE", "FPGA_5GNR", "ACC100", "ACC200", "VRB1", "VRB2":
	default:
		return fmt.Errorf("incorrect deviceName for pf config: %s", deviceName)
	}
	if token == nil {
		if deviceName == "ACC200" || deviceName == "VRB1" {
			_, err := runExecCmd([]string{pfConfigAppFilepath, "VRB1", "-c", cfgFilepath, "-p", pciAddress, "-f", srsFftWindowsCoefficientFilepath}, p.log)
			return err
		} else if deviceName == "VRB2" {
			_, err := runExecCmd([]string{pfConfigAppFilepath, deviceName, "-c", cfgFilepath, "-p", pciAddress, "-f", srsFftWindowsCoefficientFilepath}, p.log)
			return err
		} else {
			_, err := runExecCmd([]string{pfConfigAppFilepath, deviceName, "-c", cfgFilepath, "-p", pciAddress}, p.log)
			return err
		}
	} else {
		if deviceName == "ACC200" || deviceName == "VRB1" {
			_, err := runExecCmd([]string{pfConfigAppFilepath, "VRB1", "-c", cfgFilepath, "-v", *token, "-p", pciAddress, "-f", srsFftWindowsCoefficientFilepath}, p.log)
			return err
		} else if deviceName == "VRB2" {
			_, err := runExecCmd([]string{pfConfigAppFilepath, deviceName, "-c", cfgFilepath, "-v", *token, "-p", pciAddress, "-f", srsFftWindowsCoefficientFilepath}, p.log)
			return err
		} else {
			_, err := runExecCmd([]string{pfConfigAppFilepath, deviceName, "-c", cfgFilepath, "-v", *token, "-p", pciAddress}, p.log)
			return err
		}
	}
}

func (p *pfBBConfigController) stopPfBBConfig(pciAddress string) error {
	_, err := execAndSuppress([]string{
		"pkill",
		"-9",
		"-f",
		fmt.Sprintf("pf_bb_config.*%s", pciAddress),
	}, p.log, func(e error) bool {
		if ee, ok := e.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			p.log.Info("ignoring errorCode(1) returned by pkill")
			return true
		}
		return false
	})

	//TODO: Remove workaround
	//Code below implements workaround problem related with pf_bb_config app. Ticket describing an issue SCSY-190446
	sockFileToBeDeleted := fmt.Sprintf("/tmp/pf_bb_config.%s.sock", pciAddress)
	p.log.WithField("applying-work-around", "SCSY-190446").Info("deleting", sockFileToBeDeleted)

	if err := os.Remove(sockFileToBeDeleted); err != nil {
		p.log.WithError(err).Infof("cannot remove (%s)) file: %s", sockFileToBeDeleted, err)
		return nil
	}

	return err
}

func (f *fftUpdater) getFftFilePath(p *pfBBConfigController, pf *sriovv2.FFTLutParam) (string, error) {
	// when both url and checksum are empty
	if pf.FftUrl == "" && pf.FftChecksum == "" {
		p.log.Info("Using default SRS FFT file for configuration")
		return "", nil
	}
	// when both url and checksum are not empty
	if pf.FftUrl != "" && pf.FftChecksum != "" {
		newFftFp, err := p.fftUpdater.updateFftFile(pf)
		if err != nil {
			p.log.WithError(err).Error("failed to update the FFT file")
			return "", err
		}
		return newFftFp, nil
	}
	return "", fmt.Errorf("missing one of the FFT parameters")
}

func (f *fftUpdater) updateFftFile(fft *sriovv2.FFTLutParam) (string, error) {
	var newFftFile string
	fftUrl := fft.FftUrl
	fftChecksum := fft.FftChecksum

	targetPath := artifactsFolder
	f.log.Info(" Target Path: ", targetPath)

	fftTarFile := filepath.Join(targetPath, filepath.Base(fftUrl))
	f.log.Info("Downloading FFT tar file from url", fftUrl)

	err := downloadFile(fftTarFile, fftUrl, fftChecksum, f.httpClient)
	if err != nil {
		return "", err
	}

	f.log.Info("FFT file downloaded successfully - now extracting...")

	newFftFile, err = untarFile(fftTarFile, targetPath, log)
	if err != nil {
		log.Error("Error in extracting the file")
		return "", err
	}
	return newFftFile, nil
}

func (f *fftUpdater) VrbgetFftFilePath(p *pfBBConfigController, pf *vrbv1.FFTLutParam) (string, error) {
	// when both url and checksum are empty
	if pf.FftUrl == "" && pf.FftChecksum == "" {
		p.log.Info("Using default SRS FFT file for configuration")
		return "", nil
	}
	// when both url and checksum are not empty
	if pf.FftUrl != "" && pf.FftChecksum != "" {
		newFftFp, err := p.fftUpdater.VrbupdateFftFile(pf)
		if err != nil {
			p.log.WithError(err).Error("failed to update the FFT file")
			return "", err
		}
		return newFftFp, nil
	}
	return "", fmt.Errorf("missing one of the FFT parameters")
}

func (f *fftUpdater) VrbupdateFftFile(fft *vrbv1.FFTLutParam) (string, error) {
	var newFftFile string
	fftUrl := fft.FftUrl
	fftChecksum := fft.FftChecksum

	targetPath := artifactsFolder
	f.log.Info(" Target Path: ", targetPath)

	fftTarFile := filepath.Join(targetPath, filepath.Base(fftUrl))
	f.log.Info("Downloading FFT tar file from url", fftUrl)

	err := downloadFile(fftTarFile, fftUrl, fftChecksum, f.httpClient)
	if err != nil {
		return "", err
	}

	f.log.Info("FFT file downloaded successfully - now extracting...")

	newFftFile, err = untarFile(fftTarFile, targetPath, log)
	if err != nil {
		log.Error("Error in extracting the file")
		return "", err
	}
	return newFftFile, nil
}
