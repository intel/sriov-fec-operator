// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2021 Intel Corporation

package utils

import (
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"os"
	"path/filepath"
)

type AcceleratorDiscoveryConfig struct {
	VendorID  map[string]string
	Class     string
	SubClass  string
	Devices   map[string]string
	NodeLabel string
}

const (
	CONFIG_FILE_SIZE_LIMIT_IN_BYTES = 10485760 //10 MB
	SRIOV_PREFIX                    = "SRIOV_FEC_"
	PCI_PF_STUB_DASH                = "pci-pf-stub"
	PCI_PF_STUB_UNDERSCORE          = "pci_pf_stub"
	VFIO_PCI                        = "vfio-pci"
	IGB_UIO                         = "igb_uio"
)

func LoadDiscoveryConfig(cfgPath string) (AcceleratorDiscoveryConfig, error) {
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
	if stat.Size() > CONFIG_FILE_SIZE_LIMIT_IN_BYTES {
		return cfg, fmt.Errorf("Config file size %d, exceeds limit %d bytes",
			stat.Size(), CONFIG_FILE_SIZE_LIMIT_IN_BYTES)
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

func SetOsEnvIfNotSet(key, value string, logger logr.Logger) error {
	if osValue := os.Getenv(key); osValue != "" {
		logger.Info("skipping ENV because it is already set", "key", key, "value", osValue)
		return nil
	}
	logger.Info("setting ENV var", "key", key, "value", value)
	return os.Setenv(key, value)
}
