// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2024 Intel Corporation

package daemon

import (
	"archive/tar"
	"compress/gzip"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"io"
	"io/fs"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/intel/sriov-fec-operator/pkg/common/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("verifyChecksum", func() {
	var _ = It("will return false and error if it's not able to open file", func() {
		result, err := verifyChecksum("./invalidfile", "somechecksum")
		Expect(err).To(HaveOccurred())
		Expect(result).To(Equal(false))
	})

	var _ = It("will return false and no error if the expected is empty", func() {
		result, err := verifyChecksum("./invalidfile", "")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(false))
	})

	var _ = It("will return false if checksum does not match", func() {
		tmpfile, err := os.CreateTemp(".", "update")
		Expect(err).ToNot(HaveOccurred())

		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"))
		Expect(err).ToNot(HaveOccurred())
		err = tmpfile.Close()
		Expect(err).ToNot(HaveOccurred())

		result, err := verifyChecksum(tmpfile.Name(), "somechecksum")
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(false))
	})

	var _ = It("will return true if checksum does match", func() {
		tmpfile, err := os.CreateTemp(".", "testfile")
		Expect(err).ToNot(HaveOccurred())

		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"))
		Expect(err).ToNot(HaveOccurred())
		err = tmpfile.Close()
		Expect(err).ToNot(HaveOccurred())

		f, _ := os.Open(tmpfile.Name())

		h := sha1.New()
		_, err = io.Copy(h, f)
		Expect(err).ToNot(HaveOccurred())

		result, err := verifyChecksum(tmpfile.Name(), hex.EncodeToString(h.Sum(nil)))
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(true))
	})
})

var _ = Describe("DownloadFile", func() {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Hello"))
	}))
	defer server.Close()

	secureClient, err := NewSecureHttpsClient(server.Certificate())
	Expect(err).ToNot(HaveOccurred())

	var _ = It("will return error if URL is valid and certificate isn't", func() {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("Hello"))
		}))
		defer srv.Close()

		key, err := rsa.GenerateKey(rand.Reader, 2048)
		Expect(err).ToNot(HaveOccurred())

		tml := x509.Certificate{
			NotBefore:    time.Now(),
			NotAfter:     time.Now().AddDate(5, 0, 0),
			SerialNumber: big.NewInt(123123),
			Subject: pkix.Name{
				CommonName:   "New Name",
				Organization: []string{"New Org."},
			},
			BasicConstraintsValid: true,
		}
		certPem, err := x509.CreateCertificate(rand.Reader, &tml, &tml, &key.PublicKey, key)
		Expect(err).ToNot(HaveOccurred())

		crt, err := x509.ParseCertificates(certPem)
		Expect(err).ToNot(HaveOccurred())

		clientWithInvalidCertificate, err := NewSecureHttpsClient(crt[0])
		Expect(err).ToNot(HaveOccurred())

		err = DownloadFile("/tmp/somefolder", srv.URL, "", clientWithInvalidCertificate)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("certificate signed by unknown authority"))
	})

	var _ = It("will return error if url format is invalid", func() {
		defer os.Remove("/tmp/somefileanme")
		err = DownloadFile("/tmp/somefolder", "/tmp/fake", "", secureClient)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unsupported protocol"))
	})

	var _ = It("will return error if file already exists, but cannot acquire file", func() {
		tmpfile, err := os.CreateTemp("/tmp", "somefilename")
		defer os.Remove(tmpfile.Name())
		Expect(err).ToNot(HaveOccurred())

		err = DownloadFile(tmpfile.Name(), "http://0.0.0.0/tmp/fake", "check", secureClient)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unable to download image"))
	})

	var _ = It("will return a download error if file already exists and checksum matches, but no file with url found", func() {
		filePath := "/tmp/updatefile_101.tar.gz"
		url := "/tmp/fake"

		err := os.WriteFile(filePath, []byte("1010101"), 0666)
		Expect(err).To(BeNil())
		defer os.Remove(filePath)

		err = DownloadFile(filePath, url, "63effa2530d088a06f071bc5f016f8d4", secureClient)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unsupported protocol"))
	})

	var _ = It("will return a download error if file already exists, checksum matches and url file found, but url does not match", func() {
		filePath := "/tmp/updatefile_101.tar.gz"
		fileWithUrl := filePath + ".url"
		url := "/tmp/fake"

		err := os.WriteFile(filePath, []byte("1010101"), 0666)
		Expect(err).To(BeNil())
		defer os.Remove(filePath)

		err = os.WriteFile(fileWithUrl, []byte(filePath), 0666)
		Expect(err).To(BeNil())
		defer os.Remove(fileWithUrl)

		err = DownloadFile(filePath, url, "63effa2530d088a06f071bc5f016f8d4", secureClient)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unsupported protocol"))
	})

	var _ = It("will return error if filename is invalid", func() {
		err := DownloadFile("", "/tmp/fake", "bf51ac6aceed5ca4227e640046ad9de4", secureClient)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no such file or directory"))
	})
})

var _ = Describe("Untar", func() {
	log := utils.NewLogger()

	var _ = It("will return error if it's not able to open file", func() {
		_, err := Untar("./somesrcfile", "./somedstfile", log)
		Expect(err).To(HaveOccurred())
	})

	var _ = It("will return error if input file is not an archive", func() {
		tmpfile, err := os.CreateTemp(".", "testfile")
		Expect(err).ToNot(HaveOccurred())

		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"))
		Expect(err).ToNot(HaveOccurred())
		err = tmpfile.Close()
		Expect(err).ToNot(HaveOccurred())

		_, err = Untar(tmpfile.Name(), "./somedstfile", log)
		Expect(err).To(HaveOccurred())
	})

	var _ = It("will extract valid archive", func() {
		workDir, err := os.MkdirTemp("", "untar-test")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(workDir)

		dirToBeArchived := "sample-archive"
		pathOfDirToBeArchived := "testdata/archives/"
		dirToBeArchivedPath := filepath.Join(pathOfDirToBeArchived, dirToBeArchived)

		tarFilePath := filepath.Join(workDir, "test-archive.tar.gz")
		createTarArchive(tarFilePath, pathOfDirToBeArchived, dirToBeArchived)
		_, err = Untar(tarFilePath, workDir, log)
		Expect(err).ToNot(HaveOccurred())

		_ = filepath.WalkDir(dirToBeArchivedPath, func(path string, d fs.DirEntry, err error) error {
			Expect(err).ShouldNot(HaveOccurred())
			actualRelPath, err := filepath.Rel(dirToBeArchivedPath, path)
			Expect(err).ShouldNot(HaveOccurred())
			expectedFile := filepath.Join(workDir, dirToBeArchived, actualRelPath)
			_, err = os.Stat(expectedFile)
			Expect(err).To(Not(HaveOccurred()), expectedFile, "has not been extracted from tar.gz archive")
			return nil
		})
	})
})

var _ = Describe("OpenNoLinks", func() {
	var _ = It("will succeed if a path is neither symlink nor hard link", func() {
		tmpFile, err := os.CreateTemp("", "regularFile")
		defer os.Remove(tmpFile.Name())
		Expect(err).ToNot(HaveOccurred())

		err = tmpFile.Close()
		Expect(err).ToNot(HaveOccurred())

		f, err := OpenNoLinks(tmpFile.Name())
		Expect(err).ToNot(HaveOccurred())
		Expect(f).ToNot(BeNil())

		err = f.Close()
		Expect(err).ToNot(HaveOccurred())
	})

	var _ = It("will return error if a path is a symlink", func() {
		tmpFile, err := os.CreateTemp("", "regularFile")
		defer os.Remove(tmpFile.Name())
		Expect(err).ToNot(HaveOccurred())

		err = tmpFile.Close()
		Expect(err).ToNot(HaveOccurred())

		symlinkPath := tmpFile.Name() + "-symlink"
		err = os.Symlink(tmpFile.Name(), symlinkPath)
		defer os.Remove(symlinkPath)
		Expect(err).ToNot(HaveOccurred())

		f, err := OpenNoLinks(symlinkPath)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("too many levels of symbolic links"))
		Expect(f).To(BeNil())
	})

	var _ = It("will return error if a path is a hard link", func() {
		tmpFile, err := os.CreateTemp("", "regularFile")
		defer os.Remove(tmpFile.Name())
		Expect(err).ToNot(HaveOccurred())

		err = tmpFile.Close()
		Expect(err).ToNot(HaveOccurred())

		hardlinkPath := tmpFile.Name() + "-hardlink"
		err = os.Link(tmpFile.Name(), hardlinkPath)
		defer os.Remove(hardlinkPath)
		Expect(err).ToNot(HaveOccurred())

		f, err := OpenNoLinks(hardlinkPath)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(hardlinkPath + " is a hardlink"))
		Expect(f).To(BeNil())
	})
})

func createTarArchive(tarPath string, pathToArchiveDirectory string, dirToBeArchived string) {
	tarFile, err := os.Create(tarPath)
	Expect(err).ToNot(HaveOccurred())
	defer tarFile.Close()

	gzipWriter := gzip.NewWriter(tarFile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	_ = filepath.WalkDir(filepath.Join(pathToArchiveDirectory, dirToBeArchived), func(path string, d fs.DirEntry, err error) error {
		Expect(err).ToNot(HaveOccurred())
		fileInfo, err := d.Info()
		Expect(err).ToNot(HaveOccurred())

		header, err := tar.FileInfoHeader(fileInfo, fileInfo.Name())
		Expect(err).ToNot(HaveOccurred())
		header.Name = strings.TrimPrefix(
			strings.Replace(path, pathToArchiveDirectory, "", -1),
			string(filepath.Separator),
		)

		Expect(tarWriter.WriteHeader(header)).ToNot(HaveOccurred())
		if d.Type().IsRegular() {
			bytes, err := os.ReadFile(path)
			Expect(err).ToNot(HaveOccurred())
			_, err = tarWriter.Write(bytes)
			Expect(err).ToNot(HaveOccurred())
		}
		return nil
	})
}
