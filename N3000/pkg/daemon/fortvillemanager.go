// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	fpgav1 "github.com/otcshare/openshift-operator/N3000/api/v1"
	"github.com/pkg/errors"
)

const (
	supportedFortville = `"X710\|XXV710\|XL710"`
	// nvmInstallDest     = "/nvmupdate/"
	nvmInstallDest        = "/root/test/"
	inventoryOutFile      = nvmInstallDest + "inventory.xml"
	nvmPackageDestination = nvmInstallDest + "/nvmupdate.tar.gz"
	nvmupdate64epath      = nvmInstallDest + "700Series/Linux_x64/nvmupdate64e"
)

type FortvilleManager struct {
	Log           logr.Logger
	nvmupdatePath string
}

func (fm *FortvilleManager) getNetworkDevices() ([]fpgav1.N3000FortvilleStatus, error) {
	log := fm.Log.WithName("getNetworkDevices")
	lspciFortfille := `lspci -m | grep -iw ` + supportedFortville
	cmd := exec.Command("bash", "-c", lspciFortfille)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Error(err, "Error when executing: "+lspciFortfille, "out", out.String(), "stderr", stderr.String())
		return nil, err
	}

	csvReader := csv.NewReader(strings.NewReader(out.String()))
	csvReader.Comma = ' '
	csvReader.FieldsPerRecord = -1

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, errors.New("Failed to parse CSV because: " + err.Error() + ". Input: " + out.String())
	}
	if len(records) == 0 {
		return nil, errors.New("No entries in CSV output from lspci")
	}

	devs := make([]fpgav1.N3000FortvilleStatus, 0)
	for _, rec := range records {
		if len(rec) >= 4 {
			pci, devName := rec[0], rec[3]
			devs = append(devs, fpgav1.N3000FortvilleStatus{
				Name:    devName,
				PciAddr: pci,
			})
		}
	}

	return devs, nil
}

func (fm *FortvilleManager) installdNvmupdate() error {
	log := fm.Log.WithName("installdNvmupdate")
	log.Info("Extracting nvmupdate package")
	cmd := exec.Command("tar", "xzfv", nvmPackageDestination, "-C", nvmInstallDest)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// func (fm *FortvilleManager) dryrunNvmupdate() {
// 	log := fm.Log.WithName("dryrunNvmupdate")
// 	log.Info("install Nvmupdate in dryrun mode")
// }

func (fm *FortvilleManager) flash(n *fpgav1.N3000Node) error {
	log := fm.Log.WithName("flash")
	if n.Spec.Fortville.FirmwareURL != "" {
		log.Info("Start flashing fortville")
		//call nvmupdate
	}
	return nil
}

func (fm *FortvilleManager) getInventory(n *fpgav1.N3000Node) (DeviceInventory, error) {
	log := fm.Log.WithName("getInventory")

	if n.Spec.Fortville.FirmwareURL != "" {
		err := getImage(nvmPackageDestination,
			n.Spec.Fortville.FirmwareURL,
			n.Spec.Fortville.CheckSum,
			log)
		if err != nil {
			log.Error(err, "Unable to get Fortville Image")
			return DeviceInventory{}, errors.Wrap(err, "Fortville image error:")
		}
		err = fm.installdNvmupdate()
		if err != nil {
			log.Error(err, "Unable to install nvmupdate")
			return DeviceInventory{}, errors.Wrap(err, "Fortville image error:")
		}
		fm.nvmupdatePath = nvmupdate64epath
	}

	var i DeviceInventory
	if fm.nvmupdatePath == "" {
		return i, nil
	}

	out, err := exec.Command("bash", "-c", fm.nvmupdatePath, " -i -o ", inventoryOutFile).Output()
	if err != nil {
		log.Error(err, "Error when executing", "cmd", "bash", "-c", fm.nvmupdatePath, " -i -o ")
		log.Info("Info when executing %s", string(out))
		return DeviceInventory{}, err
	}

	invf, err := os.Open(inventoryOutFile)
	if err != nil {
		log.Error(err, "Error when opening inventory xml")
		return DeviceInventory{}, err
	}
	defer invf.Close()

	b, _ := ioutil.ReadAll(invf)

	err = xml.Unmarshal(b, &i)
	if err != nil {
		return DeviceInventory{}, err
	}

	return i, nil
}

func (fm *FortvilleManager) processInventory(inv *DeviceInventory, ns *fpgav1.N3000NodeStatus) {
	log := fm.Log.WithName("processInventory")
	log.Info("Processing inventory from nvmupdate")
	for idx := range ns.Fortville {
		for _, i := range inv.InventoryList {
			bus, err := strconv.Atoi(i.Bus)
			if err != nil {
				log.Error(err, "Invalid PCI Addr value...skipping", "bus:", i.Bus, "Instance:", i)
				continue
			}
			dev, err := strconv.Atoi(i.Dev)
			if err != nil {
				log.Error(err, "Invalid PCI Addr value...skipping", "dev:", i.Dev, "Instance:", i)
				continue
			}
			f, err := strconv.Atoi(i.Func)
			if err != nil {
				log.Error(err, "Invalid PCI Addr value...skipping", "func:", i.Func, "Instance:", i)
				continue
			}

			invPciAddr := fmt.Sprintf("%02x", bus) + ":" + fmt.Sprintf("%02x", dev) + "." + fmt.Sprintf("%x", f)
			if ns.Fortville[idx].PciAddr == invPciAddr {
				for _, m := range i.Modules {
					ns.Fortville[idx].Modules = append(ns.Fortville[idx].Modules, fpgav1.N3000FortvilleStatusModules{Type: m.Type,
						Version: m.Version})
				}
				ns.Fortville[idx].MAC = i.MACAddr.Mac.Address
				ns.Fortville[idx].SAN = i.MACAddr.San.Address
			}
		}
	}
}
