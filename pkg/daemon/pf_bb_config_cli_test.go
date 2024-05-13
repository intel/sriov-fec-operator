// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2023 Intel Corporation

package daemon

import (
	"fmt"
	"net"
	"os"

	"github.com/intel/sriov-fec-operator/pkg/common/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const missingEndStr = `
Tue Oct 10 18:35:48 2023:INFO:Read 0x00C84060 0x00003030

Tue Oct 10 18:35:48 2023:INFO:`

const mmReadLog = `
Tue Oct 10 18:35:48 2023:INFO:Read 0x00C84060 0x00003030

Tue Oct 10 18:35:48 2023:INFO:-- End of Response --`

var _ = Describe("sendCmd", func() {
	var cmd = []byte{0x09, 0x00}
	It("socket doesn't exists", func() {
		err := sendCmd("0000:ff:ff.0", cmd, utils.NewLogger())
		Expect(err).To(MatchError("dial unix /tmp/pf_bb_config.0000:ff:ff.0.sock: connect: no such file or directory"))
	})

	It("socket exists and responds correctly", func() {
		pciAddr := "1111:11:23.1"
		listener, err := net.Listen("unix", fmt.Sprintf("/tmp/pf_bb_config.%v.sock", pciAddr))
		Expect(err).To(BeNil())
		defer listener.Close()

		err = sendCmd(pciAddr, cmd, utils.NewLogger())
		Expect(err).To(BeNil())
	})
})

var _ = Describe("resetMode", func() {
	It("missing reset mode argument", func() {
		_, err := resetMode([]string{})
		Expect(err).To(MatchError("error: missing argument for command reset_mode"))
	})

	It("invalid reset mode", func() {
		_, err := resetMode([]string{"dummy_mode"})
		Expect(err).To(MatchError("error: invalid reset_mode value"))
	})

	It("valid pf_flr reset mode", func() {
		_, err := resetMode([]string{"pf_flr"})
		Expect(err).To(BeNil())
	})

	It("valid cluster_reset reset mode", func() {
		_, err := resetMode([]string{"cluster_reset"})
		Expect(err).To(BeNil())
	})
})

var _ = Describe("autoReset", func() {
	It("missing auto reset argument", func() {
		_, err := autoReset([]string{})
		Expect(err).To(MatchError("error: missing argument for command auto_reset"))
	})

	It("invalid auto reset mode", func() {
		_, err := autoReset([]string{"foo"})
		Expect(err).To(MatchError("error: invalid auto_reset value"))
	})

	It("valid auto_reset on", func() {
		_, err := autoReset([]string{"on"})
		Expect(err).To(BeNil())
	})

	It("valid auto_reset off", func() {
		_, err := autoReset([]string{"off"})
		Expect(err).To(BeNil())
	})
})

var _ = Describe("regDump", func() {
	It("missing register dump argument", func() {
		_, err := regDump([]string{})
		Expect(err).To(MatchError("error: missing argument for reg_dump"))
	})

	It("invalid fec device", func() {
		_, err := regDump([]string{"5052"})
		Expect(err).To(MatchError("error: invalid device for reg_dump"))
	})

	It("valid ACC100 fec device", func() {
		_, err := regDump([]string{"0d5c"})
		Expect(err).To(BeNil())
	})

	It("valid VRB1 fec device", func() {
		_, err := regDump([]string{"57c0"})
		Expect(err).To(BeNil())
	})

	It("valid VRB2 fec device", func() {
		_, err := regDump([]string{"57c2"})
		Expect(err).To(BeNil())
	})
})

var _ = Describe("mmRead", func() {
	It("missing read address", func() {
		_, err := mmRead([]string{})
		Expect(err).To(MatchError("error: missing register address for mm_read"))
	})

	It("read incomplete register address", func() {
		_, err := mmRead([]string{"0x"})
		Expect(err).To(MatchError("error: invalid input for register address"))
	})

	It("read non hex register address", func() {
		_, err := mmRead([]string{"123456"})
		Expect(err).To(MatchError("error: dump address must be HEX"))
	})

	It("invalid hex value for register address", func() {
		_, err := mmRead([]string{"0xAVX512"})
		Expect(err).To(MatchError("error: failed to convert address string to uint"))
	})
})

var _ = Describe("pollLogFile", func() {
	It("file doesn't exists", func() {
		pciAddr := "2222:22:22.2"
		filePath := "/var/log/pf_bb_cfg_2222:22:22.2_response.log"
		searchStr := ""
		file, err := pollLogFile(pciAddr, filePath, searchStr, utils.NewLogger())
		Expect(file).To(BeNil())
		Expect(err).To(MatchError("open /var/log/pf_bb_cfg_2222:22:22.2_response.log: no such file or directory"))
	})

	It("file exists but it is empty", func() {
		pciAddr := "4321:11:23.1"
		filePath := fmt.Sprintf("/var/log/pf_bb_cfg_%v_response.log", pciAddr)
		searchStr := "-- End of Response --"

		fileHandler, err := os.Create(filePath)
		Expect(err).To(BeNil())
		defer fileHandler.Close()

		file, err := pollLogFile(pciAddr, filePath, searchStr, utils.NewLogger())
		Expect(file).To(BeNil())
		Expect(err).To(MatchError("timed out waiting for the condition"))
	})

	It("file exists but search string missing", func() {
		pciAddr := "4444:11:23.1"
		filePath := fmt.Sprintf("/var/log/pf_bb_cfg_%v_response.log", pciAddr)
		searchStr := "-- End of Response --"

		fileHandler, err := os.Create(filePath)
		Expect(err).To(BeNil())
		defer fileHandler.Close()
		_, err = fileHandler.Write([]byte(missingEndStr))
		Expect(err).To(BeNil())

		file, err := pollLogFile(pciAddr, filePath, searchStr, utils.NewLogger())
		Expect(file).To(BeNil())
		Expect(err).To(MatchError("timed out waiting for the condition"))
	})

	It("file exists and contains search string", func() {
		pciAddr := "4444:11:23.1"
		filePath := fmt.Sprintf("/var/log/pf_bb_cfg_%v_response.log", pciAddr)
		searchStr := "-- End of Response --"

		fileHandler, err := os.Create(filePath)
		Expect(err).To(BeNil())
		defer fileHandler.Close()
		_, err = fileHandler.Write([]byte(mmReadLog))
		Expect(err).To(BeNil())

		file, err := pollLogFile(pciAddr, filePath, searchStr, utils.NewLogger())
		Expect(file).To(BeEquivalentTo(mmReadLog))
		Expect(err).To(BeNil())
	})
})
