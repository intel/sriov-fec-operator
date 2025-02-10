// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2025 Intel Corporation

package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/jaypipes/ghw"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AcceleratorDiscoveryConfig struct {
	VendorID  map[string]string
	Class     string
	SubClass  string
	Devices   map[string]string
	NodeLabel string
}

const (
	ConfigFileSizeLimitInBytes = 10485760 // 10 MB
	SriovPrefix                = "SRIOV_FEC_"
	PciPfStubDash              = "pci-pf-stub"
	PciPfStubUnderscore        = "pci_pf_stub"
	VfioPci                    = "vfio-pci"
	VfioPciUnderscore          = "vfio_pci"
	IgbUio                     = "igb_uio"
)

func LoadDiscoveryConfig(cfgPath string) (AcceleratorDiscoveryConfig, error) {
	var cfg AcceleratorDiscoveryConfig
	file, err := os.Open(filepath.Clean(cfgPath))
	if err != nil {
		return cfg, fmt.Errorf("failed to open config: %v", err)
	}
	defer file.Close()

	// get file stat
	stat, err := file.Stat()
	if err != nil {
		return cfg, fmt.Errorf("failed to get file stat: %v", err)
	}

	// check file size
	if stat.Size() > ConfigFileSizeLimitInBytes {
		return cfg, fmt.Errorf("config file size %d, exceeds limit %d bytes",
			stat.Size(), ConfigFileSizeLimitInBytes)
	}

	cfgData := make([]byte, stat.Size())
	bytesRead, err := file.Read(cfgData)
	if err != nil || int64(bytesRead) != stat.Size() {
		return cfg, fmt.Errorf("unable to read config: %s", filepath.Clean(cfgPath))
	}

	if err = json.Unmarshal(cfgData, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to unmarshal config: %v", err)
	}
	return cfg, nil
}

func SetOsEnvIfNotSet(key, value string, logger logr.Logger) error {
	if osValue := os.Getenv(key); osValue != "" {
		logger.Info("skipping ENV because it is already set", "key", key, "value", osValue)
		return nil
	}
	logger.Info("setting ENV var", "key", key, "value", value)
	return os.Setenv(key, value)
}

func IsSingleNodeCluster(c client.Client) (bool, error) {
	nodeList := &corev1.NodeList{}
	err := c.List(context.TODO(), nodeList)
	if err != nil {
		return false, err
	}
	if len(nodeList.Items) == 1 {
		return true, nil
	}
	return false, nil
}

var GetPCIDevices = func() ([]*ghw.PCIDevice, error) {
	pci, err := ghw.PCI()
	if err != nil {
		return nil, fmt.Errorf("failed to get PCI info: %v", err)
	}

	devices := pci.ListDevices()
	if len(devices) == 0 {
		return nil, fmt.Errorf("got 0 devices")
	}
	return devices, nil
}

func FindAccelerator(cfgPath string) (bool, string, error) {

	cfg, err := LoadDiscoveryConfig(cfgPath)
	if err != nil {
		return false, "", fmt.Errorf("failed to load config: %v", err)
	}

	devices, err := GetPCIDevices()
	if err != nil {
		return false, "", fmt.Errorf("failed to get PCI devices: %v", err)
	}

	for _, device := range devices {
		_, exist := cfg.VendorID[device.Vendor.ID]
		if !(exist &&
			device.Class.ID == cfg.Class &&
			device.Subclass.ID == cfg.SubClass) {
			continue
		}

		if _, ok := cfg.Devices[device.Product.ID]; ok {
			fmt.Printf("[%s]Accelerator found %v\n", cfgPath, device)
			return true, cfg.NodeLabel, nil
		}
	}
	return false, "", nil
}

// Function to find all VFs associated with a given PF PCI address
func FindVFs(pfPCIAddress string) ([]string, error) {
	// Path to the PF's SR-IOV directory in sysfs
	sriovPath := fmt.Sprintf("/sys/bus/pci/devices/%s/virtfn*", pfPCIAddress)

	// Find all symbolic links matching the virtfn* pattern
	vfLinks, err := filepath.Glob(sriovPath)
	if err != nil {
		return nil, err
	}

	vfAddresses := make([]string, 0, len(vfLinks))
	for _, vfLink := range vfLinks {
		// Read the target of the symbolic link to get the VF PCI address
		vfTarget, err := os.Readlink(vfLink)
		if err != nil {
			return nil, err
		}

		// Extract the VF PCI address from the target path
		vfAddress := filepath.Base(vfTarget)
		vfAddresses = append(vfAddresses, vfAddress)
	}

	return vfAddresses, nil
}

func GetVFDeviceID(pfPCIAddress string) (string, error) {
	// Path to the PF's SR-IOV VF device ID file in sysfs
	deviceIDPath := fmt.Sprintf("/sys/bus/pci/devices/%s/sriov_vf_device", pfPCIAddress)

	// Read the device ID from the file
	deviceID, err := os.ReadFile(deviceIDPath)
	if err != nil {
		return "", err
	}

	return strings.ToLower(strings.TrimSpace(string(deviceID))), nil
}
