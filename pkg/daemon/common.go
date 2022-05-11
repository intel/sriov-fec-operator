// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"errors"
	"github.com/sirupsen/logrus"
	"os/exec"
)

func execCmd(args []string, log *logrus.Logger) (string, error) {
	var cmd *exec.Cmd
	if len(args) == 0 {
		log.Error("provided cmd is empty")
		return "", errors.New("cmd is empty")
	} else if len(args) == 1 {
		cmd = exec.Command(args[0])
	} else {
		cmd = exec.Command(args[0], args[1:]...)
	}

	log.WithField("cmd", cmd).Info("executing command")

	out, err := cmd.Output()
	if err != nil {
		log.WithField("cmd", args).WithField("output", string(out)).WithError(err).Error("failed to execute command")
		return "", err
	}

	output := string(out)
	log.WithField("output", output).Info("commands output")
	return output, nil
}
