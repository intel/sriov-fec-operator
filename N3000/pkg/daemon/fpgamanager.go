// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	fpgav1 "github.com/otcshare/openshift-operator/N3000/api/v1"
	"github.com/pkg/errors"
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
	fpgaUserImageSubfolderPath = "/root/test"
	fpgaUserImageFile          = fpgaUserImageSubfolderPath + "/fpga.bin"
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
}

func (fpga *FPGAManager) dryrunFPGAprogramming() {
	log := fpga.Log.WithName("dryrunFPGAprogramming")
	log.Info("FPGA programming in dryrun mode")
}

func (fpga *FPGAManager) FPGAprogramming() error {
	log := fpga.Log.WithName("FPGAprogramming")
	log.Info("Start programming FPGA")
	//TODO: call cmd fpgasupdate <bin file> <PCIe>
	return nil
}

func (fpga *FPGAManager) verifyPCIAddrs(fpgaCR []fpgav1.N3000Fpga) error {
	log := fpga.Log.WithName("verifyPCIAddrs")
	currentInventory, err := getFPGAInventory()
	if err != nil {
		return fmt.Errorf("Unable to get FPGA inventory before programming err: " + err.Error())
	}
	for idx := range fpgaCR {
		pciFound := false
		for i := range currentInventory {
			if fpgaCR[idx].PCIAddr == currentInventory[i].PciAddr {
				log.Info("PCIAddr detected", "PciAddr", fpgaCR[idx].PCIAddr)
				pciFound = true
				break
			}
		}
		if !pciFound {
			return fmt.Errorf("Unable to detect FPGA PCIAddr=%s: ", fpgaCR[idx].PCIAddr)
		}
	}
	return nil
}

func (fpga *FPGAManager) processFPGA(n *fpgav1.N3000Node) error {
	log := fpga.Log.WithName("processFPGA")

	err := fpga.verifyPCIAddrs(n.Spec.FPGA)
	if err != nil {
		return err
	}
	err = createFolder(fpgaUserImageSubfolderPath, log)
	if err != nil {
		return err
	}
	for _, obj := range n.Spec.FPGA {
		log.Info("Start processFPGA", "url", obj.UserImageURL)
		err := getImage(fpgaUserImageFile,
			obj.UserImageURL,
			obj.CheckSum,
			log)
		if err != nil {
			log.Error(err, "Unable to get FPGA Image")
			return errors.Wrap(err, "FPGA image error:")
		}
		if n.DryRun == true {
			fpga.dryrunFPGAprogramming()
		} else {
			err = fpga.FPGAprogramming()
			if err != nil {
				log.Error(err, "Unable to programming FPGA")
				return err
			}
		}
	}
	return nil
}
