// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"encoding/csv"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	fpgav1 "github.com/otcshare/openshift-operator/N3000/api/v1"
	"github.com/pkg/errors"
)

const (
	fpgadiagPath = "fpgadiag"
	ethtoolPath  = "ethtool"
	nvmupdate64e = "./nvmupdate64e"
	// Currently going from pre-4.42 to post-4.42 is the only 2 step upgrade process
	updateStepCount = 2
)

var (
	nvmupdateExec = runExec
	fpgadiagExec  = runExec
	ethtoolExec   = runExec
	tarExec       = runExec

	pciRegex     = regexp.MustCompile(`^([a-f0-9]{4}):([a-f0-9]{2}):([a-f0-9]{2})\.([012357])$`)
	mactestRegex = regexp.MustCompile(`^(?:\s*)([a-z0-9]+)(?:\s*)([a-f0-9]{2}:[a-f0-9]{2}:[a-f0-9]{2}:[a-f0-9]{2}:[a-f0-9]{2}:[a-f0-9]{2})$`)
	ethtoolRegex = regexp.MustCompile(`^([a-z-]+?)(?:\s*:\s)(.+)$`)

	nvmInstallDest        = "/n3000-workdir/nvmupdate/"
	updateOutFile         = nvmInstallDest + "update.xml"
	nvmPackageDestination = nvmInstallDest + "nvmupdate.tar.gz"
	nvmupdate64ePath      = nvmInstallDest + "700Series/Linux_x64/"
	configFile            = nvmInstallDest + "700Series/Linux_x64/nvmupdate.cfg"
)

type FortvilleManager struct {
	Log           logr.Logger
	nvmupdatePath string
}

func (fm *FortvilleManager) getN3000Devices() ([]string, error) {
	log := fm.Log.WithName("getN3000Device")
	fpgaInfoBMCOutput, err := fpgaInfoExec(exec.Command(fpgaInfoPath, "bmc"), log, false)
	if err != nil {
		return nil, err
	}

	var devs []string
	for _, deviceBMCOutput := range strings.Split(fpgaInfoBMCOutput, "//****** BMC SENSORS ******//") {
		for _, line := range strings.Split(deviceBMCOutput, "\n") {
			matches := bmcRegex.FindStringSubmatch(line)
			if len(matches) == 3 && matches[1] == "PCIe s:b:d.f" {
				devs = append(devs, matches[2])
				break
			}
		}
	}
	return devs, nil
}

func (fm *FortvilleManager) getN3000NICs(bmcPCI string) ([]fpgav1.FortvilleStatus, error) {
	log := fm.Log.WithName("getN3000NICs")

	var fs []fpgav1.FortvilleStatus

	matches := pciRegex.FindStringSubmatch(bmcPCI)
	if len(matches) == 5 {
		out, err := fpgadiagExec(exec.Command(fpgadiagPath, "-m", "mactest", "-S", matches[1], "-B",
			matches[2], "-D", matches[3], "-F", matches[4]), log, false)
		if err == nil {
			for _, line := range strings.Split(out, "\n") {
				m := mactestRegex.FindStringSubmatch(line)
				if len(m) == 3 {
					s := fpgav1.FortvilleStatus{
						MAC: m[2],
					}
					err := fm.addEthtoolInfo(m[1], &s)
					if err != nil {
						log.Error(err, "Unable to get ethtool info for", "interface", m[1])
					}
					err = fm.addDeviceName(&s)
					if err != nil {
						log.Error(err, "Unable to get lspci info for", "interface", m[1])
					}
					fs = append(fs, s)
				}
			}
		} else {
			log.Error(err, "Unable to get fpgadiag -m mactest info for", "PCI", bmcPCI)
			return fs, err
		}
	} else {
		return fs, errors.New("Invalid BMC PCI address: " + bmcPCI)
	}
	return fs, nil
}

func (fm *FortvilleManager) addEthtoolInfo(ifName string, fs *fpgav1.FortvilleStatus) error {
	log := fm.Log.WithName("addEthtoolInfo")
	out, err := ethtoolExec(exec.Command(ethtoolPath, "-i", ifName), log, false)
	if err == nil {
		for _, line := range strings.Split(out, "\n") {
			m := ethtoolRegex.FindStringSubmatch(line)
			if len(m) == 3 {
				switch m[1] {
				case "bus-info":
					fs.PciAddr = m[2]
				case "firmware-version":
					fs.Version = m[2]
				}
			}
		}
	} else {
		return err
	}
	return nil
}

func (fm *FortvilleManager) addDeviceName(fs *fpgav1.FortvilleStatus) error {
	log := fm.Log.WithName("addDeviceName")

	lspciFortfille := `lspci -Dm | grep -i ` + fs.PciAddr
	out, err := exec.Command("bash", "-c", lspciFortfille).CombinedOutput()

	if err != nil {
		log.Error(err, "Error when executing: "+lspciFortfille)
		return err
	}

	csvReader := csv.NewReader(strings.NewReader(string(out)))
	csvReader.Comma = ' '
	csvReader.FieldsPerRecord = -1

	records, err := csvReader.ReadAll()
	if err != nil {
		return errors.New("Failed to parse CSV because: " + err.Error() + ". Input: " + string(out))
	}
	if len(records) == 0 {
		return errors.New("No entries in CSV output from lspci")
	}

	if len(records[0]) >= 4 {
		fs.Name = records[0][3]
	}
	return nil
}

func (fm *FortvilleManager) getInventory() ([]fpgav1.N3000FortvilleStatus, error) {
	log := fm.Log.WithName("getNetworkDevices")

	devs, err := fm.getN3000Devices()
	if err != nil {
		log.Error(err, "Unable to retrieve N3000 devices")
		return nil, err
	}

	var nfs []fpgav1.N3000FortvilleStatus
	for _, d := range devs {
		nf := fpgav1.N3000FortvilleStatus{
			N3000PCI: d,
		}
		fs, err := fm.getN3000NICs(d)
		if err != nil {
			log.Error(err, "Unable to retrieve Fortville devices for N3000 card", "BMC PCI", d)
			return nfs, err
		} else {
			nf.NICs = fs
		}
		nfs = append(nfs, nf)
	}

	return nfs, nil
}

func (fm *FortvilleManager) installNvmupdate() error {
	log := fm.Log.WithName("installNvmupdate")
	log.Info("Extracting nvmupdate package")
	_, err := tarExec(exec.Command("tar", "xzfv", nvmPackageDestination, "-C", nvmInstallDest), log, false)
	return err
}

func (fm *FortvilleManager) getNVMUpdate(n *fpgav1.N3000Node) error {
	log := fm.Log.WithName("getNVMUpdate")
	if n.Spec.Fortville.FirmwareURL != "" {
		err := getImage(nvmPackageDestination,
			n.Spec.Fortville.FirmwareURL,
			n.Spec.Fortville.CheckSum,
			log)
		if err != nil {
			log.Error(err, "Unable to get NVMUpdate package")
			return errors.Wrap(err, "NVMUpdate package error:")
		}
		err = fm.installNvmupdate()
		if err != nil {
			log.Error(err, "Unable to install nvmupdate")
			return errors.Wrap(err, "NVMUpdate package error:")
		}
		fm.nvmupdatePath = nvmupdate64ePath
	} else {
		return errors.New("Empty Fortville.FirmwareURL")
	}
	return nil
}

func (fm *FortvilleManager) flashMac(mac string, dryRun bool) error {
	log := fm.Log.WithName("flashMac")
	step := 0
	for {

		// Call nvmupdate64 -i first to refresh devices
		cmd := exec.Command(nvmupdate64e, "-i")
		cmd.Dir = nvmupdate64ePath
		_, err := nvmupdateExec(cmd, log, dryRun)
		if err != nil {
			return err
		}

		log.Info("Updating", "MAC", mac)
		m := strings.Replace(mac, ":", "", -1)
		m = strings.ToUpper(m)
		cmd = exec.Command(nvmupdate64e, "-u", "-m", m, "-c", configFile, "-o", updateOutFile, "-l")
		cmd.Dir = nvmupdate64ePath
		_, err = nvmupdateExec(cmd, log, dryRun)
		if err != nil {
			return err
		}

		if dryRun {
			log.Info("Dry run device update succeeded", "MAC", mac)
			break
		} else {
			us, err := getDeviceUpdateFromFile(updateOutFile)
			if err != nil {
				return err
			}

			var em moduleStatus
			var errStatus error

			moduleVersions := ""
			for _, m := range us.Modules {
				if m.Status != em {
					if m.Status.Result != "Success" {
						errStatus = fmt.Errorf("Invalid update result: %s for MAC: %s module %s version %s",
							m.Status.Result, mac, m.Type, m.Version)
						log.Error(err, "flashMac error")
					} else {
						moduleVersions = moduleVersions + " Module: " + m.Type + " version: " + m.Version
					}
				}
			}

			if errStatus != nil {
				return errStatus
			}

			step++
			if us.NextUpdateAvailable == 1 {
				log.Info("Device updated", "MAC", mac, "Modules", moduleVersions)
				if updateStepCount == step {
					log.Info("Next update available", "MAC", mac)
					log.Info("Maximum step count reached - ending...", "MAC", mac)
					break
				}
				log.Info("Next update available - updating", "MAC", mac)
			} else {
				log.Info("Device updated to latest firmware", "MAC", mac, "Modules", moduleVersions)
				break
			}
		}
	}

	return nil
}

func appendBMC(bmcs []string, bmcPCI string) []string {
	found := false
	for _, b := range bmcs {
		if b == bmcPCI {
			found = true
		}
	}
	if !found {
		return append(bmcs, bmcPCI)
	}
	return bmcs
}

func (fm *FortvilleManager) flash(n *fpgav1.N3000Node) error {
	log := fm.Log.WithName("flashMac")

	inv, err := fm.getInventory()
	if err != nil {
		log.Error(err, "Unable to get inventory")
		return err
	}

	var bmcs []string
	for _, m := range n.Spec.Fortville.MACs {
		for _, i := range inv {
			for _, nic := range i.NICs {
				if m.MAC == nic.MAC {
					bmcs = appendBMC(bmcs, i.N3000PCI)
					err := fm.flashMac(m.MAC, n.Spec.DryRun)
					if err != nil {
						log.Error(err, "Failed to update")
						return err
					}
					break
				}
			}
		}
	}

	if len(bmcs) != 0 {
		err = fm.powerCycle(bmcs, n.Spec.DryRun)
	}

	return err
}

func (fm *FortvilleManager) verifyPreconditions(n *fpgav1.N3000Node) error {
	log := fm.Log.WithName("verifyPreconditions")
	if n.Spec.Fortville.FirmwareURL == "" {
		return fmt.Errorf("Empty Fortville.FirmwareURL")
	}

	inv, err := fm.getInventory()
	if err != nil {
		log.Error(err, "Unable to get inventory")
		return err
	}

	for _, m := range n.Spec.Fortville.MACs {
		found := false
		for _, i := range inv {
			for _, nic := range i.NICs {
				if m.MAC == nic.MAC {
					found = true
					break
				}
			}
			if !found {
				return errors.New("MAC not found: " + m.MAC)
			}
		}
	}

	err = createFolder(nvmInstallDest, log)
	if err != nil {
		return err
	}

	log.Info("Start downloading", "url", n.Spec.Fortville.FirmwareURL)
	err = fm.getNVMUpdate(n)
	if err != nil {
		return err
	}
	log.Info("Package downloaded and installed", "url", n.Spec.Fortville.FirmwareURL)

	return nil
}

func (fm *FortvilleManager) powerCycle(pcis []string, dryRun bool) error {
	log := fm.Log.WithName("powerCycle")
	for _, p := range pcis {
		log.Info("Power cycling N3000 device", "pci", p)
		_, err := rsuExec(exec.Command(rsuPath, "bmcimg", p), log, dryRun)
		if err != nil {
			log.Error(err, "Failed to power cycle N3000 device")
		}
	}

	return nil
}
