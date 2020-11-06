// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"encoding/xml"
	"io/ioutil"
	"os"
)

type Instance struct {
	XMLName   xml.Name     `xml:"Instance"`
	Vendor    string       `xml:"vendor,attr"`
	Device    string       `xml:"device,attr"`
	Subdevice string       `xml:"subdevicedevice,attr"`
	Subvendor string       `xml:"subvendor,attr"`
	Bus       string       `xml:"bus,attr"`
	Dev       string       `xml:"dev,attr"`
	Func      string       `xml:"func,attr"`
	PBA       string       `xml:"PBA,attr"`
	PortID    string       `xml:"port_id,attr"`
	Display   string       `xml:"display,attr"`
	Modules   []Module     `xml:"Module"`
	VPDs      []VPD        `xml:"VPD"`
	MACAddr   MACAddresses `xml:"MACAddresses"`
}

type Module struct {
	XMLName xml.Name     `xml:"Module"`
	Type    string       `xml:"type,attr"`
	Version string       `xml:"version,attr"`
	Status  ModuleStatus `xml:"Status"`
}

type ModuleStatus struct {
	XMLName xml.Name `xml:"Status"`
	Result  string   `xml:"result,attr"`
}

type VPD struct {
	XMLName xml.Name `xml:"VPD"`
	Type    string   `xml:"type,attr"`
	Key     string   `xml:"key,attr"`
}

type MAC struct {
	XMLName xml.Name `xml:"MAC"`
	Address string   `xml:"address,attr"`
}

type SAN struct {
	XMLName xml.Name `xml:"SAN"`
	Address string   `xml:"address,attr"`
}

type MACAddresses struct {
	XMLName xml.Name `xml:"MACAddresses"`
	Mac     MAC      `xml:"MAC"`
	San     SAN      `xml:"SAN"`
}

type DeviceUpdate struct {
	XMLName             xml.Name   `xml:"DeviceUpdate"`
	Instance            []Instance `xml:"Instance"`
	PowerCycleRequired  int        `xml:"PowerCycleRequired"`
	NextUpdateAvailable int        `xml:"NextUpdateAvailable"`
	RebootRequired      int        `xml:"RebootRequired"`
}

func getDeviceUpdateFromFile(path string) (*DeviceUpdate, error) {
	invf, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer invf.Close()
	b, _ := ioutil.ReadAll(invf)

	u := &DeviceUpdate{}
	err = xml.Unmarshal(b, u)
	if err != nil {
		return nil, err
	}
	return u, nil
}
