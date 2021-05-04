// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/go-logr/logr"
	fpgav1 "github.com/open-ness/openshift-operator/N3000/api/v1"
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
	nvmupdateExec = runExecWithLog
	fpgadiagExec  = runExec
	ethtoolExec   = runExec
	tarExec       = runExec

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

	devs, _ := filepath.Glob(fmt.Sprintf("/sys/bus/pci/devices/%s/fpga_region/region*/dfl-fme.0/dfl*/net/*", bmcPCI))
	for _, f := range devs {
		mac, err := ioutil.ReadFile(fmt.Sprintf("%s/address", f))
		if err != nil {
			fmt.Print(err)
		}
		s := fpgav1.FortvilleStatus{
			MAC:     string(mac),
			PciAddr: bmcPCI,
		}
		netDev := filepath.Base(f)
		err = fm.addEthtoolInfo(netDev, &s)
		if err != nil {
			log.Error(err, "Unable to get ethtool info for", "interface", netDev)
		}
		err = fm.addDeviceName(&s)
		if err != nil {
			log.Error(err, "Unable to get lspci info for", "interface", netDev)
		}
		fs = append(fs, s)
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
	log.V(4).Info("Extracting nvmupdate package")
	_, err := tarExec(exec.Command("tar", "xzfv", nvmPackageDestination, "-C", nvmInstallDest), log, false)
	return err
}

func verifyImagePaths() error {
	paths := []string{
		path.Join(nvmupdate64ePath, nvmupdate64e),
		configFile,
	}
	for _, p := range paths {
		fi, err := os.Lstat(p)
		if err != nil {
			return errors.Wrap(err, "Failed to get file info")
		}
		if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
			return errors.New("Symbolic link detected in nvm package " + p)
		}
	}
	return nil
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
	return verifyImagePaths()
}

func (fm *FortvilleManager) flashMac(mac string, dryRun bool) error {
	log := fm.Log.WithName("flashMac")
	step := 0
	for {
		rootAttr := &syscall.SysProcAttr{
			Credential: &syscall.Credential{Uid: 0, Gid: 0},
		}
		// Call nvmupdate64 -i first to refresh devices
		cmd := exec.Command(nvmupdate64e, "-i")
		cmd.SysProcAttr = rootAttr
		cmd.Dir = nvmupdate64ePath
		err := nvmupdateExec(cmd, log, dryRun)
		if err != nil {
			return err
		}

		log.V(2).Info("Updating", "MAC", mac)
		m := strings.Replace(mac, ":", "", -1)
		m = strings.ToUpper(m)
		cmd = exec.Command(nvmupdate64e, "-u", "-m", m, "-c", configFile, "-o", updateOutFile, "-l")
		cmd.SysProcAttr = rootAttr
		cmd.Dir = nvmupdate64ePath
		err = nvmupdateExec(cmd, log, dryRun)
		if err != nil {
			return err
		}

		if dryRun {
			log.V(2).Info("Dry run device update succeeded", "MAC", mac)
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
				log.V(2).Info("Device updated", "MAC", mac, "Modules", moduleVersions)
				if updateStepCount == step {
					log.V(2).Info("Next update available", "MAC", mac)
					log.V(2).Info("Maximum step count reached - ending...", "MAC", mac)
					break
				}
				log.V(2).Info("Next update available - updating", "MAC", mac)
			} else {
				log.V(2).Info("Device updated to latest firmware", "MAC", mac, "Modules", moduleVersions)
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
		}

		if !found {
			return errors.New("MAC not found: " + m.MAC)
		}
	}

	err = createFolder(nvmInstallDest, log)
	if err != nil {
		return err
	}

	log.V(4).Info("Start downloading", "url", n.Spec.Fortville.FirmwareURL)
	err = fm.getNVMUpdate(n)
	if err != nil {
		return err
	}
	log.V(4).Info("Package downloaded and installed", "url", n.Spec.Fortville.FirmwareURL)

	return nil
}

func (fm *FortvilleManager) powerCycle(pcis []string, dryRun bool) error {
	log := fm.Log.WithName("powerCycle")
	for _, p := range pcis {
		log.V(2).Info("Power cycling N3000 device", "pci", p)
		err := rsuExec(exec.Command(rsuPath, "bmcimg", p), log, dryRun)
		if err != nil {
			log.Error(err, "Failed to power cycle N3000 device")
		}
	}

	return nil
}
