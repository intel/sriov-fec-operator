// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2025 Intel Corporation

package daemon

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/hpcloud/tail"
	sriovv2 "github.com/intel/sriov-fec-operator/api/sriovfec/v2"
	vrbv1 "github.com/intel/sriov-fec-operator/api/sriovvrb/v1"
	"github.com/intel/sriov-fec-operator/pkg/common/utils"
	"github.com/sirupsen/logrus"
)

var (
	downloadFile    = DownloadFile
	untarFile       = Untar
	artifactsFolder = "/tmp"

	srsFftWindowsCoefficientFilepath string
	monitoredFiles                   sync.Map
)

const pfConfigAppFilepath = "/sriov_workdir/pf_bb_config"

type fftUpdater struct {
	log        *logrus.Logger
	httpClient *http.Client
}

func NewPfBBConfigController(log *logrus.Logger, sharedVfioToken string) *pfBBConfigController {
	var err error
	cert := getTLSCert(log)

	httpClient := http.DefaultClient
	if cert != nil {
		log.Info("using HTTPS client")
		httpClient, err = NewSecureHTTPSClient(cert)
		if err != nil {
			log.WithError(err).Error("failed to create HTTPS client")
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
	log             *logrus.Logger
	sharedVfioToken string
	fftUpdater      *fftUpdater
}

func getTLSCert(log *logrus.Logger) *x509.Certificate {

	var certFilePath = "/etc/certificate/tls.crt"

	if _, err := os.Stat(certFilePath); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	derBytes, err := os.ReadFile(certFilePath)
	if err != nil {
		log.WithError(err).Error("failed to read certificate file")
		return nil
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		log.WithError(err).Error("failed to parse certificate")
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

		if err := p.configureDevice(acc, pf, bbdevConfigFilepath); err != nil {
			p.log.WithError(err).WithField("pci", pf.PCIAddress).Error("failed to configure device's queues")
			return err
		}
	} else {
		p.log.Info("All sections of 'BBDevConfig' are nil - queues will not be (re)configured")
	}

	return nil
}

func (p *pfBBConfigController) configureDevice(acc sriovv2.SriovAccelerator, pf *sriovv2.PhysicalFunctionConfigExt, bbdevConfigFilepath string) error {
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

	p.log.Infof("pf-bb-config file path is : %s", pfConfigAppFilepath)
	logLinkStatus(pf.PCIAddress, p.log)
	var token *string
	if strings.EqualFold(pf.PFDriver, utils.VfioPci) {
		token = &p.sharedVfioToken
	}

	return p.runPFConfig(deviceName, bbdevConfigFilepath, pf.PCIAddress, token)
}

func (p *pfBBConfigController) VrbinitializePfBBConfig(acc vrbv1.SriovAccelerator, pf *vrbv1.PhysicalFunctionConfigExt) error {
	if pf.BBDevConfig.VRB1 != nil || pf.BBDevConfig.VRB2 != nil {
		bbdevConfigFilepath := filepath.Join(workdir, fmt.Sprintf("%s.ini", pf.PCIAddress))
		if err := generateVrbBBDevConfigFile(pf.BBDevConfig, bbdevConfigFilepath); err != nil {
			p.log.WithError(err).WithField("pci", pf.PCIAddress).Error("failed to create bbdev config file")
			return err
		}

		if err := p.configureVrbDevice(acc, pf, bbdevConfigFilepath); err != nil {
			p.log.WithError(err).WithField("pci", pf.PCIAddress).Error("failed to configure device's queues")
			return err
		}

	} else {
		p.log.Info("All sections of 'BBDevConfig' are nil - queues will not be (re)configured")
	}

	return nil
}

func (p *pfBBConfigController) configureVrbDevice(acc vrbv1.SriovAccelerator, pf *vrbv1.PhysicalFunctionConfigExt, bbdevConfigFilepath string) error {
	deviceName := VrbsupportedAccelerators.Devices[acc.DeviceID]

	switch deviceName {
	case "VRB1":
		err := p.updateFftWindowsCoefficientFilepath(&pf.BBDevConfig.VRB1.FFTLut, "/sriov_workdir/vrb1/srs_fft_windows_coefficient.bin")
		if err != nil {
			return err
		}

	case "VRB2":
		err := p.updateFftWindowsCoefficientFilepath(&pf.BBDevConfig.VRB2.FFTLut, "/sriov_workdir/vrb2/srs_fft_windows_coefficient.bin")
		if err != nil {
			return err
		}

	default:
		// Handle the case where deviceName is neither "VRB1" nor "VRB2"
		p.log.Warnf("Unsupported device name: %s", deviceName)
		return fmt.Errorf("unsupported device name: %s", deviceName)
	}

	logLinkStatus(pf.PCIAddress, p.log)
	var token *string
	if strings.EqualFold(pf.PFDriver, utils.VfioPci) {
		token = &p.sharedVfioToken
	}

	return p.runPFConfig(deviceName, bbdevConfigFilepath, pf.PCIAddress, token)
}

func (p *pfBBConfigController) updateFftWindowsCoefficientFilepath(fftLutConfig *vrbv1.FFTLutParam, defaultFilePath string) error {
	var err error
	srsFftWindowsCoefficientFilepath, err = p.fftUpdater.VrbgetFftFilePath(p, fftLutConfig)
	if err != nil {
		p.log.WithError(err)
		return err
	}
	if srsFftWindowsCoefficientFilepath == "" {
		srsFftWindowsCoefficientFilepath = defaultFilePath
	}
	p.log.Infof("SRS FFT file path is : %s", srsFftWindowsCoefficientFilepath)
	return nil
}

// runPFConfig executes a pf-bb-config tool
// deviceName is one of: FPGA_LTE (N3000) FPGA_5GNR (N3000), ACC100, ACC200(deprecated), VRB1, VRB2
// cfgFilepath is a filepath to the config
// pciAddress points to a specific PF device
func (p *pfBBConfigController) runPFConfig(deviceName, cfgFilepath, pciAddress string, token *string) error {
	switch deviceName {
	case "FPGA_LTE", "FPGA_5GNR", "ACC100", "VRB1", "VRB2":
	case "ACC200":
		deviceName = "VRB1"
	default:
		return fmt.Errorf("incorrect deviceName for pf config: %s", deviceName)
	}
	args := []string{pfConfigAppFilepath, deviceName, "-c", cfgFilepath, "-p", pciAddress}

	if token != nil {
		args = append(args, "-v", *token)
	}

	if strings.Contains(deviceName, "VRB") {
		args = append(args, "-f", srsFftWindowsCoefficientFilepath)
	}
	_, err := runExecCmd(args, p.log)
	if err != nil {
		p.log.WithError(err).Error("failed to run pf_bb_config")
		return err
	}

	// Monitor the log file only when vfio-pci is used
	if token != nil {
		if err := monitorLogFile(pciAddress, p.log); err != nil {
			return err
		}
	}

	return nil
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

	// TODO: Remove workaround
	// Code below implements workaround problem related with pf_bb_config app. Ticket describing an issue SCSY-190446
	sockFileToBeDeleted := fmt.Sprintf("/tmp/pf_bb_config.%s.sock", pciAddress)
	p.log.WithField("applying-work-around", "SCSY-190446").Info("deleting", sockFileToBeDeleted)

	if err := os.Remove(sockFileToBeDeleted); err != nil {
		p.log.WithError(err).Infof("cannot remove (%s)) file: %s", sockFileToBeDeleted, err)
		return nil
	}

	return err
}

func (f *fftUpdater) getFftFilePath(p *pfBBConfigController, pf *sriovv2.FFTLutParam) (string, error) {
	// When both url and checksum are empty
	if pf.FftUrl == "" && pf.FftChecksum == "" {
		p.log.Info("Using default SRS FFT file for configuration")
		return "", nil
	}
	// When both url and checksum are not empty
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
	// When both url and checksum are empty
	if pf.FftUrl == "" && pf.FftChecksum == "" {
		p.log.Info("Using default SRS FFT file for configuration")
		return "", nil
	}
	// When both url and checksum are not empty
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

func logLinkStatus(pciAddr string, log *logrus.Logger) {
	// Execute the lspci command
	cmd := exec.Command("lspci", "-vvs", pciAddr)
	output, err := cmd.Output()
	if err != nil {
		log.WithError(err).WithField("pciAddr", pciAddr).Warning("Error running lspci")
		return
	}

	// Convert output to string
	outputStr := strings.ToLower(string(output))

	// Regular expression pattern for LnkSta case insensitive
	re := regexp.MustCompile(`(?i)LnkSta:.+?(\n|$)`)
	match := re.FindString(outputStr)

	// Trim leading and trailing whitespace
	match = strings.TrimSpace(match)
	match = strings.ReplaceAll(match, "\t", " ")

	if match != "" {
		// Report warning only if link is downgraded
		if strings.Contains(match, "downgraded") {
			log.WithField("pciAddr", pciAddr).Warning(match)
		} else {
			log.WithField("pciAddr", pciAddr).Debug(match)
		}
	}
}

func monitorLogFile(pciAddr string, log *logrus.Logger) error {
	pfBbConfigLog := fmt.Sprintf("/var/log/pf_bb_cfg_%s.log", pciAddr)
	// Check if the pciAddr is already being monitored
	if _, loaded := monitoredFiles.LoadOrStore(pciAddr, struct{}{}); loaded {
		// If loaded is true, it means the pciAddr is already being monitored
		log.WithField("pciAddr", pciAddr).Infof("%s file is already being monitored", pfBbConfigLog)
		return nil
	}
	t, err := tail.TailFile(pfBbConfigLog, tail.Config{Follow: true, ReOpen: true, Logger: log, Poll: true})
	if err != nil {
		log.WithError(err).WithField("pciAddr", pciAddr).Errorf("Failed to tail log file: %v", err)
		monitoredFiles.Delete(pciAddr)
		return err
	}

	// Start watching the file
	go func() {
		defer t.Cleanup()
		defer func() {
			err := t.Stop()
			if err != nil {
				log.WithError(err).Errorf("tail Stop returned error")
			}
		}()
		defer monitoredFiles.Delete(pciAddr)
		for line := range t.Lines {
			log.WithField("pciAddr", pciAddr).Infof("%s", line.Text)
		}
	}()

	return nil
}
