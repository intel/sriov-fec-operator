// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2021 Intel Corporation

package daemon

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"strings"

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

func createKernelController(log *logrus.Logger) (*kernelController, error) {
	osReleaseFile, err := ini.Load(osReleaseFilepath)
	if err != nil {
		log.WithError(err).WithField("file", osReleaseFilepath).Error("cannot determine OS, failed to read")
		osReleaseFile = ini.Empty()
	}

	flat := osReleaseFile.Section("")
	osID := strings.ToLower(flat.Key("ID").String())
	osIDLike := strings.ToLower(flat.Key("ID_LIKE").String())

	var kernelArgsSetter func(log *logrus.Logger) error
	switch {
	case osID == "rhcos":
		kernelArgsSetter = rpmostreeBasedKernelArgsSetter
	case strings.Contains(osIDLike, "fedora"):
		kernelArgsSetter = grubbyBasedKernelArgsSetter
	default:
		kernelArgsSetter = errorReturningKernelArgsSetter
	}

	return &kernelController{
		log:           log,
		setKernelArgs: kernelArgsSetter,
	}, nil
}

type kernelController struct {
	log           *logrus.Logger
	setKernelArgs func(log *logrus.Logger) error
}

func (k *kernelController) isAnyKernelParamsMissing() (bool, error) {
	cmdlineBytes, err := ioutil.ReadFile(procCmdlineFilePath)
	if err != nil {
		k.log.WithError(err).WithField("path", procCmdlineFilePath).Error("failed to read file contents")

		return false, err
	}
	cmdline := string(cmdlineBytes)
	for _, param := range kernelParams {
		if !strings.Contains(cmdline, param) {
			k.log.WithField("param", param).Info("missing kernel param")
			return true, nil
		}
	}

	return false, nil
}

func (k *kernelController) addMissingKernelParams() error {
	return k.setKernelArgs(k.log)
}

func errorReturningKernelArgsSetter(log *logrus.Logger) error {
	return fmt.Errorf("cannot modify set kernel params, detected OS is not supported")
}

func grubbyBasedKernelArgsSetter(log *logrus.Logger) error {
	_, err := runExecCmd(setKernelParamsGrubby, log)
	log.Info("added missing params")
	return err
}

func rpmostreeBasedKernelArgsSetter(log *logrus.Logger) error {
	kargs, err := runExecCmd([]string{"chroot", "--userspec", "0", "/host/", "rpm-ostree", "kargs"}, log)
	if err != nil {
		return err
	}

	log.WithField("kargs", kargs).Info("rpm-ostree")

	for _, param := range kernelParams {
		if !strings.Contains(kargs, param) {
			log.WithField("param", param).Info("missing param - adding")
			_, err = runExecCmd([]string{"chroot", "--userspec", "0", "/host/", "rpm-ostree", "kargs", "--append", param}, log)
			if err != nil {
				return err
			}
		}
	}

	log.Info("added missing params")
	return nil
}
