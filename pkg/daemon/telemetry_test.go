// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Intel Corporation

package daemon

import (
	"fmt"
	"net"
	"os"
	"strings"

	v2 "github.com/intel/sriov-fec-operator/api/sriovfec/v2"
	"github.com/intel/sriov-fec-operator/pkg/common/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
)

const fileLog = `
Fri Sep 16 10:42:33 2022:INFO:Device Status:: 6 VFs
Fri Sep 16 10:42:33 2022:INFO:-  VF 0 RTE_BBDEV_DEV_CONFIGURED
Fri Sep 16 10:42:33 2022:INFO:-  VF 1 RTE_BBDEV_DEV_CONFIGURED
Fri Sep 16 10:42:33 2022:INFO:-  VF 2 RTE_BBDEV_DEV_CONFIGURED
Fri Sep 16 10:42:33 2022:INFO:-  VF 3 RTE_BBDEV_DEV_CONFIGURED
Fri Sep 16 10:42:33 2022:INFO:-  VF 4 RTE_BBDEV_DEV_CONFIGURED
Fri Sep 16 10:42:33 2022:INFO:-  VF 5 RTE_BBDEV_DEV_CONFIGURED
Fri Sep 16 10:42:33 2022:INFO:5GUL counters: Code Blocks
Fri Sep 16 10:42:33 2022:INFO:0 0 0 0 0 0 
Fri Sep 16 10:42:33 2022:INFO:5GUL counters: Data (Bytes)
Fri Sep 16 10:42:33 2022:INFO:0 0 0 0 0 0 
Fri Sep 16 10:42:33 2022:INFO:5GUL counters: Per Engine
Fri Sep 16 10:42:33 2022:INFO:0 0 0 0 0 
Fri Sep 16 10:42:33 2022:INFO:5GDL counters: Code Blocks
Fri Sep 16 10:42:33 2022:INFO:0 0 0 0 0 0 
Fri Sep 16 10:42:33 2022:INFO:5GDL counters: Data (Bytes)
Fri Sep 16 10:42:33 2022:INFO:0 0 0 0 0 0 
Fri Sep 16 10:42:33 2022:INFO:5GDL counters: Per Engine
Fri Sep 16 10:42:33 2022:INFO:0 0 
Fri Sep 16 10:42:33 2022:INFO:FFT counters: Code Blocks
Fri Sep 16 10:42:33 2022:INFO:0 0 0 0 0 0 
Fri Sep 16 10:42:33 2022:INFO:FFT counters: Data (Bytes)
Fri Sep 16 10:42:33 2022:INFO:0 0 0 0 0 0 
Fri Sep 16 10:42:33 2022:INFO:FFT counters: Per Engine
Fri Sep 16 10:42:33 2022:INFO:0 
Fri Sep 16 10:42:33 2022:DEBUG:event_processor(): Waiting on poll...
Mon Sep 19 07:45:28 2022:DEBUG:sig_fun(): Signal Received with sigNum = 15
-- End of Response --
`

var _ = Describe("clearLog", func() {
	It("file doesn't exists", func() {
		testPciAddr := "0123:34:56.0"
		err := clearLog(testPciAddr)
		Expect(err).To(BeNil())
	})

	It("file exists and is not empty", func() {
		testPciAddr := "1111:11:11.1"
		file, err := os.Create(fmt.Sprintf("/var/log/pf_bb_cfg_%v_response.log", testPciAddr))
		Expect(err).To(BeNil())

		_, err = file.Write([]byte("abcdefg"))
		Expect(err).To(BeNil())

		stat, err := file.Stat()
		Expect(err).To(BeNil())
		Expect(stat.Size()).ToNot(Equal(int64(0)))

		err = clearLog(testPciAddr)

		Expect(err).To(BeNil())
		stat, err = file.Stat()
		Expect(err).To(BeNil())
		Expect(stat.Size()).To(Equal(int64(0)))
	})
})

var _ = Describe("requestTelemetry", func() {
	It("socket doesn't exists", func() {
		err := requestTelemetry("0000:ff:ff.0", utils.NewLogger())
		Expect(err).To(MatchError("dial unix /tmp/pf_bb_config.0000:ff:ff.0.sock: connect: no such file or directory"))
	})

	It("socket exists and responds correctly", func() {
		pciAddr := "1111:11:23.1"
		listener, err := net.Listen("unix", fmt.Sprintf("/tmp/pf_bb_config.%v.sock", pciAddr))
		Expect(err).To(BeNil())
		defer listener.Close()

		err = requestTelemetry(pciAddr, utils.NewLogger())
		Expect(err).To(BeNil())
	})

})

var _ = Describe("readFileWithTelemetry", func() {
	It("file doesn't exists", func() {
		logger := utils.NewLogger()
		th := &testHook{
			expectedError: "open /var/log/pf_bb_cfg_2222:22:22.2_response.log: no such file or directory",
		}

		logger.AddHook(th)

		file, err := readFileWithTelemetry("2222:22:22.2", logger)

		Expect(file).To(BeNil())
		Expect(err).To(MatchError("timed out waiting for the condition"))
		Expect(th.expectedErrorOccured).To(BeTrue())
	})

	It("file exists but it is empty", func() {
		pciAddr := "4321:11:23.1"
		fileHandler, err := os.Create(fmt.Sprintf("/var/log/pf_bb_cfg_%v_response.log", pciAddr))
		Expect(err).To(BeNil())
		defer fileHandler.Close()

		file, err := readFileWithTelemetry(pciAddr, utils.NewLogger())

		Expect(file).To(BeNil())
		Expect(err).To(MatchError("timed out waiting for the condition"))
	})

	It("file exists, is not empty but end tag is missing", func() {
		pciAddr := "4444:11:23.1"
		logger := utils.NewLogger()
		th := &testHook{
			expectedError: "timed out waiting for the condition",
		}

		logger.AddHook(th)
		fileHandler, err := os.Create(fmt.Sprintf("/var/log/pf_bb_cfg_%v_response.log", pciAddr))
		Expect(err).To(BeNil())
		defer fileHandler.Close()
		fileLogWithoutEndTag := strings.Replace(fileLog, "-- End of Response --", "", -1)
		_, err = fileHandler.Write([]byte(fileLogWithoutEndTag))
		Expect(err).To(BeNil())

		file, err := readFileWithTelemetry(pciAddr, logger)

		Expect(file).To(BeNil())
		Expect(err).To(MatchError("timed out waiting for the condition"))
		Expect(th.expectedErrorOccured).To(BeTrue())
	})

	It("file exists and is not empty", func() {
		pciAddr := "4444:11:23.1"
		fileHandler, err := os.Create(fmt.Sprintf("/var/log/pf_bb_cfg_%v_response.log", pciAddr))
		Expect(err).To(BeNil())
		defer fileHandler.Close()
		_, err = fileHandler.Write([]byte(fileLog))
		Expect(err).To(BeNil())

		file, err := readFileWithTelemetry(pciAddr, utils.NewLogger())

		Expect(file).To(BeEquivalentTo(fileLog))
		Expect(err).To(BeNil())
	})
})

var _ = Describe("parseCounters", func() {
	tg := newTelemetryGatherer()
	BeforeEach(func() {
		tg.resetMetrics()
	})

	It("value Line null", func() {
		fieldLine := "Fri Sep 13 10:49:25 2022:INFO:FFT counters: Per Engine"
		valueLine := ""

		logger := utils.NewLogger()
		hook := &testHook{
			expectedError: "Metrics values are null, skip it.",
		}
		logger.AddHook(hook)

		parseCounters(fieldLine, valueLine, []v2.VF{
			{PCIAddress: "9999:99:99.9"},
		}, "9999:99:99.0", tg, logger)
		tg.updateMetrics()

		Expect(testutil.CollectAndCount(tg.engineGauge)).To(Equal(0))
		Expect(hook.expectedErrorOccured).To(BeTrue())
	})

	It("field Line null", func() {
		fieldLine := ""
		valueLine := ""

		logger := utils.NewLogger()
		hook := &testHook{
			expectedError: "Metrics values are null, skip it.",
		}
		logger.AddHook(hook)

		parseCounters(fieldLine, valueLine, []v2.VF{
			{PCIAddress: "9999:99:99.9"},
		}, "9999:99:99.0", tg, logger)
		tg.updateMetrics()

		Expect(testutil.CollectAndCount(tg.engineGauge)).To(Equal(0))
		Expect(hook.expectedErrorOccured).To(BeTrue())
	})

	It("Incomplete Value Line data", func() {
		fieldLine := "Fri Sep 13 10:49:25 2022:INFO:FFT counters: Per Engine"
		valueLine := "Tue Sep 13 10:49:25 2022:INFO:"

		logger := utils.NewLogger()
		hook := &testHook{
			expectedError: "failed to parse string into float64. Skipping metric.",
		}
		logger.AddHook(hook)

		parseCounters(fieldLine, valueLine, []v2.VF{
			{PCIAddress: "9999:99:99.9"},
		}, "9999:99:99.0", tg, logger)
		tg.updateMetrics()

		Expect(testutil.CollectAndCount(tg.engineGauge)).To(Equal(0))
		Expect(hook.expectedErrorOccured).To(BeTrue())
	})

	It("one FFT engine value is exposed", func() {
		parseCounters("Fri Sep 13 10:49:25 2022:INFO:FFT counters: Per Engine", "Tue Sep 13 10:49:25 2022:INFO:123", []v2.VF{
			{PCIAddress: "9999:99:99.9"},
		}, "9999:99:99.0", tg, utils.NewLogger())
		tg.updateMetrics()

		Expect(testutil.ToFloat64(tg.engineGauge)).To(Equal(float64(123)))
	})

	It("one FFT engine value is exposed - check metric with labels", func() {
		pfPciAddr := "9999:99:99.0"
		parseCounters("Fri Sep 13 10:49:25 2022:INFO:FFT counters: Per Engine", "Tue Sep 13 10:49:25 2022:INFO:999", []v2.VF{
			{PCIAddress: "9999:99:99.9"},
		}, pfPciAddr, tg, utils.NewLogger())
		tg.updateMetrics()

		gauge, err := tg.engineGauge.GetMetricWith(map[string]string{engineIdLabel: "0", queueTypeLabel: "FFT", pciAddressLabel: pfPciAddr})
		Expect(err).To(Succeed())
		Expect(testutil.ToFloat64(gauge)).To(Equal(float64(999)))
	})

	It("3 5GUL values are exposed with no values", func() {
		pfPciAddr := "9999:00:00.0"
		fieldLine := "Fri Sep 13 10:49:25 2022:INFO:5GUL counters: Data (Bytes)"
		valueLine := "Tue Sep 13 10:49:25 2022:INFO:"

		logger := utils.NewLogger()
		hook := &testHook{
			expectedError: "number of metrics doesn't equals to number of VFs",
		}
		logger.AddHook(hook)

		parseCounters(fieldLine, valueLine, []v2.VF{
			{PCIAddress: "9999:01:00.0"},
			{PCIAddress: "9999:01:00.1"},
			{PCIAddress: "9999:01:00.2"},
		}, pfPciAddr, tg, logger)
		tg.updateMetrics()

		Expect(testutil.CollectAndCount(tg.engineGauge)).To(Equal(0))
		Expect(hook.expectedErrorOccured).To(BeTrue())
	})

	It("3 5GUL values are exposed", func() {
		pfPciAddr := "9999:00:00.0"
		opType := "5GUL"
		parseCounters("Fri Sep 13 10:49:25 2022:INFO:"+opType+" counters: Data (Bytes)", "Tue Sep 13 10:49:25 2022:INFO:123 456 789", []v2.VF{
			{PCIAddress: "9999:01:00.0"},
			{PCIAddress: "9999:01:00.1"},
			{PCIAddress: "9999:01:00.2"},
		}, pfPciAddr, tg, utils.NewLogger())
		tg.updateMetrics()

		gauge0, err := tg.bytesGauge.GetMetricWith(map[string]string{queueTypeLabel: opType, pciAddressLabel: "9999:01:00.0"})
		Expect(err).To(Succeed())
		Expect(testutil.ToFloat64(gauge0)).To(Equal(float64(123)))

		gauge1, err := tg.bytesGauge.GetMetricWith(map[string]string{queueTypeLabel: opType, pciAddressLabel: "9999:01:00.1"})
		Expect(err).To(Succeed())
		Expect(testutil.ToFloat64(gauge1)).To(Equal(float64(456)))

		gauge2, err := tg.bytesGauge.GetMetricWith(map[string]string{queueTypeLabel: opType, pciAddressLabel: "9999:01:00.2"})
		Expect(err).To(Succeed())
		Expect(testutil.ToFloat64(gauge2)).To(Equal(float64(789)))
	})

	It("when unknown metric is exposed, then it should be skipped", func() {
		pfPciAddr := "9999:99:99.0"
		logger := utils.NewLogger()
		hook := &testHook{
			expectedError: "found unknown metric. Skipping it.",
		}
		logger.AddHook(hook)

		parseCounters("Fri Sep 13 10:49:25 2022:INFO:FFT counters: MewType", "Tue Sep 13 10:49:25 2022:INFO:999", []v2.VF{
			{PCIAddress: "9999:99:99.9"},
		}, pfPciAddr, tg, logger)
		tg.updateMetrics()

		Expect(testutil.CollectAndCount(tg.engineGauge)).To(Equal(0))
		Expect(hook.expectedErrorOccured).To(BeTrue())
	})

	It("when metric is non-numeric, then it should be skipped", func() {
		pfPciAddr := "9999:99:99.0"
		logger := utils.NewLogger()
		hook := &testHook{
			expectedError: "failed to parse string into float64. Skipping metric.",
		}
		logger.AddHook(hook)

		parseCounters("Fri Sep 13 10:49:25 2022:INFO:FFT counters: Data (Bytes)", "Tue Sep 13 10:49:25 2022:INFO:shouldBeFloat", []v2.VF{
			{PCIAddress: "9999:99:99.9"},
		}, pfPciAddr, tg, logger)
		tg.updateMetrics()

		Expect(testutil.CollectAndCount(tg.engineGauge)).To(Equal(0))
		Expect(hook.expectedErrorOccured).To(BeTrue())
	})

	It("2 values are exposed - numeric one should be gathered, string should be skipped", func() {
		pfPciAddr := "9999:00:00.0"
		opType := "5GUL"
		logger := utils.NewLogger()
		hook := &testHook{
			expectedError: "failed to parse string into float64. Skipping metric.",
		}
		logger.AddHook(hook)

		parseCounters("Fri Sep 13 10:49:25 2022:INFO:"+opType+" counters: Data (Bytes)", "Tue Sep 13 10:49:25 2022:INFO:invalid 456", []v2.VF{
			{PCIAddress: "9999:01:00.0"},
			{PCIAddress: "9999:01:00.1"},
		}, pfPciAddr, tg, logger)
		tg.updateMetrics()

		Expect(hook.expectedErrorOccured).To(BeTrue())

		gauge1, err := tg.bytesGauge.GetMetricWith(map[string]string{queueTypeLabel: opType, pciAddressLabel: "9999:01:00.1"})
		Expect(testutil.CollectAndCount(tg.bytesGauge)).To(Equal(1))
		Expect(err).To(Succeed())
		Expect(testutil.ToFloat64(gauge1)).To(Equal(float64(456)))
	})

	It("when number of metrics doesn't equals number of VFs, then metrics should be skipped", func() {
		pfPciAddr := "9999:00:00.0"
		opType := "5GUL"
		logger := utils.NewLogger()
		hook := &testHook{
			expectedError: "number of metrics doesn't equals to number of VFs",
		}
		logger.AddHook(hook)

		parseCounters("Fri Sep 13 10:49:25 2022:INFO:"+opType+" counters: Data (Bytes)", "Tue Sep 13 10:49:25 2022:INFO:456", []v2.VF{
			{PCIAddress: "9999:01:00.0"},
			{PCIAddress: "9999:01:00.1"},
		}, pfPciAddr, tg, logger)
		tg.updateMetrics()

		Expect(hook.expectedErrorOccured).To(BeTrue())

		Expect(testutil.CollectAndCount(tg.bytesGauge)).To(Equal(0))
	})
})

var _ = Describe("parseDeviceStatus", func() {
	tg := newTelemetryGatherer()
	BeforeEach(func() {
		tg.resetMetrics()
	})

	It("Should correctly handle proper data without blank lines", func() {
		fileLog := `Fri Sep 16 10:42:33 2022:INFO:Device Status:: 4 VFs
Fri Sep 16 10:42:33 2022:INFO:-  VF 0 RTE_BBDEV_DEV_CONFIGURED
Fri Sep 16 10:42:33 2022:INFO:-  VF 1 RTE_BBDEV_DEV_ACTIVE
Fri Sep 16 10:42:33 2022:INFO:-  VF 2 RTE_BBDEV_DEV_RESTART_REQ
Fri Sep 16 10:42:33 2022:INFO:-  VF 3 RTE_BBDEV_DEV_RECONFIG_REQ
`

		parseDeviceStatus(strings.Split(fileLog, "\n"), "1111:00:00.0", []v2.VF{
			{PCIAddress: "1111:01:00.0"},
			{PCIAddress: "1111:01:00.1"},
			{PCIAddress: "1111:01:00.2"},
			{PCIAddress: "1111:01:00.3"},
		}, tg, utils.NewLogger())
		tg.updateMetrics()

		Expect(testutil.ToFloat64(tg.vfCountGauge)).To(Equal(float64(4)))
		Expect(testutil.CollectAndCount(tg.vfStatusGauge)).To(Equal(4))

		vf0Gauge, err := tg.vfStatusGauge.GetMetricWith(map[string]string{pciAddressLabel: "1111:01:00.0", statusLabel: "RTE_BBDEV_DEV_CONFIGURED"})
		Expect(err).To(Succeed())
		Expect(testutil.CollectAndCount(vf0Gauge)).To(Equal(1))
		Expect(testutil.ToFloat64(vf0Gauge)).To(Equal(float64(1)))

		vf1Gauge, err := tg.vfStatusGauge.GetMetricWith(map[string]string{pciAddressLabel: "1111:01:00.1", statusLabel: "RTE_BBDEV_DEV_ACTIVE"})
		Expect(err).To(Succeed())
		Expect(testutil.CollectAndCount(vf1Gauge)).To(Equal(1))
		Expect(testutil.ToFloat64(vf1Gauge)).To(Equal(float64(1)))

		vf2Gauge, err := tg.vfStatusGauge.GetMetricWith(map[string]string{pciAddressLabel: "1111:01:00.2", statusLabel: "RTE_BBDEV_DEV_FATAL_ERR"})
		Expect(err).To(Succeed())
		Expect(testutil.CollectAndCount(vf2Gauge)).To(Equal(1))
		Expect(testutil.ToFloat64(vf2Gauge)).To(Equal(float64(0)))

		vf3Gauge, err := tg.vfStatusGauge.GetMetricWith(map[string]string{pciAddressLabel: "1111:01:00.3", statusLabel: "RTE_BBDEV_DEV_RESTART_REQ"})
		Expect(err).To(Succeed())
		Expect(testutil.CollectAndCount(vf3Gauge)).To(Equal(1))
		Expect(testutil.ToFloat64(vf3Gauge)).To(Equal(float64(0)))
	})

	It("Should correctly handle proper data with blank lines", func() {
		fileLog := `Fri Sep 16 10:42:33 2022:INFO:Device Status:: 4 VFs

Fri Sep 16 10:42:33 2022:INFO:-  VF 0 RTE_BBDEV_DEV_CONFIGURED

Fri Sep 16 10:42:33 2022:INFO:-  VF 1 RTE_BBDEV_DEV_ACTIVE

Fri Sep 16 10:42:33 2022:INFO:-  VF 2 RTE_BBDEV_DEV_RESTART_REQ

Fri Sep 16 10:42:33 2022:INFO:-  VF 3 RTE_BBDEV_DEV_RECONFIG_REQ

`

		parseDeviceStatus(strings.Split(fileLog, "\n"), "1111:00:00.0", []v2.VF{
			{PCIAddress: "1111:01:00.0"},
			{PCIAddress: "1111:01:00.1"},
			{PCIAddress: "1111:01:00.2"},
			{PCIAddress: "1111:01:00.3"},
		}, tg, utils.NewLogger())
		tg.updateMetrics()

		Expect(testutil.ToFloat64(tg.vfCountGauge)).To(Equal(float64(4)))
		Expect(testutil.CollectAndCount(tg.vfStatusGauge)).To(Equal(4))

		vf0Gauge, err := tg.vfStatusGauge.GetMetricWith(map[string]string{pciAddressLabel: "1111:01:00.0", statusLabel: "RTE_BBDEV_DEV_CONFIGURED"})
		Expect(err).To(Succeed())
		Expect(testutil.CollectAndCount(vf0Gauge)).To(Equal(1))
		Expect(testutil.ToFloat64(vf0Gauge)).To(Equal(float64(1)))

		vf1Gauge, err := tg.vfStatusGauge.GetMetricWith(map[string]string{pciAddressLabel: "1111:01:00.1", statusLabel: "RTE_BBDEV_DEV_ACTIVE"})
		Expect(err).To(Succeed())
		Expect(testutil.CollectAndCount(vf1Gauge)).To(Equal(1))
		Expect(testutil.ToFloat64(vf1Gauge)).To(Equal(float64(1)))

		vf2Gauge, err := tg.vfStatusGauge.GetMetricWith(map[string]string{pciAddressLabel: "1111:01:00.2", statusLabel: "RTE_BBDEV_DEV_FATAL_ERR"})
		Expect(err).To(Succeed())
		Expect(testutil.CollectAndCount(vf2Gauge)).To(Equal(1))
		Expect(testutil.ToFloat64(vf2Gauge)).To(Equal(float64(0)))

		vf3Gauge, err := tg.vfStatusGauge.GetMetricWith(map[string]string{pciAddressLabel: "1111:01:00.3", statusLabel: "RTE_BBDEV_DEV_RESTART_REQ"})
		Expect(err).To(Succeed())
		Expect(testutil.CollectAndCount(vf3Gauge)).To(Equal(1))
		Expect(testutil.ToFloat64(vf3Gauge)).To(Equal(float64(0)))
	})

	It("Should skip metrics if unable to parse amount of VFs", func() {
		logger := utils.NewLogger()
		hook := &testHook{
			expectedError: "failed to parse string into float64. Skipping metric.",
		}
		logger.AddHook(hook)
		fileLog := `Fri Sep 16 10:42:33 2022:INFO:Device Status:: XYZ VFs`
		parseDeviceStatus(strings.Split(fileLog, "\n"), "1111:00:00.0", []v2.VF{
			{PCIAddress: "1111:01:00.0"},
		}, tg, logger)

		Expect(testutil.CollectAndCount(tg.vfStatusGauge)).To(Equal(0))
		Expect(testutil.CollectAndCount(tg.vfCountGauge)).To(Equal(0))
		Expect(hook.expectedErrorOccured).To(BeTrue())
	})

	It("Should skip metrics for insufficient data", func() {
		logger := utils.NewLogger()
		hook := &testHook{
			expectedError: "failed to parse VF status. Skipping metric.",
		}
		logger.AddHook(hook)
		fileLog := `Fri Sep 16 10:42:33 2022:INFO:Device Status:: 1 VFs`
		parseDeviceStatus(strings.Split(fileLog, "\n"), "1111:00:00.0", []v2.VF{
			{PCIAddress: "1111:01:00.0"},
		}, tg, logger)
		tg.updateMetrics()

		Expect(testutil.CollectAndCount(tg.vfCountGauge)).To(Equal(1))
		Expect(hook.expectedErrorOccured).To(BeTrue())
	})

	It("Should skip metrics for when VF count is less than expected", func() {
		logger := utils.NewLogger()
		hook := &testHook{
			expectedError: "No. of VFs from in metrics log is wrong. Skipping metric.",
		}
		logger.AddHook(hook)
		fileLog := `Fri Sep 16 10:42:33 2022:INFO:Device Status:: 1 VFs
Fri Sep 16 10:42:33 2022:INFO:-  VF 0 RTE_BBDEV_DEV_CONFIGURED

`
		parseDeviceStatus(strings.Split(fileLog, "\n"), "1111:00:00.0", []v2.VF{
			{PCIAddress: "1111:01:00.0"},
			{PCIAddress: "1111:01:00.1"},
			{PCIAddress: "1111:01:00.2"},
		}, tg, logger)
		tg.updateMetrics()

		Expect(testutil.CollectAndCount(tg.vfCountGauge)).To(Equal(1))
		Expect(hook.expectedErrorOccured).To(BeTrue())
	})

	It("Should skip metrics for when VF count is more than expected without blank lines", func() {
		logger := utils.NewLogger()
		hook := &testHook{
			expectedError: "No. of VFs from in metrics log is wrong. Skipping metric.",
		}
		logger.AddHook(hook)
		fileLog := `Fri Sep 16 10:42:33 2022:INFO:Device Status:: 3 VFs
Fri Sep 16 10:42:33 2022:INFO:-  VF 0 RTE_BBDEV_DEV_CONFIGURED
Fri Sep 16 10:42:33 2022:INFO:-  VF 1 RTE_BBDEV_DEV_CONFIGURED
Fri Sep 16 10:42:33 2022:INFO:-  VF 3 RTE_BBDEV_DEV_CONFIGURED
`
		parseDeviceStatus(strings.Split(fileLog, "\n"), "1111:00:00.0", []v2.VF{
			{PCIAddress: "1111:01:00.1"},
		}, tg, logger)
		tg.updateMetrics()

		Expect(testutil.CollectAndCount(tg.vfCountGauge)).To(Equal(1))
		Expect(hook.expectedErrorOccured).To(BeTrue())
	})

	It("Should skip metrics for when VF count is more than expected with blank lines", func() {
		logger := utils.NewLogger()
		hook := &testHook{
			expectedError: "No. of VFs from in metrics log is wrong. Skipping metric.",
		}
		logger.AddHook(hook)
		fileLog := `Fri Sep 16 10:42:33 2022:INFO:Device Status:: 3 VFs

Fri Sep 16 10:42:33 2022:INFO:-  VF 0 RTE_BBDEV_DEV_CONFIGURED

Fri Sep 16 10:42:33 2022:INFO:-  VF 1 RTE_BBDEV_DEV_CONFIGURED

Fri Sep 16 10:42:33 2022:INFO:-  VF 3 RTE_BBDEV_DEV_CONFIGURED

`
		parseDeviceStatus(strings.Split(fileLog, "\n"), "1111:00:00.0", []v2.VF{
			{PCIAddress: "1111:01:00.1"},
		}, tg, logger)
		tg.updateMetrics()

		Expect(testutil.CollectAndCount(tg.vfCountGauge)).To(Equal(1))
		Expect(hook.expectedErrorOccured).To(BeTrue())
	})
})

type testHook struct {
	expectedError        string
	expectedErrorOccured bool
}

func (th *testHook) Levels() []logrus.Level {
	return []logrus.Level{0, 1, 2, 3, 4, 5, 6, 7}
}

func (th *testHook) Fire(entry *logrus.Entry) error {
	entr, err := entry.String()
	if err != nil {
		return err
	}
	if strings.Contains(entr, th.expectedError) {
		th.expectedErrorOccured = true
	}
	return nil
}
