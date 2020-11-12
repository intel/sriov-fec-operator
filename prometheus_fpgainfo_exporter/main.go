// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020 Intel Corporation

package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	bmcFloatRegex       = regexp.MustCompile(`^\(\s*[0-9]*\)\s+(.+?)(?:\s*:\s*)(.+)$`)
	bmcQualitativeRegex = regexp.MustCompile(`^([a-zA-Z .:]+?)(?:\s*:\s)(.+)$`)
	pciRegex            = regexp.MustCompile(`[a-fA-F0-9]{4}:[a-fA-F0-9]{2}:[a-fA-F0-9]{2}\.[0-9]`)

	fpgaInfoPath = "fpgainfo"
	address      = ":42222"

	fpgaInfoExec = func(command string) (string, error) {
		cmd := exec.Command(fpgaInfoPath, command)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("`%s %s` executed unsuccessfully. Output:'%s', Error: %+v",
				fpgaInfoPath, command, string(output), err)
			return "", err
		}

		return string(output), nil
	}
)

// DeviceBMCInfo contains various metrics obtained from `fpgainfo bmc` grouped by unit families.
type DeviceBMCInfo struct {
	// Qualitative stores textual values such as "Object ID", "Bitstream ID", etc.
	Qualitative map[string]string
	// Temperatures stores values (Celsius) for accelerator components, e.g. Board, FPGA Die, PKVL, etc.
	Temperatures map[string]float64
	// Voltages stores values (Volts) for accelerator components, e.g. 12V Aux, 12V Backplane, etc.
	Voltages map[string]float64
	// Currents stores values (Amps) for accelerator components, e.g. 12V Aux, 12V Backplane, etc.
	Currents map[string]float64
	// Powers stores values (Watts) for accelerator components, e.g. Board
	Powers map[string]float64
}

func sanitizeLabel(in string) string {
	r := strings.NewReplacer(" ", "_", ":", "_", ".", "_")
	return r.Replace(strings.ToLower(in))
}

func extractUnit(in string) (string, string) {
	split := strings.Split(in, " ")
	value := strings.Join(split[:len(split)-1], " ")
	unit := split[len(split)-1]

	return value, unit
}

func extractFloatValue(in string) (float64, error) {
	if in == "N/A" {
		return -1, nil
	}
	val, _ := extractUnit(in)
	return strconv.ParseFloat(val, 64)
}

// NewDeviceBMCInfo parses given string and returns a DeviceBMCInfo.
// Given `output` string must contain data only for one device.
func NewDeviceBMCInfo(output string) *DeviceBMCInfo {
	devBMC := &DeviceBMCInfo{
		Qualitative:  map[string]string{},
		Temperatures: map[string]float64{},
		Voltages:     map[string]float64{},
		Currents:     map[string]float64{},
		Powers:       map[string]float64{},
	}

	for _, line := range strings.Split(output, "\n") {
		textMatches := bmcQualitativeRegex.FindStringSubmatch(line)
		if len(textMatches) == 3 {
			label := sanitizeLabel(textMatches[1])
			value := textMatches[2]

			if !strings.Contains(label, "pci") {
				devBMC.Qualitative[label] = value
			}
		}

		floatMatches := bmcFloatRegex.FindStringSubmatch(line)
		if len(floatMatches) == 3 {
			name, unitFamily := extractUnit(floatMatches[1])

			label := sanitizeLabel(name)
			floatVal, err := extractFloatValue(floatMatches[2])
			if err != nil {
				log.Printf("Failed to convert string (%s) to float. Error: %+v", floatMatches[2], err)
				continue
			}

			if unitFamily == "Power" {
				devBMC.Powers[label] = floatVal
			} else if unitFamily == "Voltage" {
				devBMC.Voltages[label] = floatVal
			} else if unitFamily == "Current" {
				devBMC.Currents[label] = floatVal
			} else if unitFamily == "Temperature" {
				devBMC.Temperatures[label] = floatVal
			}
		}
	}

	return devBMC
}

// PCIAddress is a type definition for PCI
type PCIAddress string

// BMCInfo stores Board Management Information for each device identified by PCI
type BMCInfo struct {
	Devices map[PCIAddress]*DeviceBMCInfo
}

// GetBMCInfo obtains and parses output of `fpgainfo bmc` command
func GetBMCInfo() (*BMCInfo, error) {
	fpgaInfoBMCOutput, err := fpgaInfoExec("bmc")
	if err != nil {
		log.Printf("GetBMCInfo(): Failed to get output from fpgainfo: %+v", err)
		return nil, err
	}

	bmcInfo := &BMCInfo{
		Devices: map[PCIAddress]*DeviceBMCInfo{},
	}

	for _, deviceBMCOutput := range strings.Split(fpgaInfoBMCOutput, "Board Management Controller") {
		pciMatches := pciRegex.FindStringSubmatch(deviceBMCOutput)
		if len(pciMatches) == 0 {
			continue
		}
		pci := PCIAddress(pciMatches[0])

		bmcInfo.Devices[pci] = NewDeviceBMCInfo(deviceBMCOutput)
	}

	return bmcInfo, nil
}

// FpgaInfoCollector implements prometheus.Collector interface for FpgaInfo utility
type FpgaInfoCollector struct {
	temperatureDesc   *prometheus.Desc
	powerDesc         *prometheus.Desc
	currentDesc       *prometheus.Desc
	voltageDesc       *prometheus.Desc
	qualitativeDesc   *prometheus.Desc
	qualitativeLabels []string
}

// NewFpgaInfoCollector creates new Collector for FpgaInfo metrics
func NewFpgaInfoCollector() *FpgaInfoCollector {
	bmc, err := GetBMCInfo()
	if err != nil {
		log.Printf("Creating new collector - could not obtain BMC Info: %+v", err)
		return nil
	}

	// Create a list of qualitative values.
	// Because the list is dynamic (built on start of the exporter)
	// we need to persist it, so when Prometheus asks for metrics
	// we'll be able to give them in the same order
	bmcLabels := []string{"pci"}
	for _, device := range bmc.Devices {
		for bmcLabel := range device.Qualitative {
			bmcLabels = append(bmcLabels, bmcLabel)
		}
		break
	}

	return &FpgaInfoCollector{
		qualitativeLabels: bmcLabels,
		temperatureDesc: prometheus.NewDesc(
			"fpgainfo_temperature_celsius",
			"fpgainfo temperature metrics. 'N/A' are converted into -1.",
			[]string{"pci", "component"},
			nil,
		),
		powerDesc: prometheus.NewDesc(
			"fpgainfo_power_watts",
			"fpgainfo power metrics",
			[]string{"pci", "component"},
			nil,
		),

		currentDesc: prometheus.NewDesc(
			"fpgainfo_current_amps",
			"fpgainfo current metrics",
			[]string{"pci", "component"},
			nil,
		),
		voltageDesc: prometheus.NewDesc(
			"fpgainfo_voltage_volts",
			"fpgainfo voltage metrics. "+
				"dots (.) are substituted with underscore, e.g. 1_2v (1.2V) and 12v_aux (12V Aux).",
			[]string{"pci", "component"},
			nil,
		),
		qualitativeDesc: prometheus.NewDesc(
			"fpgainfo_bmc_info",
			"fpgainfo qualitative metrics such as Bitstream ID, Device ID, etc.",
			bmcLabels,
			nil,
		),
	}
}

// Describe feeds Prometheus' channel with Descriptions
func (c *FpgaInfoCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.temperatureDesc
	ch <- c.powerDesc
	ch <- c.currentDesc
	ch <- c.voltageDesc
	ch <- c.qualitativeDesc
}

func (c *FpgaInfoCollector) sendMetricToChannel(ch chan<- prometheus.Metric, desc *prometheus.Desc,
	value float64, labelValues []string) {
	ch <- prometheus.MustNewConstMetric(
		desc,
		prometheus.GaugeValue,
		value,
		labelValues...,
	)
}

// Collect obtains data from the FpgaInfo and feeds into Prometheus' channel with Metrics
func (c *FpgaInfoCollector) Collect(ch chan<- prometheus.Metric) {
	bmcInfo, err := GetBMCInfo()
	if err != nil {
		log.Printf("Collect(): could not obtain BMC Info: %+v", err)
		return
	}

	for pci, device := range bmcInfo.Devices {

		qLabelValues := []string{}
		for _, storedLabel := range c.qualitativeLabels {
			if storedLabel == "pci" {
				qLabelValues = append(qLabelValues, string(pci))
			} else {
				if value, ok := device.Qualitative[storedLabel]; ok {
					qLabelValues = append(qLabelValues, string(value))
				} else {
					log.Printf("Collect(): Failed to find a value for a label: %s", storedLabel)
				}
			}
		}

		c.sendMetricToChannel(ch, c.qualitativeDesc, 1, qLabelValues)

		for name, value := range device.Temperatures {
			c.sendMetricToChannel(ch, c.temperatureDesc, value, []string{string(pci), name})
		}
		for name, value := range device.Voltages {
			c.sendMetricToChannel(ch, c.voltageDesc, value, []string{string(pci), name})
		}
		for name, value := range device.Currents {
			c.sendMetricToChannel(ch, c.currentDesc, value, []string{string(pci), name})
		}
		for name, value := range device.Powers {
			c.sendMetricToChannel(ch, c.powerDesc, value, []string{string(pci), name})
		}
	}
}

func init() {
	flag.StringVar(&fpgaInfoPath, "fpgainfo", "fpgainfo", "Path to fpgainfo utility")
	flag.StringVar(&address, "address", ":42222", "Address on which metrics are exposed")
}

func main() {
	flag.Parse()

	_, err := fpgaInfoExec("bmc")
	if err != nil {
		log.Fatalf("fpgainfo bmc failed to run: %+v. Exporter will exit", err)
	}

	log.Printf("fpgaInfoPath: %s", fpgaInfoPath)

	prometheus.MustRegister(NewFpgaInfoCollector())

	log.Printf("Listening on address: %s", address)
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Printf("ListenAndServer failed: %+v", err)
		os.Exit(1)
	}
}
