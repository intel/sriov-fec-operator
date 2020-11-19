// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package main

import (
	"fmt"
	"github.com/jaypipes/ghw"
	"github.com/jaypipes/pcidb"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"
	"testing"
)

func TestMain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Main suite")
}

var _ = Describe("Labeler", func() {
	var _ = Describe("loadConfig", func() {
		var _ = It("will fail if the file does not exist", func() {
			cfg, err := loadConfig("notExistingFile.json")
			Expect(err).To(HaveOccurred())
			Expect(cfg).To(Equal(AcceleratorDiscoveryConfig{}))
		})
		var _ = It("will fail if the file is not json", func() {
			cfg, err := loadConfig("testdata/invalid.json")
			Expect(err).To(HaveOccurred())
			Expect(cfg).To(Equal(AcceleratorDiscoveryConfig{}))
		})
		var _ = It("will load the valid config successfully", func() {
			cfg, err := loadConfig("testdata/valid.json")
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).To(Equal(AcceleratorDiscoveryConfig{
				VendorID:  "0000",
				Class:     "00",
				SubClass:  "00",
				Devices:   map[string]string{"test": "test"},
				NodeLabel: "LABEL",
			}))
		})
	})
	var _ = Describe("getPCIDevices", func() {
		var _ = It("return PCI devices", func() {
			devices, err := getPCIDevices()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(devices)).ToNot(Equal(0))
		})
	})
	var _ = Describe("findAccelerator", func() {
		var _ = It("will fail if config is not provided", func() {
			found, err := findAccelerator(nil)
			Expect(err).To(HaveOccurred())
			Expect(found).To(Equal(false))
		})

		var _ = It("will fail if getPCIDevices fails", func() {
			getPCIDevices = func() ([]*ghw.PCIDevice, error) { return nil, fmt.Errorf("ErrorStub") }

			cfg, err := loadConfig("testdata/valid.json")
			Expect(err).ToNot(HaveOccurred())

			found, err := findAccelerator(&cfg)
			Expect(err).To(HaveOccurred())
			Expect(found).To(Equal(false))
		})

		var _ = It("will return false if there is no devices found", func() {
			getPCIDevices = func() ([]*ghw.PCIDevice, error) {
				return []*ghw.PCIDevice{}, nil
			}

			cfg, err := loadConfig("testdata/valid.json")
			Expect(err).ToNot(HaveOccurred())

			found, err := findAccelerator(&cfg)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(Equal(false))
		})

		var _ = It("will return true if there is a device found", func() {
			getPCIDevices = func() ([]*ghw.PCIDevice, error) {
				var devices []*ghw.PCIDevice
				devices = append(devices,
					&ghw.PCIDevice{
						Vendor: &pcidb.Vendor{
							ID: "0000",
						},
						Class: &pcidb.Class{
							ID: "00",
						},
						Subclass: &pcidb.Subclass{
							ID: "02",
						},
						Product: &pcidb.Product{
							ID: "test",
						},
					},
					&ghw.PCIDevice{
						Vendor: &pcidb.Vendor{
							ID: "0000",
						},
						Class: &pcidb.Class{
							ID: "00",
						},
						Subclass: &pcidb.Subclass{
							ID: "00",
						},
						Product: &pcidb.Product{
							ID: "test",
						},
					},
				)
				return devices, nil
			}

			cfg, err := loadConfig("testdata/valid.json")
			Expect(err).ToNot(HaveOccurred())

			found, err := findAccelerator(&cfg)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(Equal(true))
		})
	})
	var _ = Describe("setNodeLabel", func() {
		var _ = It("will fail if there is no cluster", func() {
			err := setNodeLabel("", "", false)
			Expect(err).To(HaveOccurred())
		})
	})
	var _ = Describe("acceleratorDiscovery", func() {
		var _ = It("will fail if load config fails", func() {
			err := acceleratorDiscovery("")
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will fail if findAccelerator fails", func() {
			getPCIDevices = func() ([]*ghw.PCIDevice, error) { return nil, fmt.Errorf("ErrorStub") }
			err := acceleratorDiscovery("testdata/valid.json")
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will fail if there is no NODENAME env", func() {
			getPCIDevices = func() ([]*ghw.PCIDevice, error) { return []*ghw.PCIDevice{}, nil }
			err := acceleratorDiscovery("testdata/valid.json")
			Expect(err).To(HaveOccurred())
		})
		var _ = It("will fail if there is no k8s cluster", func() {
			getPCIDevices = func() ([]*ghw.PCIDevice, error) { return []*ghw.PCIDevice{}, nil }
			os.Setenv("NODENAME", "test")
			err := acceleratorDiscovery("testdata/valid.json")
			os.Unsetenv("NODENAME")
			Expect(err).To(HaveOccurred())
		})
	})
})
