// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	fpgav1 "github.com/otcshare/openshift-operator/N3000/api/v1"
)

var (
	supportedFortville = `"X710\|XXV710\|XL710"`
	// nvmInstallDest     = "/nvmupdate/"
	nvmInstallDest   = "/root/test/"
	inventoryOutFile = nvmInstallDest + "inventory.xml"
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

func (fm *FortvilleManager) installNvmupdate(firmwareURL string) (string, error) {
	log := fm.Log.WithName("installNvmupdate")

	_, err := os.Stat(nvmInstallDest + "700Series/Linux_x64/nvmupdate64e")
	if os.IsNotExist(err) {
		if firmwareURL == "" {
			return "", errors.New("Unable to install nvmupdate - empty .Spec.Fortville.FirmwareURL")
		}

		log.Info("nvmupdate tool not found - downloading", "url", firmwareURL)
		f, err := os.Create(nvmInstallDest + "/nvmupdate.tar.gz")
		if err != nil {
			return "", err
		}
		defer f.Close()

		r, err := http.Get(firmwareURL)
		if err != nil {
			return "", err
		}

		if r.StatusCode != http.StatusOK {
			return "", fmt.Errorf("Unable to download nvmupdate package from: %s err: %s", firmwareURL, r.Status)
		}
		defer r.Body.Close()

		_, err = io.Copy(f, r.Body)
		if err != nil {
			return "", err
		}

		log.Info("Extracting nvmupdate.tar.gz")
		cmd := exec.Command("tar", "xzfv", nvmInstallDest+"/nvmupdate.tar.gz", "-C", nvmInstallDest)
		err = cmd.Run()
		if err != nil {
			return "", err
		}
	}
	return nvmInstallDest + "700Series/Linux_x64/nvmupdate64e", nil
}

func (fm *FortvilleManager) getNvmupdatePath(firmwareURL string) (string, error) {
	if fm.nvmupdatePath != "" {
		return fm.nvmupdatePath, nil
	}

	p, err := fm.installNvmupdate(firmwareURL)
	if err != nil {
		return "", err
	}

	fm.nvmupdatePath = p
	return p, nil
}

func (fm *FortvilleManager) getInventory(firmwareURL string) (DeviceInventory, error) {
	log := fm.Log.WithName("getInventory")
	nvmPath, err := fm.getNvmupdatePath(firmwareURL)
	if err != nil {
		log.Error(err, "Unable to get nvmupdate")
		return DeviceInventory{}, err
	}

	inventoryCmd := nvmPath + " -i -o " + inventoryOutFile
	_, err = exec.Command("bash", "-c", inventoryCmd).Output()
	if err != nil {
		log.Error(err, "Error when executing", "cmd", inventoryCmd)
		return DeviceInventory{}, err
	}

	invf, err := os.Open(inventoryOutFile)
	if err != nil {
		log.Error(err, "Error when opening inventory xml")
		return DeviceInventory{}, err
	}
	defer invf.Close()

	b, _ := ioutil.ReadAll(invf)

	var i DeviceInventory
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
