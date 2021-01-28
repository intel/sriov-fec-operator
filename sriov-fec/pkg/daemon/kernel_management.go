// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2021 Intel Corporation

package daemon

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/go-logr/logr"
	"gopkg.in/ini.v1"
)

var (
	kernelParams = []string{"intel_iommu=on", "iommu=pt"}

	setKernelParamsGrubby = []string{
		"chroot",
		"/host",
		"grubby",
		"--update-kernel=DEFAULT",
		fmt.Sprintf(`--args="%s"`, strings.Join(kernelParams, " ")),
	}

	osReleaseFilepath   = "/host/etc/os-release"
	procCmdlineFilePath = "/host/proc/cmdline"
)

func createKernelController(log logr.Logger) (*kernelController, error) {
	osReleaseFile, err := ini.Load(osReleaseFilepath)
	if err != nil {
		log.Error(err, "cannot determine OS, failed to read", "file", osReleaseFilepath)
		osReleaseFile = ini.Empty()
	}

	flat := osReleaseFile.Section("")
	osID := strings.ToLower(flat.Key("ID").String())
	osIDLike := strings.ToLower(flat.Key("ID_LIKE").String())

	var kernelArgsSetter func(log logr.Logger) error
	switch {
	case osID == "rhcos":
		kernelArgsSetter = rpmostreeBasedKernelArgsSetter
	case strings.Contains(osIDLike, "fedora"):
		kernelArgsSetter = grubbyBasedKernelArgsSetter
	default:
		kernelArgsSetter = errorReturningKernelArgsSetter
	}

	return &kernelController{
		log:           log.WithName(osID).WithName("kernelController"),
		setKernelArgs: kernelArgsSetter,
	}, nil
}

type kernelController struct {
	log           logr.Logger
	setKernelArgs func(log logr.Logger) error
}

func (k *kernelController) isAnyKernelParamsMissing() (bool, error) {
	log := k.log.WithName("isAnyKernelParamsMissing")
	cmdlineBytes, err := ioutil.ReadFile(procCmdlineFilePath)
	if err != nil {
		log.Error(err, "failed to read file contents", "path", procCmdlineFilePath)

		return false, err
	}
	cmdline := string(cmdlineBytes)
	for _, param := range kernelParams {
		if !strings.Contains(cmdline, param) {
			log.Info("missing kernel param", "param", param)
			return true, nil
		}
	}

	return false, nil
}

func (k *kernelController) addMissingKernelParams() error {
	log := k.log.WithName("addMissingKernelParams")
	return k.setKernelArgs(log)
}

func errorReturningKernelArgsSetter(log logr.Logger) error {
	return fmt.Errorf("cannot modify set kernel params, detected OS is not supported")
}

func grubbyBasedKernelArgsSetter(log logr.Logger) error {
	_, err := runExecCmd(setKernelParamsGrubby, log)
	log.V(2).Info("added missing params")
	return err
}

func rpmostreeBasedKernelArgsSetter(log logr.Logger) error {
	kargs, err := runExecCmd([]string{"chroot", "/host/", "rpm-ostree", "kargs"}, log)
	if err != nil {
		return err
	}

	log.V(2).Info("rpm-ostree", "kargs", kargs)

	for _, param := range kernelParams {
		if !strings.Contains(kargs, param) {
			log.V(2).Info("missing param - adding", "param", param)
			_, err = runExecCmd([]string{"chroot", "/host/", "rpm-ostree", "kargs", "--append", param}, log)
			if err != nil {
				return err
			}
		}
	}

	log.V(2).Info("added missing params")
	return nil
}
