// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2024 Intel Corporation

package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	fec "github.com/intel/sriov-fec-operator/api/sriovfec/v2"
	vrbv1 "github.com/intel/sriov-fec-operator/api/sriovvrb/v1"
	"github.com/intel/sriov-fec-operator/pkg/common/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	pciAddressLabel = "pci_address"
	queueTypeLabel  = "queue_type"
	engineIDLabel   = "engine_id"
	statusLabel     = "status"
)

type telemetryGatherer struct {
	codeBlocksGauge, bytesGauge, engineGauge, vfStatusGauge, vfCountGauge *prometheus.GaugeVec
	metricUpdates                                                         []func()
}

// VFUnion represents a union of fec.VF and vrbv1.VF
type VFUnion interface{}

func newTelemetryGatherer() *telemetryGatherer {
	t := &telemetryGatherer{}
	t.codeBlocksGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "code_blocks_per_vfs",
		Help: `number of code blocks processed by VF. 'pci_address' - represents unique BDF for VF. 'queue_type' - represents queue type for Vfs. Available values: '5GDL', '5GUL', 'FFT'`,
	}, []string{pciAddressLabel, queueTypeLabel})

	t.bytesGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bytes_processed_per_vfs",
		Help: `represents number of bytes that are processed by VF. 'pci_address' - represents unique BDF for VF. 'queue_type' - represents queue type for Vfs. Available values: '5GDL', '5GUL', 'FFT'`,
	}, []string{pciAddressLabel, queueTypeLabel})

	t.engineGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "counters_per_engine",
		Help: `number of code blocks processed by Engine. 'engine_id' - represents integer ID of engine on card. 'pci_address' - represents unique BDF for card on which engine is located. 'queue_type' - represents queue type for Vfs. Available values: '5GDL', '5GUL', 'FFT'`,
	}, []string{engineIDLabel, pciAddressLabel, queueTypeLabel})

	vfStatusGaugeHelp := `equals to 1 if 'status' is 'RTE_BBDEV_DEV_CONFIGURED' or 'RTE_BBDEV_DEV_ACTIVE' and 0 otherwise.` +
		`'pci_address' - represents unique BDF for VF.'status' - represents status as exposed by pf-bb-config.` +
		`Available values: 'RTE_BBDEV_DEV_NOSTATUS', 'RTE_BBDEV_DEV_NOT_SUPPORTED', 'RTE_BBDEV_DEV_RESET','RTE_BBDEV_DEV_CONFIGURED', 'RTE_BBDEV_DEV_ACTIVE', 'RTE_BBDEV_DEV_FATAL_ERR', 'RTE_BBDEV_DEV_RESTART_REQ', 'RTE_BBDEV_DEV_RECONFIG_REQ', 'RTE_BBDEV_DEV_CORRECT_ERR'`
	t.vfStatusGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vf_status",
		Help: vfStatusGaugeHelp,
	}, []string{pciAddressLabel, statusLabel})

	t.vfCountGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vf_count",
		Help: `describes number of configured VFs on card.'pci_address' - represents unique BDF for PF.'status' - represents current status of SriovFecNodeConfig. Available values: 'InProgress', 'Succeeded', 'Failed', 'Ignored'`,
	}, []string{pciAddressLabel, statusLabel})
	return t
}

func (t *telemetryGatherer) queueMetric(gauge *prometheus.GaugeVec, labels map[string]string, val float64) {
	t.metricUpdates = append(t.metricUpdates, func() {
		gauge.With(labels).Set(val)
	})
}

func (t *telemetryGatherer) resetMetrics() {
	t.vfCountGauge.Reset()
	t.vfStatusGauge.Reset()
	t.bytesGauge.Reset()
	t.codeBlocksGauge.Reset()
	t.engineGauge.Reset()
}

func (t *telemetryGatherer) updateMetrics() {
	t.resetMetrics()
	for _, metricUpdate := range t.metricUpdates {
		metricUpdate()
	}
	t.metricUpdates = nil
}

func (t *telemetryGatherer) updateVfStatus(pciAddr, status string, value float64) {
	t.queueMetric(t.vfStatusGauge, map[string]string{pciAddressLabel: pciAddr, statusLabel: status}, value)
}

func (t *telemetryGatherer) updateVfCount(pciAddr, status string, value float64) {
	t.queueMetric(t.vfCountGauge, map[string]string{pciAddressLabel: pciAddr, statusLabel: status}, value)
}

func (t *telemetryGatherer) updateCodeBlocks(opType, pciAddr string, value float64) {
	t.queueMetric(t.codeBlocksGauge, map[string]string{queueTypeLabel: opType, pciAddressLabel: pciAddr}, value)
}

func (t *telemetryGatherer) updateBytes(opType, pciAddr string, value float64) {
	t.queueMetric(t.bytesGauge, map[string]string{queueTypeLabel: opType, pciAddressLabel: pciAddr}, value)
}

func (t *telemetryGatherer) updateEngines(opType, engineID, pciAddr string, value float64) {
	t.queueMetric(t.engineGauge, map[string]string{queueTypeLabel: opType, engineIDLabel: engineID, pciAddressLabel: pciAddr}, value)
}

func (t *telemetryGatherer) getGauges() []*prometheus.GaugeVec {
	return []*prometheus.GaugeVec{t.codeBlocksGauge, t.bytesGauge, t.engineGauge, t.vfStatusGauge, t.vfCountGauge}
}

func StartTelemetryDaemon(mgr manager.Manager, nodeName string, ns string, directClient client.Client, log *logrus.Logger) {
	reg := prometheus.NewRegistry()
	telemetryGatherer := newTelemetryGatherer()
	for _, collector := range telemetryGatherer.getGauges() {
		reg.MustRegister(collector)
	}
	err := mgr.AddMetricsExtraHandler("/bbdevconfig", promhttp.HandlerFor(
		reg, promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		}))
	if err != nil {
		log.WithError(err).Error("cannot register handler for telemetry")
		os.Exit(1)
	}
	log.Info("registered Prometheus telemetry collectors and endpoint")
	go getMetrics(nodeName, ns, directClient, log, telemetryGatherer)
}

func getFecMetrics(log *logrus.Logger, telemetryGatherer *telemetryGatherer, fecNodeConfig *fec.SriovFecNodeConfig) {

	if len(fecNodeConfig.Status.Conditions) > 0 && fecNodeConfig.Status.Conditions[0].Reason == string(fec.SucceededSync) {
		for _, acc := range fecNodeConfig.Status.Inventory.SriovAccelerators {
			if strings.EqualFold(acc.PFDriver, utils.VfioPci) {
				getTelemetry(acc.PCIAddress, acc.VFs, telemetryGatherer, log)
			}
		}
	} else {
		for _, acc := range fecNodeConfig.Status.Inventory.SriovAccelerators {
			if strings.EqualFold(acc.PFDriver, utils.VfioPci) {
				if len(fecNodeConfig.Status.Conditions) != 0 {
					telemetryGatherer.updateVfCount(acc.PCIAddress, fecNodeConfig.Status.Conditions[0].Reason, 0)
				} else {
					telemetryGatherer.updateVfCount(acc.PCIAddress, "", 0)
				}
			}
		}
	}
}

func getVrbMetrics(log *logrus.Logger, telemetryGatherer *telemetryGatherer, vrbNodeConfig *vrbv1.SriovVrbNodeConfig) {

	if len(vrbNodeConfig.Status.Conditions) > 0 && vrbNodeConfig.Status.Conditions[0].Reason == string(vrbv1.SucceededSync) {
		for _, acc := range vrbNodeConfig.Status.Inventory.SriovAccelerators {
			if strings.EqualFold(acc.PFDriver, utils.VfioPci) {
				VrbgetTelemetry(acc.PCIAddress, acc.VFs, telemetryGatherer, log)
			}
		}
	} else {
		for _, acc := range vrbNodeConfig.Status.Inventory.SriovAccelerators {
			if strings.EqualFold(acc.PFDriver, utils.VfioPci) {
				if len(vrbNodeConfig.Status.Conditions) != 0 {
					telemetryGatherer.updateVfCount(acc.PCIAddress, vrbNodeConfig.Status.Conditions[0].Reason, 0)
				} else {
					telemetryGatherer.updateVfCount(acc.PCIAddress, "", 0)
				}
			}
		}
	}
}

func getMetrics(nodeName, namespace string, c client.Client, log *logrus.Logger, telemetryGatherer *telemetryGatherer) {
	sleepEnv := os.Getenv(utils.SriovPrefix + "METRIC_GATHER_INTERVAL")
	sleepDuration, err := time.ParseDuration(sleepEnv)
	if err != nil {
		log.WithError(err).WithField("default", sleepDuration).Error("failed to parse SRIOV_FEC_METRIC_GATHER_INTERVAL env variable, disabling metrics update loop")
		return
	} else if sleepDuration == 0 {
		log.WithField("default", sleepDuration).Info("disabling metrics update loop")
		return
	}

	utils.NewLogger().Info("metrics update loop will run every ", sleepDuration)
	wait.Forever(func() {
		fecNodeConfig := &fec.SriovFecNodeConfig{}
		vrbNodeConfig := &vrbv1.SriovVrbNodeConfig{}

		fecNodeConfigErr := c.Get(context.Background(), client.ObjectKey{Name: nodeName, Namespace: namespace}, fecNodeConfig)
		vrbNodeConfigErr := c.Get(context.Background(), client.ObjectKey{Name: nodeName, Namespace: namespace}, vrbNodeConfig)

		if fecNodeConfigErr != nil && vrbNodeConfigErr != nil {
			log.WithError(fecNodeConfigErr).WithField("nodeName", nodeName).WithField("namespace", namespace).Error("failed to get SriovFecNodeConfig to fetch telemetry")
			log.WithError(vrbNodeConfigErr).WithField("nodeName", nodeName).WithField("namespace", namespace).Error("failed to get SriovVrbNodeConfig to fetch telemetry")
			return
		}

		if fecNodeConfigErr == nil && len(fecNodeConfig.Spec.PhysicalFunctions) != 0 {
			getFecMetrics(log, telemetryGatherer, fecNodeConfig)
		}

		if vrbNodeConfigErr == nil && len(vrbNodeConfig.Spec.PhysicalFunctions) != 0 {
			getVrbMetrics(log, telemetryGatherer, vrbNodeConfig)
		}

		telemetryGatherer.updateMetrics()
	}, sleepDuration)
}

func getTelemetry(pciAddr string, vfs []fec.VF, telemetryGatherer *telemetryGatherer, log *logrus.Logger) {
	err := clearLog(pciAddr)
	if err != nil {
		log.WithError(err).WithField("pciAddr", pciAddr).Error("error occurred during preparation for telemetry loop")
		return
	}
	err = requestTelemetry(pciAddr, log)
	if err != nil {
		log.WithError(err).WithField("pciAddr", pciAddr).Error("couldn't request telemetry from socket")
		return
	}

	file, err := readFileWithTelemetry(pciAddr, log)
	if err != nil {
		log.WithError(err).Error("failed to open file with telemetry metrics, skipping telemetry loop")
		return
	}

	parseTelemetry(file, vfs, pciAddr, telemetryGatherer, log)
}

func VrbgetTelemetry(pciAddr string, vfs []vrbv1.VF, telemetryGatherer *telemetryGatherer, log *logrus.Logger) {
	err := clearLog(pciAddr)
	if err != nil {
		log.WithError(err).WithField("pciAddr", pciAddr).Error("error occurred during preparation for telemetry loop")
		return
	}
	err = requestTelemetry(pciAddr, log)
	if err != nil {
		log.WithError(err).WithField("pciAddr", pciAddr).Error("couldn't request telemetry from socket")
		return
	}

	file, err := readFileWithTelemetry(pciAddr, log)
	if err != nil {
		log.WithError(err).Error("failed to open file with telemetry metrics, skipping telemetry loop")
		return
	}

	VrbparseTelemetry(file, vfs, pciAddr, telemetryGatherer, log)
}

func parseTelemetry(file []byte, vfs []fec.VF, pciAddr string, telemetryGatherer *telemetryGatherer, logger *logrus.Logger) {
	lines := strings.Split(string(file), "\n")
	for i := range lines {
		// Fri Sep 13 10:49:25 2022:INFO:FFT counters: Per Engine
		// Tue Sep 13 10:49:25 2022:INFO:0 0
		if strings.Contains(lines[i], "counters") {
			if i+1 < len(lines) {
				parseCounters(lines[i], lines[i+1], vfs, pciAddr, telemetryGatherer, logger)
			} else {
				logger.WithField("metrics", len(lines)).WithField("pciAddr", pciAddr).Errorf("Telemetry counter value line missing.")
				return
			}
		}

		// Tue Sep 13 11:25:32 2022:INFO:Device Status:: 3 VFs
		//
		// Fri Sep 13 11:25:32 2022:INFO:-  VF 0 RTE_BBDEV_DEV_CONFIGURED
		//
		// Fri Sep 13 11:25:32 2022:INFO:-  VF 1 RTE_BBDEV_DEV_CONFIGURED
		if strings.Contains(lines[i], "Device Status:: ") {
			parseDeviceStatus(lines[i:], pciAddr, vfs, telemetryGatherer, logger)
		}
	}
}

func VrbparseTelemetry(file []byte, vfs []vrbv1.VF, pciAddr string, telemetryGatherer *telemetryGatherer, logger *logrus.Logger) {
	lines := strings.Split(string(file), "\n")
	for i := range lines {
		// Fri Sep 13 10:49:25 2022:INFO:FFT counters: Per Engine
		// Tue Sep 13 10:49:25 2022:INFO:0 0
		if strings.Contains(lines[i], "counters") {
			if i+1 < len(lines) {
				VrbparseCounters(lines[i], lines[i+1], vfs, pciAddr, telemetryGatherer, logger)
			} else {
				logger.WithField("metrics", len(lines)).WithField("pciAddr", pciAddr).Errorf("Telemetry counter value line missing.")
				return
			}
		}

		// Tue Sep 13 11:25:32 2022:INFO:Device Status:: 3 VFs
		//
		// Fri Sep 13 11:25:32 2022:INFO:-  VF 0 RTE_BBDEV_DEV_CONFIGURED
		//
		// Fri Sep 13 11:25:32 2022:INFO:-  VF 1 RTE_BBDEV_DEV_CONFIGURED
		if strings.Contains(lines[i], "Device Status:: ") {
			VrbparseDeviceStatus(lines[i:], pciAddr, vfs, telemetryGatherer, logger)
		}
	}
}

/******************************************************************************
 * Function: parseVFStatus
 * Description: Parses the VF status and updates the telemetryGatherer.
 *****************************************************************************/
func parseVFStatus(lines []string, vfCount int, vfs []VFUnion, telemetryGatherer *telemetryGatherer, log *logrus.Logger) {
	for vfIdx := 0; vfIdx < vfCount; vfIdx++ {
		for _, str := range lines {
			if strings.Contains(str, fmt.Sprintf("VF %v ", vfIdx)) {
				vfStatus := strings.Split(str, fmt.Sprintf("VF %v ", vfIdx))
				isReady := float64(0)
				if len(vfStatus) < 2 {
					log.WithField("value", str).Error("incomplete VF status line")
					continue
				}
				if strings.Contains(vfStatus[1], "CONFIGURED") || strings.Contains(vfStatus[1], "ACTIVE") {
					isReady = 1
				}

				var pciAddress string
				switch vf := vfs[vfIdx].(type) {
				case fec.VF:
					pciAddress = vf.PCIAddress
				case vrbv1.VF:
					pciAddress = vf.PCIAddress
				default:
					log.WithField("value", str).Error("unknown VF type")
					continue
				}

				telemetryGatherer.updateVfStatus(pciAddress, vfStatus[1], isReady)
			}
		}
	}
}

/******************************************************************************
 * Function: parseDeviceStatus
 * Description: Parses the device status and updates the telemetryGatherer.
 *              Calls parseVFStatus to parse and update the VF status.
 *****************************************************************************/
func parseDeviceStatus(lines []string, pfPciAddr string, vfs []fec.VF, telemetryGatherer *telemetryGatherer, log *logrus.Logger) {
	if len(lines) == 0 {
		log.Error("No lines provided for parsing device status")
		return
	}

	deviceStatus := strings.Split(lines[0], "Device Status:: ")
	if len(deviceStatus) < 2 {
		log.WithField("value", lines[0]).Error("incomplete device status line")
		return
	}

	vfCount, err := strconv.ParseFloat(strings.TrimSuffix(deviceStatus[1], " VFs"), 64)
	if err != nil {
		log.WithError(err).WithField("value", strings.TrimSuffix(deviceStatus[1], " VFs")).
			Error("failed to parse string into float64. Skipping metric.")
		return
	}
	telemetryGatherer.updateVfCount(pfPciAddr, string(fec.SucceededSync), vfCount)

	if int(vfCount) != len(vfs) {
		log.WithError(err).WithField("value", strings.TrimSuffix(deviceStatus[1], " VFs")).
			Error("No. of VFs from in metrics log is wrong. Skipping metric.")
		return
	}

	if len(lines) < (int(vfCount) + 1) {
		log.WithError(err).WithField("value", strings.TrimSuffix(deviceStatus[1], " VFs")).
			Error("failed to parse VF status. Skipping metric.")
		return
	}

	// Convert fec.VF to VFUnion
	vfUnion := make([]VFUnion, len(vfs))
	for i, vf := range vfs {
		vfUnion[i] = vf
	}

	parseVFStatus(lines[1:], int(vfCount), vfUnion, telemetryGatherer, log)
}

/******************************************************************************
 * Function: VrbparseDeviceStatus
 * Description: Parses the device status and updates the telemetryGatherer.
 *              Calls parseVFStatus to parse and update the VF status.
 *****************************************************************************/
func VrbparseDeviceStatus(lines []string, pfPciAddr string, vfs []vrbv1.VF, telemetryGatherer *telemetryGatherer, log *logrus.Logger) {
	if len(lines) == 0 {
		log.Error("No lines provided for parsing device status")
		return
	}

	deviceStatus := strings.Split(lines[0], "Device Status:: ")
	if len(deviceStatus) < 2 {
		log.WithField("value", lines[0]).Error("incomplete device status line")
		return
	}

	vfCount, err := strconv.ParseFloat(strings.TrimSuffix(deviceStatus[1], " VFs"), 64)
	if err != nil {
		log.WithError(err).WithField("value", strings.TrimSuffix(deviceStatus[1], " VFs")).
			Error("failed to parse string into float64. Skipping metric.")
		return
	}
	telemetryGatherer.updateVfCount(pfPciAddr, string(vrbv1.SucceededSync), vfCount)

	if int(vfCount) != len(vfs) {
		log.WithError(err).WithField("value", strings.TrimSuffix(deviceStatus[1], " VFs")).
			Error("No. of VFs from in metrics log is wrong. Skipping metric.")
		return
	}

	if len(lines) < (int(vfCount) + 1) {
		log.WithError(err).WithField("value", strings.TrimSuffix(deviceStatus[1], " VFs")).
			Error("failed to parse VF status. Skipping metric.")
		return
	}

	// Convert vrbv1.VF to VFUnion
	vfUnion := make([]VFUnion, len(vfs))
	for i, vf := range vfs {
		vfUnion[i] = vf
	}

	parseVFStatus(lines[1:], int(vfCount), vfUnion, telemetryGatherer, log)
}

/******************************************************************************
 * Function: processMetrics
 * Description: Processes the parsed metrics and updates the telemetryGatherer.
 *****************************************************************************/
func processMetrics(fieldName string, valueLineFormatted []string, vfs []VFUnion, pfPciAddr string, telemetryGatherer *telemetryGatherer, log *logrus.Logger) {
	opType := strings.Split(fieldName, " ")[0]
	for idx, val := range valueLineFormatted {
		value, err := strconv.ParseFloat(val, 64)
		if err != nil {
			log.WithError(err).WithField("name", fieldName).Error("failed to parse string into float64. Skipping metric.")
			continue
		}

		if strings.Contains(fieldName, "Engine") {
			updateTelemetry(fieldName, opType, "", pfPciAddr, value, idx, telemetryGatherer, log)
		} else {
			if idx >= len(vfs) {
				log.WithField("name", fieldName).Error("index out of range for VFs. Skipping metric.")
				continue
			}

			pciAddress, err := getPCIAddress(vfs[idx])
			if err != nil {
				log.WithError(err).WithField("fieldName", fieldName).WithField("value", value).Error("failed to get PCI address. Skipping metric.")
				continue
			}

			updateTelemetry(fieldName, opType, pciAddress, pfPciAddr, value, idx, telemetryGatherer, log)
		}
	}
}

/******************************************************************************
 * Function: getPCIAddress
 * Description: Extracts the PCI address from the VFUnion.
 *****************************************************************************/
func getPCIAddress(vf VFUnion) (string, error) {
	switch vf := vf.(type) {
	case fec.VF:
		return vf.PCIAddress, nil
	case vrbv1.VF:
		return vf.PCIAddress, nil
	default:
		return "", fmt.Errorf("unknown VF type")
	}
}

/******************************************************************************
 * Function: updateTelemetry
 * Description: Updates the telemetryGatherer with the parsed metrics.
 *****************************************************************************/
func updateTelemetry(fieldName, opType, pciAddress, pfPciAddr string, value float64, idx int, telemetryGatherer *telemetryGatherer, log *logrus.Logger) {
	switch {
	case strings.Contains(fieldName, "Blocks"):
		telemetryGatherer.updateCodeBlocks(opType, pciAddress, value)
	case strings.Contains(fieldName, "Bytes"):
		telemetryGatherer.updateBytes(opType, pciAddress, value)
	case strings.Contains(fieldName, "Engine"):
		telemetryGatherer.updateEngines(opType, strconv.Itoa(idx), pfPciAddr, value)
	default:
		log.WithField("fieldName", fieldName).WithField("value", value).Error("found unknown metric. Skipping it.")
	}
}

/******************************************************************************
 * Function: parseCounters
 * Description: Parses and processes telemetry counters for VFs and PFs.
 *              Updates the telemetryGatherer with the parsed metrics.
 *****************************************************************************/
func parseCounters(fieldLine, valueLine string, vfs []fec.VF, pfPciAddr string, telemetryGatherer *telemetryGatherer, log *logrus.Logger) {
	if len(fieldLine) == 0 || len(valueLine) == 0 {
		log.WithField("metrics", len(valueLine)).WithField("pciAddr", pfPciAddr).
			Errorf("Metrics values are null, skip it.")
		return
	}

	fieldName := strings.Split(fieldLine, "INFO:")[1]
	parts := strings.Split(valueLine, "INFO:")
	if len(parts) <= 1 {
		log.WithField("metrics", len(valueLine)).WithField("pciAddr", pfPciAddr).
			Errorf("Value line missing INFO, skip it.")
		return
	}

	valueLineFormatted := strings.Split(strings.TrimSpace(parts[1]), " ")

	if !strings.Contains(fieldName, "Engine") && len(valueLineFormatted) != len(vfs) {
		log.WithField("metrics", len(valueLine)).WithField("pciAddr", pfPciAddr).
			WithField("fieldName", fieldName).WithField("vfs", len(vfs)).
			Errorf("number of metrics doesn't equals to number of VFs")
		return
	}

	vfUnion := make([]VFUnion, len(vfs))
	for i, vf := range vfs {
		vfUnion[i] = vf
	}

	processMetrics(fieldName, valueLineFormatted, vfUnion, pfPciAddr, telemetryGatherer, log)
}

/******************************************************************************
 * Function: VrbparseCounters
 * Description: Parses and processes telemetry counters for VFs and PFs.
 *              Updates the telemetryGatherer with the parsed metrics.
 *****************************************************************************/
func VrbparseCounters(fieldLine, valueLine string, vfs []vrbv1.VF, pfPciAddr string, telemetryGatherer *telemetryGatherer, log *logrus.Logger) {
	if len(fieldLine) == 0 || len(valueLine) == 0 {
		log.WithField("metrics", len(valueLine)).WithField("pciAddr", pfPciAddr).
			Errorf("Metrics values are null, skip it.")
		return
	}

	fieldName := strings.Split(fieldLine, "INFO:")[1]
	parts := strings.Split(valueLine, "INFO:")
	if len(parts) <= 1 {
		log.WithField("metrics", len(valueLine)).WithField("pciAddr", pfPciAddr).
			Errorf("Value line missing INFO, skip it.")
		return
	}

	valueLineFormatted := strings.Split(strings.TrimSpace(parts[1]), " ")
	if !strings.Contains(fieldName, "Engine") && len(valueLineFormatted) != len(vfs) {
		log.WithField("metrics", len(valueLine)).WithField("pciAddr", pfPciAddr).
			WithField("fieldName", fieldName).WithField("vfs", len(vfs)).
			Errorf("number of metrics doesn't equals to number of VFs")
		return
	}

	vfUnion := make([]VFUnion, len(vfs))
	for i, vf := range vfs {
		vfUnion[i] = vf
	}

	processMetrics(fieldName, valueLineFormatted, vfUnion, pfPciAddr, telemetryGatherer, log)
}

func readFileWithTelemetry(pciAddr string, log *logrus.Logger) ([]byte, error) {
	var file []byte
	var fileContent string

	err := wait.Poll(time.Millisecond*50, time.Second, func() (done bool, err error) {
		file, err = os.ReadFile(fmt.Sprintf("/var/log/pf_bb_cfg_%v_response.log", pciAddr))
		if err != nil {
			log.WithField("pciAddr", pciAddr).WithError(err).Warnf("failed to read pf_bb_config log")
			return false, nil
		}
		fileContent = string(file)
		return strings.Contains(fileContent, "-- End of Response --"), nil
	})
	if err != nil {
		log.WithField("pciAddr", pciAddr).WithError(err).Error("timeout reading telemetry file")
		return nil, err
	}
	return file, nil
}

func requestTelemetry(pciAddr string, log *logrus.Logger) error {
	conn, err := net.Dial("unix", fmt.Sprintf("/tmp/pf_bb_config.%v.sock", pciAddr))
	if err != nil {
		log.WithField("pciAddr", pciAddr).WithError(err).Error("failed to open socket")
		return err
	}
	defer conn.Close()

	err = conn.SetWriteDeadline(time.Now().Add(time.Second))
	if err != nil {
		log.WithField("pciAddr", pciAddr).WithError(err).Error("failed to set timeout for request")
		return err
	}

	_, err = conn.Write([]byte{0x09, 0x00})
	if err != nil {
		log.WithField("pciAddr", pciAddr).WithError(err).Error("failed to send request to socket")
		return err
	}
	return nil
}

func clearLog(pciAddr string) error {
	fileName := fmt.Sprintf("/var/log/pf_bb_cfg_%v_response.log", pciAddr)
	_, err := os.Lstat(fileName)
	if errors.Is(err, os.ErrNotExist) {
		return nil // Don't truncate if file doesn't exists
	}
	if err != nil {
		return err
	}

	err = os.Truncate(fileName, 0)
	if err != nil {
		return err
	}
	return nil
}
