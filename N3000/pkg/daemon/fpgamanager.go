// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	fpgav1 "github.com/otcshare/openshift-operator/N3000/api/v1"
)

var (
	fpgaInfoPath = "fpgainfo"
	bmcRegex     = regexp.MustCompile(`^([a-zA-Z .:]+?)(?:\s*:\s)(.+)$`)
	fpgaInfoExec = func(command string) (string, error) {
		cmd := exec.Command(fpgaInfoPath, command)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("`%s %s` executed unsuccessfully. Output:'%s', Error: %+v",
				fpgaInfoPath, command, string(output), err)
			return "", err
		}
		return string(output), nil
	}
)

func getFPGAInventory() ([]fpgav1.N3000FpgaStatus, error) {
	fpgaInfoBMCOutput, err := fpgaInfoExec("bmc")
	if err != nil {
		log.Printf("getFPGAInventory(): Failed to get output from fpgainfo: %+v", err)
		return nil, err
	}

	var inventory []fpgav1.N3000FpgaStatus
	for _, deviceBMCOutput := range strings.Split(fpgaInfoBMCOutput, "//****** BMC SENSORS ******//") {
		var dev fpgav1.N3000FpgaStatus
		pciFound := false
		for _, line := range strings.Split(deviceBMCOutput, "\n") {
			matches := bmcRegex.FindStringSubmatch(line)
			if matches != nil && len(matches) == 3 {
				switch matches[1] {
				case "PCIe s:b:d.f":
					dev.PciAddr = matches[2]
					pciFound = true
				case "Device Id":
					dev.DeviceID = matches[2]
				case "Bitstream Id":
					dev.BitstreamID = matches[2]
				case "Bitstream Version":
					dev.BitstreamVersion = matches[2]
				case "Numa Node":
					dev.NumaNode, _ = strconv.Atoi(matches[2])
				}
			}
		}
		if pciFound {
			inventory = append(inventory, dev)
		}
	}
	return inventory, nil
}

type FPGAManager struct {
	Log logr.Logger
	d   *Daemon
}

func (fpgaM *FPGAManager) getFPGAStatus() ([]fpgav1.N3000FpgaStatus, error) {
	//log := dc.Log.WithName("getFPGAStatus")

	//TODO fpga get status data
	devs := make([]fpgav1.N3000FpgaStatus, 0)
	//...

	return devs, nil
}
