// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Intel Corporation

package daemon

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type resourceNamePredicate struct {
	predicate.Funcs
	requiredName string
	log          *logrus.Logger
}

func (r resourceNamePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectNew.GetName() != r.requiredName {
		r.log.WithField("expected name", r.requiredName).Info("CR intended for another node - ignoring")
		return false
	}
	return true
}

func (r resourceNamePredicate) Create(e event.CreateEvent) bool {
	if e.Object.GetName() != r.requiredName {
		r.log.WithField("expected name", r.requiredName).Info("CR intended for another node - ignoring")
		return false
	}
	return true
}

// returns result indicating necessity of re-queuing Reconcile after configured resyncPeriod
func requeueLater() (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: resyncPeriod}, nil
}

// returns result indicating necessity of re-queuing Reconcile(...) immediately; non-nil err will be logged by controller
func requeueNowWithError(e error) (reconcile.Result, error) {
	return reconcile.Result{Requeue: true}, e
}

// returns result indicating necessity of re-queuing Reconcile(...):
// immediately - in case when given err is non-nil;
// on configured schedule, when err is nil
func requeueLaterOrNowIfError(e error) (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: resyncPeriod}, e
}

// operator is unable to write to sysfs files if device is currently in use
// this function is supposed to either write successfully to file or return timeout error
func writeFileWithTimeout(filename, data string) error {
	done := make(chan struct{})
	var err error

	go func() {
		err = os.WriteFile(filename, []byte(data), os.ModeAppend)
		done <- struct{}{}
	}()

	select {
	case <-done:
		return err
	case <-time.After(60 * time.Second):
		return fmt.Errorf("failed to write to sysfs file. Usually it means that device is in use by other process")
	}

}

func isHardLink(path string) (bool, error) {
	var stat syscall.Stat_t

	err := syscall.Stat(path, &stat)
	if err != nil {
		return false, err
	}

	if stat.Nlink > 1 {
		return true, nil
	}

	return false, nil
}

func OpenNoLinks(path string) (*os.File, error) {
	return OpenFileNoLinks(path, os.O_RDONLY, 0)
}

func CreateNoLinks(path string) (*os.File, error) {
	return OpenFileNoLinks(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

func OpenFileNoLinks(path string, flag int, perm os.FileMode) (*os.File, error) {
	// O_NOFOLLOW - If the trailing component (i.e., basename) of pathname is a symbolic link,
	// then the open fails, with the error ELOOP.
	file, err := os.OpenFile(path, flag|syscall.O_NOFOLLOW, perm)
	if err != nil {
		return nil, err
	}

	hardLink, err := isHardLink(path)
	if err != nil {
		file.Close()
		return nil, err
	}

	if hardLink {
		file.Close()
		return nil, fmt.Errorf("%v is a hardlink", path)
	}

	return file, nil
}

func NewSecureHttpsClient(cert *x509.Certificate) (*http.Client, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("failed to get syscerts - %v", err)
	}
	certPool.AddCert(cert)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:    certPool,
			ClientAuth: tls.RequireAndVerifyClientCert,
		},
	}
	httpsClient := http.Client{
		Transport: transport,
	}
	return &httpsClient, nil
}

func verifyChecksum(path, expected string) (bool, error) {
	if expected == "" {
		return false, nil
	}
	f, err := OpenNoLinks(path)
	if err != nil {
		return false, errors.New("failed to open file to calculate sha-1")
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, errors.New("failed to copy file to calculate sha-1")
	}
	if hex.EncodeToString(h.Sum(nil)) != strings.ToLower(expected) {
		return false, nil
	}

	return true, nil
}

func DownloadFile(path, url, checksum string, client *http.Client) error {
	f, err := CreateNoLinks(path)
	if err != nil {
		return err
	}
	defer f.Close()

	r, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("unable to download image from: %s err: %s", url, err)
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("unable to download image from: %s err: %s",
			url, r.Status)
	}

	_, err = io.Copy(f, r.Body)
	if err != nil {
		return err
	}

	match, err := verifyChecksum(path, checksum)
	if err != nil {
		return err
	}
	if !match {
		err = os.Remove(path)
		if err != nil {
			return err
		}
		return fmt.Errorf("checksum mismatch in downloaded file: %s", url)
	}
	return nil
}

func Untar(tarFile string, dstPath string, log *logrus.Logger) (string, error) {
	log.Info("Untar file ", tarFile, "into destination path ", dstPath)

	f, err := OpenNoLinks(tarFile)
	if err != nil {
		log.Error(err, "Unable to open file")
		return "", err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var nfDst string
	for {
		fh, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error(err, "Error when reading tar")
			return "", err
		}
		if fh == nil {
			err = fmt.Errorf("invalid header in file %s", fh.Name)
			log.Error(err, "Invalid tar header")
			return "", err
		}

		nfDst = filepath.Join(dstPath, fh.Name)

		// Check for ZipSlip (Directory traversal)
		// https://snyk.io/research/zip-slip-vulnerability
		if !strings.HasPrefix(nfDst, filepath.Clean(dstPath)+string(os.PathSeparator)) {
			return "", fmt.Errorf("illegal file path: %s", nfDst)
		}

		fhCopy := *fh
		trCopy := *tr

		if err = tarHandler(&fhCopy, nfDst, &trCopy); err != nil {
			return "", err
		}
	}
	return nfDst, nil
}

func tarHandler(fh *tar.Header, nfDst string, tr *tar.Reader) error {
	switch fh.Typeflag {
	case tar.TypeReg:
		nf, err := OpenFileNoLinks(nfDst, os.O_CREATE|os.O_RDWR, os.FileMode(fh.Mode))
		if err != nil {
			log.Error("Error creating a new file")
			return err
		}
		defer nf.Close()

		_, err = io.Copy(nf, tr)
		if err != nil {
			log.Error("Error copying the file data")
			return err
		}
	case tar.TypeDir:
		err := os.MkdirAll(nfDst, fh.FileInfo().Mode())
		if err != nil {
			log.Error("Error creating the directory")
			return err
		}
		log.Infof("Created Directory: %s\n", fh.FileInfo().Name())
	case tar.TypeSymlink, tar.TypeLink:
		log.Info("Skipping (sym)link", "filename", fh.FileInfo().Name())
	default:
		err := fmt.Errorf("unable to untar type: %c in file %s", fh.Typeflag, fh.Name)
		log.Error(err, "Invalid untar type")
		return err
	}
	return nil
}
