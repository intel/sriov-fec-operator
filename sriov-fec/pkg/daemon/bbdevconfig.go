// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	sriovv1 "github.com/open-ness/openshift-operator/sriov-fec/api/v1"
	"gopkg.in/ini.v1"
)

const (
	mode         = "MODE"
	ul           = "UL"
	dl           = "DL"
	flr          = "FLR"
	pf_mode_en   = "pf_mode_en"
	bandwidth    = "bandwidth"
	load_balance = "load_balance"
	vfqmap       = "vfqmap"
	flr_time_out = "flr_time_out"

	pfConfigAppFilepath = "/sriov_workdir/pf_bb_config"
)

func generateN3000BBDevConfigFile(nc *sriovv1.N3000BBDevConfig, file string) error {
	if nc == nil {
		return errors.New("received nil N3000BBDevConfig")
	}

	cfg := ini.Empty()
	err := cfg.NewSections(mode, ul, dl, flr)
	if err != nil {
		return fmt.Errorf("Unable to create sections in bbdevconfig")
	}

	var modeValue string
	if nc.PFMode {
		modeValue = "1"
	} else {
		modeValue = "0"
	}
	cfg.Section(mode).Key(pf_mode_en).SetValue(modeValue)
	cfg.Section(ul).Key(bandwidth).SetValue(strconv.Itoa(nc.Uplink.Bandwidth))
	cfg.Section(ul).Key(load_balance).SetValue(strconv.Itoa(nc.Uplink.LoadBalance))
	cfg.Section(ul).Key(vfqmap).SetValue(nc.Uplink.Queues.String())
	cfg.Section(dl).Key(bandwidth).SetValue(strconv.Itoa(nc.Downlink.Bandwidth))
	cfg.Section(dl).Key(load_balance).SetValue(strconv.Itoa(nc.Downlink.LoadBalance))
	cfg.Section(dl).Key(vfqmap).SetValue(nc.Downlink.Queues.String())
	cfg.Section(flr).Key(flr_time_out).SetValue(strconv.Itoa(nc.FLRTimeOut))

	err = cfg.SaveTo(file)
	if err != nil {
		return fmt.Errorf("Unable to write config to file: %s", file)
	}
	return nil
}

// runPFConfig executes a pf-bb-config tool
// deviceName is one of: FPGA_LTE or FPGA_5GNR or ACC100
// cfgFilepath is a filepath to the config
// pciAddress points to a specific PF device
func runPFConfig(log logr.Logger, deviceName, cfgFilepath, pciAddress string) error {
	switch deviceName {
	case "FPGA_LTE", "FPGA_5GNR", "ACC100":
	default:
		return fmt.Errorf("incorrect deviceName for pf config: %s", deviceName)
	}
	_, err := runExecCmd([]string{pfConfigAppFilepath, deviceName, "-c", cfgFilepath, "-p", pciAddress}, log)
	return err
}
