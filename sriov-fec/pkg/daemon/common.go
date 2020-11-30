// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"errors"
	"os/exec"

	"github.com/go-logr/logr"
)

func execCmd(args []string, log logr.Logger) (string, error) {
	var cmd *exec.Cmd
	if len(args) == 0 {
		log.Error(nil, "provided cmd is empty")
		return "", errors.New("cmd is empty")
	} else if len(args) == 1 {
		cmd = exec.Command(args[0])
	} else {
		cmd = exec.Command(args[0], args[1:]...)
	}

	log.Info("executing command", "cmd", cmd)

	out, err := cmd.Output()
	if err != nil {
		log.Error(err, "failed to execute command", "cmd", args, "output", string(out))
		return "", err
	}

	output := string(out)
	log.Info("commands output", "output", output)
	return output, nil
}
