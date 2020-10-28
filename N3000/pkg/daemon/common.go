// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	"io"
	"net/http"
	"os"
)

func verifyChecksum(f *os.File, expected string) (bool, error) {
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, errors.New("Failed to copy file to calculate md5")
	}

	if hex.EncodeToString(h.Sum(nil)) != expected {
		return false, fmt.Errorf("Checksum mismatch: %s, expected: %s",
			hex.EncodeToString(h.Sum(nil)), expected)
	}

	return true, nil
}

func downloadImage(path string, url string, checksum string) error {
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
		ret, err := verifyChecksum(f, checksum)
		if !ret {
			return err
		}
	}
	return nil
}

func getImage(path string, url string, checksum string, log logr.Logger) error {
	f, err := os.Open(path)
	if err == nil {
		ret, _ := verifyChecksum(f, checksum)
		if !ret {
			f.Close()
			err := os.Remove(path)
			if err != nil {
				return fmt.Errorf("Unable to remove old image file: %s",
					path)
			}
			f = nil
		} else {
			log.Info("Image already downloaded", "path:", path)
			defer f.Close()
		}
	} else {
		f = nil
	}

	if f == nil {
		os.Remove(path)
		log.Info("Downloading image", "url:", url)
		err := downloadImage(path, url, checksum)
		if err != nil {
			log.Error(err, "Unable to download Image")
			return err
		}
	}
	return nil
}

func createFolder(path string, log logr.Logger) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		errDir := os.MkdirAll(path, 0777)
		if errDir != nil {
			log.Info("Unable to create", "path:", path)
			return err
		}
	}
	return nil
}
