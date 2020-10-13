// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"bytes"
	"context"
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
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
)

var (
	supportedFortville = `"X710\|XXV710\|XL710"`
	// nvmInstallDest     = "/nvmupdate/"
	nvmInstallDest   = "/root/test/"
	inventoryOutFile = nvmInstallDest + "inventory.xml"
)

type DaemonController struct {
	Log           logr.Logger
	status        *fpgav1.N3000NodeStatus
	d             *Daemon
	nvmupdatePath string
}

func newDaemonController(d *Daemon) *DaemonController {
	dc := &DaemonController{
		Log: d.Log,
		d:   d,
	}

	log := dc.Log.WithName("newDaemonController")
	err := dc.updateNodeStatus()
	if err != nil {
		log.Error(err, "Unable to update N3000NodeStatus")
	}
	return dc
}

func (dc *DaemonController) updateNodeStatus() error {
	log := dc.Log.WithName("updateNodeStatus")
	n, err := dc.getN3000Node()
	if err != nil {
		if apierr.IsNotFound(err) {
			log.V(2).Info("N3000Node resource not found - creating new one with basic status ")
			ns, err := dc.createBasicNodeStatus()
			if err != nil {
				return err
			}

			n := fpgav1.N3000Node{}
			n.Status = *ns
			n.Name = "n3000node-" + dc.d.nodeName
			n.Namespace = namespace

			o := &unstructured.Unstructured{}
			err = scheme.Scheme.Convert(&n, o, nil)
			_, err = dc.d.client.Resource(nodeGVR).Namespace(namespace).
				Create(context.TODO(), o, metav1.CreateOptions{})
			if err != nil {
				log.Error(err, "Error when creating N3000Node resource")
				return err
			}
			return nil
		}
		return err
	}

	log.V(2).Info("N3000Node resource found - updating basic status")
	ns, err := dc.createBasicNodeStatus()
	if err != nil {
		return err
	}

	log.V(2).Info("N3000Node resource found - updating status with nvmupdate inventory data")
	i, err := dc.getInventory()
	if err != nil {
		log.Error(err, "Unable to get inventory...using basic status only")
	} else {
		dc.processInventory(&i, ns) // fill ns with data from inventory
	}

	n.Status = *ns

	o := &unstructured.Unstructured{}
	err = scheme.Scheme.Convert(n, o, nil)
	_, err = dc.d.client.Resource(nodeGVR).Namespace(namespace).
		UpdateStatus(context.TODO(), o, metav1.UpdateOptions{})
	if err != nil {
		log.Error(err, "Error when updating N3000NodeStatus resource")
		return err
	}
	return nil
}

func (dc *DaemonController) processInventory(inv *DeviceInventory, ns *fpgav1.N3000NodeStatus) {
	log := dc.Log.WithName("processInventory")
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

func (dc *DaemonController) start() {

}

func (dc *DaemonController) createBasicNodeStatus() (*fpgav1.N3000NodeStatus, error) {
	ns := &fpgav1.N3000NodeStatus{}
	fStatus, err := dc.getNetworkDevices()
	if err != nil {
		return nil, err
	}
	ns.Fortville = fStatus

	// TODO: fill with fpga basic status
	return ns, nil
}

func (dc *DaemonController) getN3000NodeStatus() (*fpgav1.N3000NodeStatus, error) {
	n, err := dc.getN3000Node()
	if err != nil {
		return nil, err
	}

	return &n.Status, nil
}

func (dc *DaemonController) getN3000Node() (*fpgav1.N3000Node, error) {
	log := dc.Log.WithName("getN3000Node")
	result, err := dc.d.client.Resource(nodeGVR).Namespace(namespace).Get(context.TODO(), "n3000node-"+dc.d.nodeName, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "Error when getting N3000Node resource")
		return nil, err
	}

	n := &fpgav1.N3000Node{}
	err = scheme.Scheme.Convert(result, n, nil)
	if err != nil {
		log.Error(err, "Unable to convert Unstructured to N3000Node")
		return nil, err
	}
	return n, nil
}

func (dc *DaemonController) getNetworkDevices() ([]fpgav1.N3000FortvilleStatus, error) {
	log := dc.Log.WithName("getNetworkDevices")
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

func (dc *DaemonController) installNvmupdate() (string, error) {
	log := dc.Log.WithName("installNvmupdate")
	_, err := os.Stat(nvmInstallDest + "700Series/Linux_x64/nvmupdate64e")
	if os.IsNotExist(err) {
		eb := dc.d.GetEventBuffer()
		if eb.newObj == nil {
			return "", errors.New("No new Object in event")
		}

		if eb.newObj.Spec.Fortville.FirmwareURL == "" {
			return "", errors.New("Unable to install nvmupdate - empty .Spec.Fortville.FirmwareURL")
		}

		log.Info("nvmupdate tool not found - downloading", "url", eb.newObj.Spec.Fortville.FirmwareURL)
		f, err := os.Create(nvmInstallDest + "/nvmupdate.tar.gz")
		if err != nil {
			return "", err
		}
		defer f.Close()

		r, err := http.Get(eb.newObj.Spec.Fortville.FirmwareURL)
		if err != nil {
			return "", err
		}

		if r.StatusCode != http.StatusOK {
			return "", fmt.Errorf("Unable to download nvmupdate package from: %s err: %s",
				eb.newObj.Spec.Fortville.FirmwareURL, r.Status)
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

func (dc *DaemonController) getNvmupdatePath() (string, error) {
	if dc.nvmupdatePath != "" {
		return dc.nvmupdatePath, nil
	}

	p, err := dc.installNvmupdate()
	if err != nil {
		return "", err
	}

	dc.nvmupdatePath = p
	return p, nil
}

func (dc *DaemonController) getInventory() (DeviceInventory, error) {
	log := dc.Log.WithName("getInventory")
	nvmPath, err := dc.getNvmupdatePath()
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
