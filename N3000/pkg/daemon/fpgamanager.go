// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"github.com/go-logr/logr"
	fpgav1 "github.com/otcshare/openshift-operator/N3000/api/v1"
)

type FPGAManager struct {
	Log logr.Logger
	d   *Daemon
}

func (fpgaM *FPGAManager) getFPGAStatus() ([]fpgav1.N3000FpgaStatus, error) {
	//log := dc.Log.WithName("getFPGAStatus")

	//TODO fpga get status data
	devs := make([]fpgav1.N3000FpgaStatus, 0)
	//...

	return devs, nil
}
