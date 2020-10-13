// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import "encoding/xml"

type DeviceInventory struct {
	XMLName       xml.Name   `xml:"DeviceInventory"`
	InventoryList []Instance `xml:"Instance"`
}

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
	XMLName xml.Name `xml:"Module"`
	Type    string   `xml:"type,attr"`
	Version string   `xml:"version,attr"`
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
