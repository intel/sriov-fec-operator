// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Intel Corporation

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

	fec "github.com/smart-edge-open/sriov-fec-operator/api/sriovfec/v2"
	vrbv1 "github.com/smart-edge-open/sriov-fec-operator/api/sriovvrb/v1"
	"github.com/smart-edge-open/sriov-fec-operator/pkg/common/utils"
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
	engineIdLabel   = "engine_id"
	statusLabel     = "status"
)

type telemetryGatherer struct {
	codeBlocksGauge, bytesGauge, engineGauge, vfStatusGauge, vfCountGauge *prometheus.GaugeVec
	metricUpdates                                                         []func()
}

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
	}, []string{engineIdLabel, pciAddressLabel, queueTypeLabel})

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

func (t *telemetryGatherer) updateEngines(opType, engineId, pciAddr string, value float64) {
	t.queueMetric(t.engineGauge, map[string]string{queueTypeLabel: opType, engineIdLabel: engineId, pciAddressLabel: pciAddr}, value)
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

func getMetrics(nodeName, namespace string, c client.Client, log *logrus.Logger, telemetryGatherer *telemetryGatherer) {
	sleepDuration := 15 * time.Second
	sleepEnv := os.Getenv(utils.SRIOV_PREFIX + "METRIC_GATHER_INTERVAL")
	if sleepEnv != "" {
		envDuration, err := time.ParseDuration(sleepEnv)
		if err != nil {
			log.WithError(err).WithField("default", sleepDuration).Error("user-provided value is incorrect 'Duration', using default value instead")
		} else {
			sleepDuration = envDuration
		}
	}

	utils.NewLogger().Info("metrics update loop will run every ", sleepDuration)
	wait.Forever(func() {
		nodeConfig := &fec.SriovFecNodeConfig{}
		err := c.Get(context.Background(), client.ObjectKey{Name: nodeName, Namespace: namespace}, nodeConfig)
		if err != nil {
			log.WithError(err).WithField("nodeName", nodeName).WithField("namespace", namespace).Error("failed to get SriovFecNodeConfig to fetch telemetry")
			return
		}

		if len(nodeConfig.Status.Conditions) > 0 && nodeConfig.Status.Conditions[0].Reason == string(fec.SucceededSync) {
			for _, acc := range nodeConfig.Status.Inventory.SriovAccelerators {
				if strings.EqualFold(acc.PFDriver, utils.VFIO_PCI) {
					getTelemetry(acc.PCIAddress, acc.VFs, telemetryGatherer, log)
				}
			}
		} else {
			for _, acc := range nodeConfig.Status.Inventory.SriovAccelerators {
				if strings.EqualFold(acc.PFDriver, utils.VFIO_PCI) {
					if len(nodeConfig.Status.Conditions) != 0 {
						telemetryGatherer.updateVfCount(acc.PCIAddress, nodeConfig.Status.Conditions[0].Reason, 0)
					} else {
						telemetryGatherer.updateVfCount(acc.PCIAddress, "", 0)
					}
				}
			}
		}

		VrbnodeConfig := &vrbv1.SriovVrbNodeConfig{}
		err = c.Get(context.Background(), client.ObjectKey{Name: nodeName, Namespace: namespace}, VrbnodeConfig)
		if err != nil {
			log.WithError(err).WithField("nodeName", nodeName).WithField("namespace", namespace).Error("failed to get SriovVrbNodeConfig to fetch telemetry")
			return
		}

		if len(VrbnodeConfig.Status.Conditions) > 0 && VrbnodeConfig.Status.Conditions[0].Reason == string(vrbv1.SucceededSync) {
			for _, acc := range VrbnodeConfig.Status.Inventory.SriovAccelerators {
				if strings.EqualFold(acc.PFDriver, utils.VFIO_PCI) {
					VrbgetTelemetry(acc.PCIAddress, acc.VFs, telemetryGatherer, log)
				}
			}
		} else {
			for _, acc := range VrbnodeConfig.Status.Inventory.SriovAccelerators {
				if strings.EqualFold(acc.PFDriver, utils.VFIO_PCI) {
					if len(VrbnodeConfig.Status.Conditions) != 0 {
						telemetryGatherer.updateVfCount(acc.PCIAddress, nodeConfig.Status.Conditions[0].Reason, 0)
					} else {
						telemetryGatherer.updateVfCount(acc.PCIAddress, "", 0)
					}
				}
			}
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
		//Fri Sep 13 10:49:25 2022:INFO:FFT counters: Per Engine
		//Tue Sep 13 10:49:25 2022:INFO:0 0
		if strings.Contains(lines[i], "counters") {
			parseCounters(lines[i], lines[i+1], vfs, pciAddr, telemetryGatherer, logger)
		}

		//Tue Sep 13 11:25:32 2022:INFO:Device Status:: 3 VFs
		//
		//Fri Sep 13 11:25:32 2022:INFO:-  VF 0 RTE_BBDEV_DEV_CONFIGURED
		//
		//Fri Sep 13 11:25:32 2022:INFO:-  VF 1 RTE_BBDEV_DEV_CONFIGURED
		if strings.Contains(lines[i], "Device Status:: ") {
			parseDeviceStatus(lines[i:], pciAddr, vfs, telemetryGatherer, logger)
		}
	}
}

func VrbparseTelemetry(file []byte, vfs []vrbv1.VF, pciAddr string, telemetryGatherer *telemetryGatherer, logger *logrus.Logger) {
	lines := strings.Split(string(file), "\n")
	for i := range lines {
		//Fri Sep 13 10:49:25 2022:INFO:FFT counters: Per Engine
		//Tue Sep 13 10:49:25 2022:INFO:0 0
		if strings.Contains(lines[i], "counters") {
			VrbparseCounters(lines[i], lines[i+1], vfs, pciAddr, telemetryGatherer, logger)
		}

		//Tue Sep 13 11:25:32 2022:INFO:Device Status:: 3 VFs
		//
		//Fri Sep 13 11:25:32 2022:INFO:-  VF 0 RTE_BBDEV_DEV_CONFIGURED
		//
		//Fri Sep 13 11:25:32 2022:INFO:-  VF 1 RTE_BBDEV_DEV_CONFIGURED
		if strings.Contains(lines[i], "Device Status:: ") {
			VrbparseDeviceStatus(lines[i:], pciAddr, vfs, telemetryGatherer, logger)
		}
	}
}

func parseDeviceStatus(lines []string, pfPciAddr string, vfs []fec.VF, telemetryGatherer *telemetryGatherer, log *logrus.Logger) {
	deviceStatus := strings.Split(lines[0], "Device Status:: ")
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

	for vfIdx := 0; vfIdx < int(vfCount); vfIdx++ {
		for _, str := range lines {
			if strings.Contains(str, fmt.Sprintf("VF %v ", vfIdx)) {
				vfStatus := strings.Split(str, fmt.Sprintf("VF %v ", vfIdx))
				isReady := float64(0)
				if strings.Contains(vfStatus[1], "CONFIGURED") || strings.Contains(vfStatus[1], "ACTIVE") {
					isReady = 1
				}
				telemetryGatherer.updateVfStatus(vfs[vfIdx].PCIAddress, vfStatus[1], isReady)
			}
		}
	}
}

func VrbparseDeviceStatus(lines []string, pfPciAddr string, vfs []vrbv1.VF, telemetryGatherer *telemetryGatherer, log *logrus.Logger) {
	deviceStatus := strings.Split(lines[0], "Device Status:: ")
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

	for vfIdx := 0; vfIdx < int(vfCount); vfIdx++ {
		for _, str := range lines {
			if strings.Contains(str, fmt.Sprintf("VF %v ", vfIdx)) {
				vfStatus := strings.Split(str, fmt.Sprintf("VF %v ", vfIdx))
				isReady := float64(0)
				if strings.Contains(vfStatus[1], "CONFIGURED") || strings.Contains(vfStatus[1], "ACTIVE") {
					isReady = 1
				}
				telemetryGatherer.updateVfStatus(vfs[vfIdx].PCIAddress, vfStatus[1], isReady)
			}
		}
	}
}

func parseCounters(fieldLine, valueLine string, vfs []fec.VF, pfPciAddr string, telemetryGatherer *telemetryGatherer, log *logrus.Logger) {

	if len(fieldLine) <= 0 || len(valueLine) <= 0 {
		log.WithField("metrics", len(valueLine)).WithField("pciAddr", pfPciAddr).
			Errorf("Metrics values are null, skip it.")
		return
	}

	fieldName := strings.Split(fieldLine, "INFO:")[1]
	value := strings.Split(valueLine, "INFO:")[1]

	valueLineFormatted := strings.Split(strings.TrimSpace(value), " ")

	if !strings.Contains(fieldName, "Engine") && len(valueLineFormatted) != len(vfs) {
		log.WithField("metrics", len(valueLine)).WithField("pciAddr", pfPciAddr).
			WithField("fieldName", fieldName).WithField("vfs", len(vfs)).
			Errorf("number of metrics doesn't equals to number of VFs")
		return
	}

	for idx := range valueLineFormatted {
		value, err := strconv.ParseFloat(valueLineFormatted[idx], 64)
		if err != nil {
			log.WithError(err).WithField("name", fieldName).Error("failed to parse string into float64. Skipping metric.")
			continue
		}

		opType := strings.Split(fieldName, " ")[0]
		switch {
		case strings.Contains(fieldName, "Blocks"):
			telemetryGatherer.updateCodeBlocks(opType, vfs[idx].PCIAddress, value)
		case strings.Contains(fieldName, "Bytes"):
			telemetryGatherer.updateBytes(opType, vfs[idx].PCIAddress, value)
		case strings.Contains(fieldName, "Engine"):
			telemetryGatherer.updateEngines(opType, strconv.Itoa(idx), pfPciAddr, value)
		default:
			log.WithField("fieldName", fieldName).WithField("value", value).Error("found unknown metric. Skipping it.")
		}
	}
}

func VrbparseCounters(fieldLine, valueLine string, vfs []vrbv1.VF, pfPciAddr string, telemetryGatherer *telemetryGatherer, log *logrus.Logger) {

	if len(fieldLine) <= 0 || len(valueLine) <= 0 {
		log.WithField("metrics", len(valueLine)).WithField("pciAddr", pfPciAddr).
			Errorf("Metrics values are null, skip it.")
		return
	}

	fieldName := strings.Split(fieldLine, "INFO:")[1]
	value := strings.Split(valueLine, "INFO:")[1]

	valueLineFormatted := strings.Split(strings.TrimSpace(value), " ")

	if !strings.Contains(fieldName, "Engine") && len(valueLineFormatted) != len(vfs) {
		log.WithField("metrics", len(valueLine)).WithField("pciAddr", pfPciAddr).
			WithField("fieldName", fieldName).WithField("vfs", len(vfs)).
			Errorf("number of metrics doesn't equals to number of VFs")
		return
	}

	for idx := range valueLineFormatted {
		value, err := strconv.ParseFloat(valueLineFormatted[idx], 64)
		if err != nil {
			log.WithError(err).WithField("name", fieldName).Error("failed to parse string into float64. Skipping metric.")
			continue
		}

		opType := strings.Split(fieldName, " ")[0]
		switch {
		case strings.Contains(fieldName, "Blocks"):
			telemetryGatherer.updateCodeBlocks(opType, vfs[idx].PCIAddress, value)
		case strings.Contains(fieldName, "Bytes"):
			telemetryGatherer.updateBytes(opType, vfs[idx].PCIAddress, value)
		case strings.Contains(fieldName, "Engine"):
			telemetryGatherer.updateEngines(opType, strconv.Itoa(idx), pfPciAddr, value)
		default:
			log.WithField("fieldName", fieldName).WithField("value", value).Error("found unknown metric. Skipping it.")
		}
	}
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
		return nil // don't truncate if file doesn't exists
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
