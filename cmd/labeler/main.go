// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2022 Intel Corporation

package main

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/smart-edge-open/sriov-fec-operator/sriov-fec/pkg/common/utils"
	"github.com/jaypipes/ghw"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	configPath = "/labeler-workspace/config/accelerators.json"
)

var getInclusterConfigFunc = rest.InClusterConfig

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

func findAccelerator(cfg *utils.AcceleratorDiscoveryConfig) (bool, error) {
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
	cfg, err := utils.LoadDiscoveryConfig(cfgPath)
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
