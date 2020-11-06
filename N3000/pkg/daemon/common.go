// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/go-logr/logr"
)

func verifyChecksum(path, expected string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, errors.New("Failed to open file to calculate md5")
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, errors.New("Failed to copy file to calculate md5")
	}
	if hex.EncodeToString(h.Sum(nil)) != expected {
		return false, nil
	}

	return true, nil
}

func downloadImage(path, url, checksum string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	r, err := http.Get(url)
	if err != nil {
		return err
	}

	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("Unable to download image from: %s err: %s",
			url, r.Status)
	}
	defer r.Body.Close()

	_, err = io.Copy(f, r.Body)
	if err != nil {
		return err
	}

	if checksum != "" {
		match, err := verifyChecksum(path, checksum)
		if err != nil {
			return err
		}
		if !match {
			return fmt.Errorf("Checksum mismatch in downloaded file: %s", url)
		}
	}
	return nil
}

func getImage(path, url, checksum string, log logr.Logger) error {
	_, err := os.Stat(path)
	if err == nil {
		ret, err := verifyChecksum(path, checksum)
		if err != nil {
			return err
		}
		if ret {
			log.Info("Image already downloaded", "path", path)
			return nil
		}
		err = os.Remove(path)
		if err != nil {
			return fmt.Errorf("Unable to remove old image file: %s",
				path)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	log.Info("Downloading image", "url", url)
	if err := downloadImage(path, url, checksum); err != nil {
		log.Error(err, "Unable to download Image")
		return err
	}
	return nil
}

func createFolder(path string, log logr.Logger) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		errDir := os.MkdirAll(path, 0777)
		if errDir != nil {
			log.Info("Unable to create", "path", path)
			return err
		}
	}
	return nil
}

func runExec(cmd *exec.Cmd, log logr.Logger, dryRun bool) (string, error) {
	if dryRun {
		log.Info("Run exec in dryrun mode", "command", cmd)
		return "", nil
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Info("Executed unsuccessfully", "cmd", cmd,
			"output", string(output))
		return "", err
	}
	return string(output), nil
}
