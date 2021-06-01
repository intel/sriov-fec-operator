// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	fpgav1 "github.com/rmr-silicom/openshift-operator/N3000/api/v1"
)

var (
	fpgaInfoPath                = "fpgainfo"
	fpgaInfoExec                = runExec
	bmcRegex                    = regexp.MustCompile(`^([a-zA-Z .:]+?)(?:\s*:\s)(.+)$`)
	bmcParametersRegex          = regexp.MustCompile(`^([()0-9]+?) (.+)(?:\s*:\s)(.+) (.+)$`)
	fpgaUserImageSubfolderPath  = "/n3000-workdir"
	fpgaUserImageFile           = filepath.Join(fpgaUserImageSubfolderPath, "fpga")
	fpgasUpdatePath             = "fpgasupdate"
	fpgasUpdateExec             = runExecWithLog
	rsuPath                     = "rsu"
	rsuExec                     = runExecWithLog
	fpgaTemperatureDefaultLimit = 85.0 //in Celsius degrees
	fpgaTemperatureBottomRange  = 40.0 //in Celsius degrees
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
			if len(matches) == 3 {
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
			if len(matches) == 3 {
				switch matches[1] {
				case "PCIe s:b:d.f":
					if PCIAddr == matches[2] {
						pciFound = true
					}
				}
			}
			matches = bmcParametersRegex.FindStringSubmatch(line)
			if len(matches) == 5 {
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

func (fpga *FPGAManager) UpdateFW(file string, PCIAddr string, idx int, dryRun bool) error {
	log := fpga.Log.WithName("UpdateFW").WithValues("pci", PCIAddr)

	log.V(4).Info("Starting")
	_ = ioutil.WriteFile("/sys/module/firmware_class/parameters/path", []byte(fpgaUserImageSubfolderPath), 0666)
	_ = ioutil.WriteFile("/sys/class/fpga_sec_mgr/fpga_sec0/update/filename", []byte(fmt.Sprintf("fpga%d.bin", idx)), 0666)

	for {
		content, _ := ioutil.ReadFile("/sys/class/fpga_sec_mgr/fpga_sec0/update/status")
		if string(content) != "idle" {
			log.V(4).Info("Done idle")
			break
		}
		time.Sleep(1 * time.Second)
	}

	for {
		content, _ := ioutil.ReadFile("/sys/class/fpga_sec_mgr/fpga_sec0/update/status")
		if string(content) != "preparing" {
			log.V(4).Info("Done preparing")
			break
		}
		time.Sleep(2 * time.Second)
	}

	for {
		content, _ := ioutil.ReadFile("/sys/class/fpga_sec_mgr/fpga_sec0/update/status")
		if string(content) != "writing" {
			log.V(4).Info("Done writing")
			break
		}
		time.Sleep(5 * time.Second)
	}

	log.V(4).Info("Program FPGA completed, start new power cycle N3000 ...")
	err := rsuExec(exec.Command(rsuPath, "bmcimg", PCIAddr), fpga.Log, dryRun)
	if err != nil {
		log.Error(err, "Failed to execute rsu")
		return err
	}
	return nil
}

func (fpga *FPGAManager) ProgramFPGA(file string, PCIAddr string, idx int, dryRun bool) error {
	log := fpga.Log.WithName("ProgramFPGA").WithValues("pci", PCIAddr)

	err := fpgasUpdateExec(exec.Command(fpgasUpdatePath, file), fpga.Log, dryRun)
	if err != nil {
		log.Error(err, "Failed to execute fpgaupdate")
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
				log.V(4).Info("PCIAddr detected", "PciAddr", fpgaCR[idx].PCIAddr)
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
		log.V(4).Info("Start downloading", "url", obj.UserImageURL)
		err = getImage(fpgaUserImageFile+indexStr+".bin",
			obj.UserImageURL,
			obj.CheckSum,
			log)
		if err != nil {
			log.Error(err, "Unable to get FPGA Image")
			return errors.Wrap(err, "FPGA image error:")
		}
		log.V(4).Info("Image downloaded", "url", obj.UserImageURL)
	}
	return nil
}

func (fpga *FPGAManager) ProgramFPGAs(n *fpgav1.N3000Node) error {
	log := fpga.Log.WithName("ProgramFPGAs")
	data := make([]byte, 128)

	for i, obj := range n.Spec.FPGA {
		err := checkFPGADieTemperature(obj.PCIAddr, fpga.Log)
		if err != nil {
			return err
		}

		log.V(4).Info("Start program", "PCIAddr", obj.PCIAddr)
		fName := fpgaUserImageFile + strconv.Itoa(i) + ".bin"
		file, err := os.Open(fName)
		if err != nil {
			log.V(4).Error(err, "Can't open file")
			return nil
		}
		defer file.Close()

		count, err := file.Read(data)
		if count != 128 || err != nil {
			log.V(4).Error(err, "Can't read data")
			return nil
		}
		if data[3] == 0xb6 && data[2] == 0xea && data[1] == 0xfd && data[0] == 0x19 {
			err = fpga.UpdateFW(fName, obj.PCIAddr, i, n.Spec.DryRun)
			if err != nil {
				log.Error(err, "Failed to program FPGA:", "pci", obj.PCIAddr)
				return err
			}
		} else {
			err = fpga.ProgramFPGA(fName, obj.PCIAddr, i, n.Spec.DryRun)
			if err != nil {
				log.Error(err, "Failed to program FPGA:", "pci", obj.PCIAddr)
				return err
			}
		}
	}
	return nil
}
