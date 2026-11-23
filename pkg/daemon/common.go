// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2026 Intel Corporation

package daemon

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

func execCmd(args []string, log *logrus.Logger) (string, error) {
	return execAndSuppress(args, log, func(error) bool {
		return false
	})
}

func execAndSuppress(args []string, log *logrus.Logger, suppressError func(e error) bool) (string, error) {
	var cmd *exec.Cmd
	if len(args) == 0 {
		log.Error("provided cmd is empty")
		return "", errors.New("cmd is empty")
	}

	if pciConfigAccessCmds[filepath.Base(args[0])] {
		var cancel context.CancelFunc
		cmd, cancel = newPciConfigCmd(args...)
		defer cancel()
	} else if len(args) == 1 {
		cmd = exec.Command(args[0])
	} else {
		cmd = exec.Command(args[0], args[1:]...)
	}

	logArgs := redactArgs(args)
	log.WithFields(logrus.Fields{
		"cmd":  cmd.Path,
		"args": logArgs,
	}).Info("executing command")

	out, err := cmd.Output()
	if err != nil {
		if suppressError(err) {
			log.WithField("cmd", logArgs).WithError(err).Info("ignoring error")
		} else {
			log.WithField("cmd", logArgs).WithField("output", string(out)).WithError(err).Error("failed to execute command")
			return string(out), err
		}
	}

	output := string(out)
	log.WithField("output", output).Info("commands output")
	return output, nil
}

const redacted = "<redacted>"

// redactArgs returns a copy of args that is safe to log: the VFIO VF token
// passed to pf_bb_config with "-v <token>" is replaced. Anyone who knows the
// token can open the VFs of the accelerator through vfio-pci.
func redactArgs(args []string) []string {
	out := append([]string(nil), args...)
	if len(out) == 0 || filepath.Base(out[0]) != filepath.Base(pfConfigAppFilepath) {
		return out
	}
	for i := 1; i < len(out)-1; i++ {
		if out[i] == "-v" {
			out[i+1] = redacted
		}
	}
	return out
}
