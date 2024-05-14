// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2024 Intel Corporation

package daemon

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	sriovv2 "github.com/intel/sriov-fec-operator/api/sriovfec/v2"
	vrbv1 "github.com/intel/sriov-fec-operator/api/sriovvrb/v1"
	"gopkg.in/ini.v1"
)

func generateBBDevConfigFile(bbDevConfig sriovv2.BBDevConfig, file string) (err error) {

	if err = bbDevConfig.Validate(); err != nil {
		return err
	}

	var iniFile *ini.File

	switch {
	case bbDevConfig.ACC100 != nil:
		if iniFile, err = createIniFileContent(acc100BBDevConfigToIniStruct, bbDevConfig.ACC100); err != nil {
			return fmt.Errorf("creation of pf_bb_config config file for ACC100 failed, %s", err)
		}
	case bbDevConfig.ACC200 != nil:
		if iniFile, err = createIniFileContent(acc200BBDevConfigToIniStruct, bbDevConfig.ACC200); err != nil {
			return fmt.Errorf("creation of pf_bb_config config file for ACC200 failed, %s", err)
		}
	case bbDevConfig.N3000 != nil:
		if iniFile, err = createIniFileContent(n3000BBDevConfigToIniStruct, bbDevConfig.N3000); err != nil {
			return fmt.Errorf("creation of pf_bb_config config file for N3000 failed, %s", err)
		}
	default:
		return fmt.Errorf("received BBDevConfig is empty")
	}

	if err := logIniFile(iniFile); err != nil {
		return err
	}

	if err := iniFile.SaveTo(file); err != nil {
		return fmt.Errorf("unable to write config to file: %s", file)
	}

	return nil
}

func generateVrbBBDevConfigFile(bbDevConfig vrbv1.BBDevConfig, file string) (err error) {

	if err = bbDevConfig.Validate(); err != nil {
		return err
	}

	var iniFile *ini.File

	switch {
	case bbDevConfig.VRB1 != nil:
		if iniFile, err = createIniFileContent(vrb1BBDevConfigToIniStruct, bbDevConfig.VRB1); err != nil {
			return fmt.Errorf("creation of pf_bb_config config file for VRB1 failed, %s", err)
		}
	case bbDevConfig.VRB2 != nil:
		if iniFile, err = createIniFileContent(vrb2BBDevConfigToIniStruct, bbDevConfig.VRB2); err != nil {
			return fmt.Errorf("creation of pf_bb_config config file for VRB2 failed, %s", err)
		}
	default:
		return fmt.Errorf("received BBDevConfig is empty")
	}

	if err := logIniFile(iniFile); err != nil {
		return err
	}

	if err := iniFile.SaveTo(file); err != nil {
		return fmt.Errorf("unable to write config to file: %s", file)
	}

	return nil
}

type bbDeviceConfig interface {
	*sriovv2.ACC100BBDevConfig | *sriovv2.ACC200BBDevConfig | *sriovv2.N3000BBDevConfig | *vrbv1.ACC100BBDevConfig | *vrbv1.VRB1BBDevConfig | *vrbv1.VRB2BBDevConfig
}

func createIniFileContent[BB bbDeviceConfig, C func(BB) interface{}](convertToIniStruct C, bbDevConfig BB) (*ini.File, error) {
	if bbDevConfig == nil {
		return nil, errors.New("received nil pf_bb_config")
	}

	iniFile := ini.Empty()
	if err := iniFile.ReflectFrom(convertToIniStruct(bbDevConfig)); err != nil {
		return nil, fmt.Errorf("creation of pf_bb_config config file for ACC200/VRB1 failed, %s", err)
	}

	return iniFile, nil
}

func logIniFile(cfg *ini.File) error {
	var b bytes.Buffer
	writer := io.Writer(&b)
	_, err := cfg.WriteTo(writer)
	if err != nil {
		return fmt.Errorf("unable to write config to log writer, %s", err)
	}
	log.WithField("generated BBDevConfig", b.String()).Info("logIniFile")
	return nil
}

type queueGroupConfigIniWrapper struct {
	NumQueueGroups  int `ini:"num_qgroups"`
	NumAqsPerGroups int `ini:"num_aqs_per_groups"`
	AqDepthLog2     int `ini:"aq_depth_log2"`
}

func queueGroupConfigToIniStruct(in sriovv2.QueueGroupConfig) queueGroupConfigIniWrapper {
	return queueGroupConfigIniWrapper{
		NumQueueGroups:  in.NumQueueGroups,
		NumAqsPerGroups: in.NumAqsPerGroups,
		AqDepthLog2:     in.AqDepthLog2,
	}
}

func VrbqueueGroupConfigToIniStruct(in vrbv1.QueueGroupConfig) queueGroupConfigIniWrapper {
	return queueGroupConfigIniWrapper{
		NumQueueGroups:  in.NumQueueGroups,
		NumAqsPerGroups: in.NumAqsPerGroups,
		AqDepthLog2:     in.AqDepthLog2,
	}
}

type acc100BBDevConfigIniWrapper struct {
	PFMode struct {
		PFMode string `ini:"pf_mode_en"`
	} `ini:"MODE"`

	NumVfBundles struct {
		NumVfBundles int `ini:"num_vf_bundles"`
	} `ini:"VFBUNDLES"`

	MaxQueueSize struct {
		MaxQueueSize int `ini:"max_queue_size"`
	} `ini:"MAXQSIZE"`

	Uplink4G   queueGroupConfigIniWrapper `ini:"QUL4G"`
	Downlink4G queueGroupConfigIniWrapper `ini:"QDL4G"`
	Uplink5G   queueGroupConfigIniWrapper `ini:"QUL5G"`
	Downlink5G queueGroupConfigIniWrapper `ini:"QDL5G"`
}

func acc100BBDevConfigToIniStruct(in *sriovv2.ACC100BBDevConfig) interface{} {
	return &acc100BBDevConfigIniWrapper{
		PFMode: struct {
			PFMode string `ini:"pf_mode_en"`
		}{
			boolToIntStringMapping[in.PFMode],
		},
		NumVfBundles: struct {
			NumVfBundles int `ini:"num_vf_bundles"`
		}{
			in.NumVfBundles,
		},
		MaxQueueSize: struct {
			MaxQueueSize int `ini:"max_queue_size"`
		}{
			in.MaxQueueSize,
		},
		Uplink5G:   queueGroupConfigToIniStruct(in.Uplink5G),
		Uplink4G:   queueGroupConfigToIniStruct(in.Uplink4G),
		Downlink5G: queueGroupConfigToIniStruct(in.Downlink5G),
		Downlink4G: queueGroupConfigToIniStruct(in.Downlink4G),
	}
}

func Vrbacc100BBDevConfigToIniStruct(in *vrbv1.ACC100BBDevConfig) interface{} {
	return &acc100BBDevConfigIniWrapper{
		PFMode: struct {
			PFMode string `ini:"pf_mode_en"`
		}{
			boolToIntStringMapping[in.PFMode],
		},
		NumVfBundles: struct {
			NumVfBundles int `ini:"num_vf_bundles"`
		}{
			in.NumVfBundles,
		},
		MaxQueueSize: struct {
			MaxQueueSize int `ini:"max_queue_size"`
		}{
			in.MaxQueueSize,
		},
		Uplink5G:   VrbqueueGroupConfigToIniStruct(in.Uplink5G),
		Uplink4G:   VrbqueueGroupConfigToIniStruct(in.Uplink4G),
		Downlink5G: VrbqueueGroupConfigToIniStruct(in.Downlink5G),
		Downlink4G: VrbqueueGroupConfigToIniStruct(in.Downlink4G),
	}
}

type acc200BBDevConfigIniWrapper struct {
	Wrapper acc100BBDevConfigIniWrapper `ini:"DEFAULT"`
	QFFT    queueGroupConfigIniWrapper  `ini:"QFFT"`
}

type vrb1BBDevConfigIniWrapper struct {
	Wrapper acc100BBDevConfigIniWrapper `ini:"DEFAULT"`
	QFFT    queueGroupConfigIniWrapper  `ini:"QFFT"`
}

type vrb2BBDevConfigIniWrapper struct {
	Wrapper acc100BBDevConfigIniWrapper `ini:"DEFAULT"`
	QFFT    queueGroupConfigIniWrapper  `ini:"QFFT"`
	QMLD    queueGroupConfigIniWrapper  `ini:"QMLD"`
}

func acc200BBDevConfigToIniStruct(in *sriovv2.ACC200BBDevConfig) interface{} {
	delegate := acc100BBDevConfigToIniStruct(&in.ACC100BBDevConfig).(*acc100BBDevConfigIniWrapper)
	return &acc200BBDevConfigIniWrapper{
		Wrapper: *delegate,
		QFFT:    queueGroupConfigToIniStruct(in.QFFT),
	}
}

func vrb1BBDevConfigToIniStruct(in *vrbv1.VRB1BBDevConfig) interface{} {
	delegate := Vrbacc100BBDevConfigToIniStruct(&in.ACC100BBDevConfig).(*acc100BBDevConfigIniWrapper)
	return &vrb1BBDevConfigIniWrapper{
		Wrapper: *delegate,
		QFFT:    VrbqueueGroupConfigToIniStruct(in.QFFT),
	}
}

func vrb2BBDevConfigToIniStruct(in *vrbv1.VRB2BBDevConfig) interface{} {
	delegate := Vrbacc100BBDevConfigToIniStruct(&in.ACC100BBDevConfig).(*acc100BBDevConfigIniWrapper)
	return &vrb2BBDevConfigIniWrapper{
		Wrapper: *delegate,
		QFFT:    VrbqueueGroupConfigToIniStruct(in.QFFT),
		QMLD:    VrbqueueGroupConfigToIniStruct(in.QMLD),
	}
}

type n3000BBDevConfigIniWrapper struct {
	PFMode struct {
		PFMode string `ini:"pf_mode_en"`
	} `ini:"MODE"`

	Uplink struct {
		Bandwidth   int    `ini:"bandwidth"`
		LoadBalance int    `ini:"load_balance"`
		Vfqmap      string `ini:"vfqmap"`
	} `ini:"UL"`

	Downlink struct {
		Bandwidth   int    `ini:"bandwidth"`
		LoadBalance int    `ini:"load_balance"`
		Vfqmap      string `ini:"vfqmap"`
	} `ini:"DL"`

	FLR struct {
		FLR int `ini:"flr_time_out"`
	} `ini:"FLR"`
}

func n3000BBDevConfigToIniStruct(in *sriovv2.N3000BBDevConfig) interface{} {
	return &n3000BBDevConfigIniWrapper{
		PFMode: struct {
			PFMode string `ini:"pf_mode_en"`
		}{
			PFMode: AsIntString(in.PFMode),
		},
		FLR: struct {
			FLR int `ini:"flr_time_out"`
		}{
			FLR: in.FLRTimeOut,
		},

		Downlink: struct {
			Bandwidth   int    `ini:"bandwidth"`
			LoadBalance int    `ini:"load_balance"`
			Vfqmap      string `ini:"vfqmap"`
		}{
			Bandwidth:   in.Downlink.Bandwidth,
			LoadBalance: in.Downlink.LoadBalance,
			Vfqmap:      in.Downlink.Queues.String(),
		},

		Uplink: struct {
			Bandwidth   int    `ini:"bandwidth"`
			LoadBalance int    `ini:"load_balance"`
			Vfqmap      string `ini:"vfqmap"`
		}{
			Bandwidth:   in.Uplink.Bandwidth,
			LoadBalance: in.Uplink.LoadBalance,
			Vfqmap:      in.Uplink.Queues.String(),
		},
	}
}

var boolToIntStringMapping = map[bool]string{false: "0", true: "1"}

func AsIntString(v bool) string {
	return boolToIntStringMapping[v]
}
