// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"bytes"
	"errors"
	"os/exec"

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
	} else if len(args) == 1 {
		cmd = exec.Command(args[0])
	} else {
		cmd = exec.Command(args[0], args[1:]...)
	}

	log.WithField("cmd", cmd).Info("executing command")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if suppressError(err) {
			log.WithField("cmd", args).WithError(err).Info("ignoring error")
		} else {
			log.WithField("cmd", args).WithField("output", string(out)).
				WithField("stderr", stderr.String()).
				WithError(err).Error("failed to execute command")
			return string(out), err
		}
	}

	output := string(out)
	log.WithField("output", output).Info("commands output")
	return output, nil
}
