// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jaypipes/ghw"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type AcceleratorDiscoveryConfig struct {
	VendorID  map[string]string
	Class     string
	SubClass  string
	Devices   map[string]string
	NodeLabel string
}

const (
	configPath                 = "/labeler-workspace/config/accelerators.json"
	configFilesizeLimitInBytes = 10485760 //10 MB
)

var getInclusterConfigFunc = rest.InClusterConfig

func loadConfig(cfgPath string) (AcceleratorDiscoveryConfig, error) {
	var cfg AcceleratorDiscoveryConfig
	file, err := os.Open(filepath.Clean(cfgPath))
	if err != nil {
		return cfg, fmt.Errorf("Failed to open config: %v", err)
	}
	defer file.Close()

	// get file stat
	stat, err := file.Stat()
	if err != nil {
		return cfg, fmt.Errorf("Failed to get file stat: %v", err)
	}

	// check file size
	if stat.Size() > configFilesizeLimitInBytes {
		return cfg, fmt.Errorf("Config file size %d, exceeds limit %d bytes",
			stat.Size(), configFilesizeLimitInBytes)
	}

	cfgData := make([]byte, stat.Size())
	bytesRead, err := file.Read(cfgData)
	if err != nil || int64(bytesRead) != stat.Size() {
		return cfg, fmt.Errorf("Unable to read config: %s", filepath.Clean(cfgPath))
	}

	if err = json.Unmarshal(cfgData, &cfg); err != nil {
		return cfg, fmt.Errorf("Failed to unmarshal config: %v", err)
	}
	return cfg, nil
}

var getPCIDevices = func() ([]*ghw.PCIDevice, error) {
	pci, err := ghw.PCI()
	if err != nil {
		return nil, fmt.Errorf("Failed to get PCI info: %v", err)
	}

	devices := pci.ListDevices()
	if len(devices) == 0 {
		return nil, fmt.Errorf("Got 0 devices")
	}
	return devices, nil
}

func findAccelerator(cfg *AcceleratorDiscoveryConfig) (bool, error) {
	if cfg == nil {
		return false, fmt.Errorf("config not provided")
	}

	devices, err := getPCIDevices()
	if err != nil {
		return false, fmt.Errorf("Failed to get PCI devices: %v", err)
	}

	for _, device := range devices {
		_, exist := cfg.VendorID[device.Vendor.ID]
		if !(exist &&
			device.Class.ID == cfg.Class &&
			device.Subclass.ID == cfg.SubClass) {
			continue
		}

		if _, ok := cfg.Devices[device.Product.ID]; ok {
			fmt.Printf("Accelerator found %v\n", device)
			return true, nil
		}
	}
	return false, nil
}

func setNodeLabel(nodeName, label string, removeLabel bool) error {
	cfg, err := getInclusterConfigFunc()
	if err != nil {
		return fmt.Errorf("Failed to get cluster config: %v\n", err.Error())
	}
	cli, err := clientset.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("Failed to initialize clientset: %v\n", err.Error())
	}

	node, err := cli.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Failed to get the node object: %v\n", err)
	}
	nodeLabels := node.GetLabels()
	if removeLabel {
		delete(nodeLabels, label)
	} else {
		nodeLabels[label] = ""

	}
	node.SetLabels(nodeLabels)
	_, err = cli.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("Failed to update the node object: %v\n", err)
	}
	return nil
}

func acceleratorDiscovery(cfgPath string) error {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("Failed to load config: %v", err)
	}
	accFound, err := findAccelerator(&cfg)
	if err != nil {
		return fmt.Errorf("Failed to find accelerator: %v", err)
	}
	nodeName := os.Getenv("NODENAME")
	if nodeName == "" {
		return fmt.Errorf("NODENAME environment variable is empty")
	}
	return setNodeLabel(nodeName, cfg.NodeLabel, !accFound)
}

func main() {
	if err := acceleratorDiscovery(configPath); err != nil {
		fmt.Printf("Accelerator discovery failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Accelerator discovery finished successfully\n")
	os.Exit(0)
}
