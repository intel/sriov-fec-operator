// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

const (
	bmcOutput = `Board Management Controller, MAX10 NIOS FW version D.2.0.12
Board Management Controller, MAX10 Build version D.2.0.5
//****** BMC SENSORS ******//
Object Id                     : 0xEF00000
PCIe s:b:d.f                  : 0000:1b:00.0
Device Id                     : 0x0b30
Numa Node                     : 0
Ports Num                     : 01
Bitstream Id                  : 0x21000000000000
Bitstream Version             : 1.0.0
Pr Interface Id               : 12345678-abcd-efgh-ijkl-0123456789ab
( 1) Board Power              : 69.24 Watts
( 2) 12V Backplane Current    : 2.75 Amps
( 3) 12V Backplane Voltage    : 12.06 Volts
( 4) 1.2V Voltage             : 1.19 Volts
( 6) 1.8V Voltage             : 1.80 Volts
( 8) 3.3V Voltage             : 3.26 Volts
(10) FPGA Core Voltage        : 0.90 Volts
(11) FPGA Core Current        : 20.99 Amps
(12) FPGA Die Temperature     : 61.50 Celsius
(13) Board Temperature        : 30.00 Celsius
(14) QSFP0 Supply Voltage     : N/A
(15) QSFP0 Temperature        : N/A
(24) 12V AUX Current          : 3.10 Amps
(25) 12V AUX Voltage          : 11.64 Volts
(37) QSFP1 Supply Voltage     : N/A
(38) QSFP1 Temperature        : N/A
(44) PKVL0 Core Temperature   : 56.50 Celsius
(45) PKVL0 SerDes Temperature : 57.00 Celsius
(46) PKVL1 Core Temperature   : 57.00 Celsius
(47) PKVL1 SerDes Temperature : 57.50 Celsius
Board Management Controller, MAX10 NIOS FW version D.2.0.12
Board Management Controller, MAX10 Build version D.2.0.5
//****** BMC SENSORS ******//
Object Id                     : 0xEF00000
PCIe s:b:d.f                  : 0000:2b:00.0
Device Id                     : 0x0b30
Numa Node                     : 1
Ports Num                     : 01
Bitstream Id                  : 0x32000000000000
Bitstream Version             : 2.0.0
Pr Interface Id               : 87654321-abcd-efgh-ijkl-0123456789ab
( 1) Board Power              : 70.25 Watts
( 2) 12V Backplane Current    : 2.79 Amps
( 3) 12V Backplane Voltage    : 12.06 Volts
( 4) 1.2V Voltage             : 1.19 Volts
( 6) 1.8V Voltage             : 1.80 Volts
( 8) 3.3V Voltage             : 3.26 Volts
(10) FPGA Core Voltage        : 0.90 Volts
(11) FPGA Core Current        : 21.19 Amps
(12) FPGA Die Temperature     : 63.00 Celsius
(13) Board Temperature        : 31.00 Celsius
(14) QSFP0 Supply Voltage     : N/A
(15) QSFP0 Temperature        : N/A
(24) 12V AUX Current          : 3.14 Amps
(25) 12V AUX Voltage          : 11.64 Volts
(37) QSFP1 Supply Voltage     : N/A
(38) QSFP1 Temperature        : N/A
(44) PKVL0 Core Temperature   : 58.00 Celsius
(45) PKVL0 SerDes Temperature : 58.00 Celsius
(46) PKVL1 Core Temperature   : 58.50 Celsius
(47) PKVL1 SerDes Temperature : 59.00 Celsius
`

	bmcExpect = `# HELP fpgainfo_bmc_info fpgainfo qualitative metrics such as Bitstream ID, Device ID, etc.
# TYPE fpgainfo_bmc_info gauge
fpgainfo_bmc_info{bitstream_id="0x21000000000000",bitstream_version="1.0.0",device_id="0x0b30",numa_node="0",object_id="0xEF00000",pci="0000:1b:00.0",ports_num="01",pr_interface_id="12345678-abcd-efgh-ijkl-0123456789ab"} 1
fpgainfo_bmc_info{bitstream_id="0x32000000000000",bitstream_version="2.0.0",device_id="0x0b30",numa_node="1",object_id="0xEF00000",pci="0000:2b:00.0",ports_num="01",pr_interface_id="87654321-abcd-efgh-ijkl-0123456789ab"} 1
# HELP fpgainfo_current_amps fpgainfo current metrics
# TYPE fpgainfo_current_amps gauge
fpgainfo_current_amps{component="12v_aux",pci="0000:1b:00.0"} 3.1
fpgainfo_current_amps{component="12v_aux",pci="0000:2b:00.0"} 3.14
fpgainfo_current_amps{component="12v_backplane",pci="0000:1b:00.0"} 2.75
fpgainfo_current_amps{component="12v_backplane",pci="0000:2b:00.0"} 2.79
fpgainfo_current_amps{component="fpga_core",pci="0000:1b:00.0"} 20.99
fpgainfo_current_amps{component="fpga_core",pci="0000:2b:00.0"} 21.19
# HELP fpgainfo_power_watts fpgainfo power metrics
# TYPE fpgainfo_power_watts gauge
fpgainfo_power_watts{component="board",pci="0000:1b:00.0"} 69.24
fpgainfo_power_watts{component="board",pci="0000:2b:00.0"} 70.25
# HELP fpgainfo_temperature_celsius fpgainfo temperature metrics. 'N/A' are converted into -1.
# TYPE fpgainfo_temperature_celsius gauge
fpgainfo_temperature_celsius{component="board",pci="0000:1b:00.0"} 30
fpgainfo_temperature_celsius{component="board",pci="0000:2b:00.0"} 31
fpgainfo_temperature_celsius{component="fpga_die",pci="0000:1b:00.0"} 61.5
fpgainfo_temperature_celsius{component="fpga_die",pci="0000:2b:00.0"} 63
fpgainfo_temperature_celsius{component="pkvl0_core",pci="0000:1b:00.0"} 56.5
fpgainfo_temperature_celsius{component="pkvl0_core",pci="0000:2b:00.0"} 58
fpgainfo_temperature_celsius{component="pkvl0_serdes",pci="0000:1b:00.0"} 57
fpgainfo_temperature_celsius{component="pkvl0_serdes",pci="0000:2b:00.0"} 58
fpgainfo_temperature_celsius{component="pkvl1_core",pci="0000:1b:00.0"} 57
fpgainfo_temperature_celsius{component="pkvl1_core",pci="0000:2b:00.0"} 58.5
fpgainfo_temperature_celsius{component="pkvl1_serdes",pci="0000:1b:00.0"} 57.5
fpgainfo_temperature_celsius{component="pkvl1_serdes",pci="0000:2b:00.0"} 59
fpgainfo_temperature_celsius{component="qsfp0",pci="0000:1b:00.0"} -1
fpgainfo_temperature_celsius{component="qsfp0",pci="0000:2b:00.0"} -1
fpgainfo_temperature_celsius{component="qsfp1",pci="0000:1b:00.0"} -1
fpgainfo_temperature_celsius{component="qsfp1",pci="0000:2b:00.0"} -1
# HELP fpgainfo_voltage_volts fpgainfo voltage metrics. dots (.) are substituted with underscore, e.g. 1_2v (1.2V) and 12v_aux (12V Aux).
# TYPE fpgainfo_voltage_volts gauge
fpgainfo_voltage_volts{component="12v_aux",pci="0000:1b:00.0"} 11.64
fpgainfo_voltage_volts{component="12v_aux",pci="0000:2b:00.0"} 11.64
fpgainfo_voltage_volts{component="12v_backplane",pci="0000:1b:00.0"} 12.06
fpgainfo_voltage_volts{component="12v_backplane",pci="0000:2b:00.0"} 12.06
fpgainfo_voltage_volts{component="1_2v",pci="0000:1b:00.0"} 1.19
fpgainfo_voltage_volts{component="1_2v",pci="0000:2b:00.0"} 1.19
fpgainfo_voltage_volts{component="1_8v",pci="0000:1b:00.0"} 1.8
fpgainfo_voltage_volts{component="1_8v",pci="0000:2b:00.0"} 1.8
fpgainfo_voltage_volts{component="3_3v",pci="0000:1b:00.0"} 3.26
fpgainfo_voltage_volts{component="3_3v",pci="0000:2b:00.0"} 3.26
fpgainfo_voltage_volts{component="fpga_core",pci="0000:1b:00.0"} 0.9
fpgainfo_voltage_volts{component="fpga_core",pci="0000:2b:00.0"} 0.9
fpgainfo_voltage_volts{component="qsfp0_supply",pci="0000:1b:00.0"} -1
fpgainfo_voltage_volts{component="qsfp0_supply",pci="0000:2b:00.0"} -1
fpgainfo_voltage_volts{component="qsfp1_supply",pci="0000:1b:00.0"} -1
fpgainfo_voltage_volts{component="qsfp1_supply",pci="0000:2b:00.0"} -1
`
)

func fakeFpgaInfo(cmd string) (string, error) {
	if cmd == "bmc" {
		return bmcOutput, nil
	}
	return "", fmt.Errorf("Unsupported command: %s", cmd)
}

func TestBMCInfoCollection(t *testing.T) {
	fpgaInfoExec = fakeFpgaInfo
	collector := NewFpgaInfoCollector()

	if err := testutil.CollectAndCompare(collector, strings.NewReader(bmcExpect)); err != nil {
		t.Fatal("Unexpected metrics returned:", err)
	}
}
