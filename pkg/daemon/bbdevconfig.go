// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2022 Intel Corporation

package daemon

import (
	"fmt"
	sriovv2 "github.com/intel-collab/applications.orchestration.operators.sriov-fec-operator/api/v2"
	"github.com/intel-collab/applications.orchestration.operators.sriov-fec-operator/pkg/common/utils"
	"github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	pfConfigAppFilepath = "/sriov_workdir/pf_bb_config"
)

func NewPfBBConfigController(log *logrus.Logger, sharedVfioToken string) *pfBBConfigController {
	return &pfBBConfigController{
		log:             log,
		sharedVfioToken: sharedVfioToken,
	}
}

type pfBBConfigController struct {
	log             *logrus.Logger
	sharedVfioToken string
}

func (p *pfBBConfigController) initializePfBBConfig(acc sriovv2.SriovAccelerator, pf *sriovv2.PhysicalFunctionConfigExt) error {
	if pf.BBDevConfig.N3000 != nil || pf.BBDevConfig.ACC100 != nil || pf.BBDevConfig.ACC200 != nil {
		bbdevConfigFilepath := filepath.Join(workdir, fmt.Sprintf("%s.ini", pf.PCIAddress))
		if err := generateBBDevConfigFile(pf.BBDevConfig, bbdevConfigFilepath); err != nil {
			p.log.WithError(err).WithField("pci", pf.PCIAddress).Error("failed to create bbdev config file")
			return err
		}
		defer func() {
			if err := os.Remove(bbdevConfigFilepath); err != nil {
				p.log.WithError(err).WithField("path", bbdevConfigFilepath).Error("failed to remove old bbdev config file")
			}
		}()

		deviceName := supportedAccelerators.Devices[acc.DeviceID]

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
	case "FPGA_LTE", "FPGA_5GNR", "ACC100", "ACC200":
	default:
		return fmt.Errorf("incorrect deviceName for pf config: %s", deviceName)
	}
	if token == nil {
		_, err := runExecCmd([]string{pfConfigAppFilepath, deviceName, "-c", cfgFilepath, "-p", pciAddress}, p.log)
		return err
	} else {
		_, err := runExecCmd([]string{pfConfigAppFilepath, deviceName, "-c", cfgFilepath, "-v", *token, "-p", pciAddress}, p.log)
		return err
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
