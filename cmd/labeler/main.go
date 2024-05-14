// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2024 Intel Corporation

package main

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intel/sriov-fec-operator/pkg/common/utils"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	configPath    = "/labeler-workspace/config/accelerators.json"
	vrbconfigPath = "/labeler-workspace/config/accelerators_vrb.json"
)

var getInclusterConfigFunc = rest.InClusterConfig

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

func acceleratorDiscovery(cfgPath string, vrbCfgPath string) error {

	fecAccFound, fecNodeLabel, err1 := utils.FindAccelerator(cfgPath)
	vrbAccFound, vrbNodeLabel, err2 := utils.FindAccelerator(vrbCfgPath)

	if err1 != nil && err2 != nil {
		return fmt.Errorf("Failed to find accelerator: %v \n%v\n", err1, err2)
	}
	nodeName := os.Getenv("NODENAME")
	if nodeName == "" {
		return fmt.Errorf("NODENAME environment variable is empty")
	}

	nodeLabel := ""
	if fecAccFound {
		nodeLabel = fecNodeLabel
	} else if vrbAccFound {
		nodeLabel = vrbNodeLabel
	}

	return setNodeLabel(nodeName, nodeLabel, !(fecAccFound || vrbAccFound))
}

func main() {
	if err := acceleratorDiscovery(configPath, vrbconfigPath); err != nil {
		fmt.Printf("Accelerator discovery failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Accelerator discovery finished successfully\n")
	os.Exit(0)
}
