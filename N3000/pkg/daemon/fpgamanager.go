// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	fpgav1 "github.com/otcshare/openshift-operator/N3000/api/v1"
	"github.com/pkg/errors"
)

var (
	fpgaInfoPath                = "fpgainfo"
	fpgaInfoExec                = runExec
	bmcRegex                    = regexp.MustCompile(`^([a-zA-Z .:]+?)(?:\s*:\s)(.+)$`)
	bmcParametersRegex          = regexp.MustCompile(`^([()0-9]+?) (.+)(?:\s*:\s)(.+) (.+)$`)
	fpgaUserImageSubfolderPath  = "/root/test"
	fpgaUserImageFile           = fpgaUserImageSubfolderPath + "/fpga.bin"
	fpgasUpdatePath             = "fpgasupdate"
	fpgasUpdateExec             = runExec
	rsuPath                     = "rsu"
	rsuExec                     = runExec
	restartTimeLimitInSeconds   = 20
	fpgaTemperatureDefaultLimit = 85.0 //in Celsius degrees
	fpgaTemperatureBottomRange  = 60.0 //in Celsius degrees
	fpgaTemperatureTopRange     = 95.0 //in Celsius degrees
	envTemperatureLimitName     = "FPGA_DIE_TEMP_LIMIT"
)

func getFPGATemperatureLimit() float64 {
	val := os.Getenv(envTemperatureLimitName)
	if val == "" {
		return fpgaTemperatureDefaultLimit
	}
	temperature, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return fpgaTemperatureDefaultLimit
	}
	if temperature < fpgaTemperatureBottomRange || temperature > fpgaTemperatureTopRange {
		return fpgaTemperatureDefaultLimit
	}
	return temperature
}

func getFPGAInventory(log logr.Logger) ([]fpgav1.N3000FpgaStatus, error) {
	fpgaInfoBMCOutput, err := fpgaInfoExec(fpgaInfoPath, []string{"bmc"}, log)
	if err != nil {
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

func checkFPGADieTemperature(PCIAddr string, log logr.Logger) error {
	fpgaInfoBMCOutput, err := fpgaInfoExec(fpgaInfoPath, []string{"bmc"}, log)
	if err != nil {
		return err
	}
	pciFound := false
	fpgaDieTemperature := 0.0
	for _, deviceBMCOutput := range strings.Split(fpgaInfoBMCOutput, "//****** BMC SENSORS ******//") {
		for _, line := range strings.Split(deviceBMCOutput, "\n") {
			matches := bmcRegex.FindStringSubmatch(line)
			if matches != nil && len(matches) == 3 {
				switch matches[1] {
				case "PCIe s:b:d.f":
					if PCIAddr == matches[2] {
						pciFound = true
					}
				}
			}
			matches = bmcParametersRegex.FindStringSubmatch(line)
			if matches != nil && len(matches) == 5 {
				switch matches[1] {
				case "(12)":
					fpgaDieTemperature, _ = strconv.ParseFloat(matches[3], 64)
				}
			}
		}
		if pciFound {
			break
		}
	}
	if pciFound {
		limit := getFPGATemperatureLimit()
		if fpgaDieTemperature > limit {
			return fmt.Errorf("FPGA temperature: %f, exceeded limit: %f, on PCIAddr: %s",
				fpgaDieTemperature, limit, PCIAddr)
		}
		return nil
	}
	return fmt.Errorf("Not found PCIAddr: %s", PCIAddr)
}

type FPGAManager struct {
	Log logr.Logger
}

func (fpga *FPGAManager) dryrunFPGAprogramming() {
	log := fpga.Log.WithName("dryrunFPGAprogramming")
	log.Info("FPGA programming in dryrun mode")
}

func (fpga *FPGAManager) FPGAprogramming(PCIAddr string) error {
	log := fpga.Log.WithName("FPGAprogramming")

	//--------REMOVE THIS BLOCK OF CODE WHEN LOGIC WILL BE FULLY TESTED----
	if true {
		err := errors.New(">>> FPGAprogramming Blocker <<<")
		log.Error(err, "Failed to programming FPGA")
		return err
	}
	//---------------------------------------------------------------------

	log.Info("Start programming FPGA")
	_, err := fpgasUpdateExec(fpgasUpdatePath,
		[]string{fpgaUserImageFile, PCIAddr}, fpga.Log)
	if err != nil {
		log.Error(err, "Failed to programming FPGA on PCIAddr", PCIAddr)
		return err
	}
	log.Info("Programming FPGA completed, start new power cycle N3000 ...")
	_, err = rsuExec(rsuPath, []string{"bmcimg", PCIAddr}, fpga.Log)
	if err != nil {
		log.Error(err, "Failed to execute rsu on PCIAddr", PCIAddr)
		return err
	}
	//TODO: wait for start next power cycle?
	time.Sleep(time.Duration(restartTimeLimitInSeconds) * time.Second)
	return nil
}

func (fpga *FPGAManager) verifyPCIAddrs(fpgaCR []fpgav1.N3000Fpga) error {
	log := fpga.Log.WithName("verifyPCIAddrs")
	currentInventory, err := getFPGAInventory(fpga.Log)
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
		err := checkFPGADieTemperature(obj.PCIAddr, fpga.Log)
		if err != nil {
			return err
		}
		log.Info("Start downloading", "url", obj.UserImageURL)
		err = getImage(fpgaUserImageFile,
			obj.UserImageURL,
			obj.CheckSum,
			log)
		if err != nil {
			log.Error(err, "Unable to get FPGA Image")
			return errors.Wrap(err, "FPGA image error:")
		}
		log.Info("Image downloaded")
		if n.DryRun == true {
			fpga.dryrunFPGAprogramming()
		} else {
			err = fpga.FPGAprogramming(obj.PCIAddr)
			if err != nil {
				log.Error(err, "Unable to programming FPGA")
				return err
			}
		}
	}
	return nil
}
