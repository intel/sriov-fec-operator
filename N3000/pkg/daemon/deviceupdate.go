// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package daemon

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"strconv"
	"time"
)

var (
	maxFileSize           = int64(10)              // Maximum update file size in kilobytes
	updateXMLParseTimeout = 100 * time.Millisecond // Update xml parse timeout
)

type module struct {
	Type    string       `xml:"type,attr"`
	Version string       `xml:"version,attr"`
	Status  moduleStatus `xml:"Status"`
}

type moduleStatus struct {
	Result string `xml:"result,attr"`
}

type deviceUpdateInstance struct {
	Modules             []module
	NextUpdateAvailable int
}

func getDeviceUpdateFromFile(path string) (*deviceUpdateInstance, error) {
	invf, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer invf.Close()

	stat, err := invf.Stat()
	if err != nil {
		return nil, err
	}

	kSize := stat.Size() / 1024
	if kSize > maxFileSize {
		return nil, errors.New("Update status xml file too large: " + strconv.Itoa(int(kSize)) + "kB")
	}

	ctx, cancel := context.WithTimeout(context.Background(), updateXMLParseTimeout)
	defer cancel()

	u := &deviceUpdateInstance{}
	decoder := xml.NewDecoder(invf)
	for {
		select {
		case <-ctx.Done():
			cancel()
			return nil, ctx.Err()
		default:
			token, err := decoder.Token()
			if token == nil {
				return u, nil
			}
			if err != nil {
				if err == io.EOF {
					return u, nil
				}
				return nil, err
			}

			switch t := token.(type) {
			case xml.StartElement:
				if t.Name.Local == "Module" {
					var m module
					err := decoder.DecodeElement(&m, &t)
					if err != nil {
						return nil, err
					}
					u.Modules = append(u.Modules, m)
				}

				if t.Name.Local == "NextUpdateAvailable" {
					var nua int
					err := decoder.DecodeElement(&nua, &t)
					if err != nil {
						return nil, err
					}
					u.NextUpdateAvailable = nua
				}
			}
		}
	}
}
