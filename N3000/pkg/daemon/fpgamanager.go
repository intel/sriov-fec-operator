// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	fpgav1 "github.com/otcshare/openshift-operator/N3000/api/v1"
	"github.com/pkg/errors"
)

var (
	fpgaInfoPath                = "fpgainfo"
	fpgaInfoExec                = runExec
	bmcRegex                    = regexp.MustCompile(`^([a-zA-Z .:]+?)(?:\s*:\s)(.+)$`)
	bmcParametersRegex          = regexp.MustCompile(`^([()0-9]+?) (.+)(?:\s*:\s)(.+) (.+)$`)
	fpgaUserImageSubfolderPath  = "/n3000-workdir"
	fpgaUserImageFile           = filepath.Join(fpgaUserImageSubfolderPath, "fpga")
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
	fpgaInfoBMCOutput, err := fpgaInfoExec(exec.Command(fpgaInfoPath, "bmc"), log, false)
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
	fpgaInfoBMCOutput, err := fpgaInfoExec(exec.Command(fpgaInfoPath, "bmc"), log, false)
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

func (fpga *FPGAManager) ProgramFPGA(file string, PCIAddr string, dryRun bool) error {
	log := fpga.Log.WithName("ProgramFPGA").WithValues("pci", PCIAddr)

	log.Info("Starting")
	_, err := fpgasUpdateExec(exec.Command(fpgasUpdatePath, file, PCIAddr), fpga.Log, dryRun)
	if err != nil {
		log.Error(err, "Failed to program FPGA")
		return err
	}
	log.Info("Program FPGA completed, start new power cycle N3000 ...")
	_, err = rsuExec(exec.Command(rsuPath, "bmcimg", PCIAddr), fpga.Log, dryRun)
	if err != nil {
		log.Error(err, "Failed to execute rsu")
		return err
	}
	return nil
}

func (fpga *FPGAManager) verifyPCIAddrs(fpgaCR []fpgav1.N3000Fpga) error {
	log := fpga.Log.WithName("verifyPCIAddrs")
	currentInventory, err := getFPGAInventory(fpga.Log)
	if err != nil {
		return fmt.Errorf("Unable to get FPGA inventory before program err: " + err.Error())
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

func (fpga *FPGAManager) verifyPreconditions(n *fpgav1.N3000Node) error {
	log := fpga.Log.WithName("verifyPreconditions")
	err := fpga.verifyPCIAddrs(n.Spec.FPGA)
	if err != nil {
		return err
	}
	err = createFolder(fpgaUserImageSubfolderPath, log)
	if err != nil {
		return err
	}
	for i, obj := range n.Spec.FPGA {
		err := checkFPGADieTemperature(obj.PCIAddr, fpga.Log)
		if err != nil {
			return err
		}
		indexStr := strconv.Itoa(i)
		log.Info("Start downloading", "url", obj.UserImageURL)
		err = getImage(fpgaUserImageFile+indexStr+".bin",
			obj.UserImageURL,
			obj.CheckSum,
			log)
		if err != nil {
			log.Error(err, "Unable to get FPGA Image")
			return errors.Wrap(err, "FPGA image error:")
		}
		log.Info("Image downloaded", "url", obj.UserImageURL)
	}
	return nil
}

func (fpga *FPGAManager) ProgramFPGAs(n *fpgav1.N3000Node) error {
	log := fpga.Log.WithName("ProgramFPGAs")
	for i, obj := range n.Spec.FPGA {
		err := checkFPGADieTemperature(obj.PCIAddr, fpga.Log)
		if err != nil {
			return err
		}
		indexStr := strconv.Itoa(i)
		log.Info("Start program", "PCIAddr", obj.PCIAddr)
		err = fpga.ProgramFPGA(fpgaUserImageFile+indexStr+".bin", obj.PCIAddr, n.Spec.DryRun)
		if err != nil {
			log.Error(err, "Failed to program FPGA:", "pci", obj.PCIAddr)
			return err
		}
	}
	return nil
}
