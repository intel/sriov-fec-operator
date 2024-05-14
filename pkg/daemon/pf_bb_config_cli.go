// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2024 Intel Corporation

package daemon

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	fec "github.com/intel/sriov-fec-operator/api/sriovfec/v2"
	"github.com/intel/sriov-fec-operator/pkg/common/utils"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type cmdFunc func(cmd []string) ([]byte, error)

type cmdDef struct {
	execFunc    cmdFunc
	respFile    string
	respPattern string
}

// FEC Device IDs
var fecDevices = map[string][]byte{
	"0d5c": {0x0, 0x0, 0x0, 0x0, 0x5C, 0x0D, 0x0, 0x0}, // ACC100
	"57c0": {0x0, 0x0, 0x0, 0x0, 0xC0, 0x57, 0x0, 0x0}, // VRB1
	"57c2": {0x0, 0x0, 0x0, 0x0, 0xC2, 0x57, 0x0, 0x0}, // VRB2
}

// Valid pf_bb_config_cli commands
var cliCommands = map[string]cmdDef{
	"reset_mode": {
		execFunc:    resetMode,
		respFile:    "main",
		respPattern: "reset_mode set to",
	},
	"auto_reset": {
		execFunc:    autoReset,
		respFile:    "main",
		respPattern: "Auto reset set to",
	},
	"clear_log": {
		execFunc:    clearLogCli,
		respFile:    "main",
		respPattern: "",
	},
	"reg_dump": {
		execFunc:    regDump,
		respFile:    "response",
		respPattern: "-- End of Response --",
	},
	"mm_read": {
		execFunc:    mmRead,
		respFile:    "response",
		respPattern: "-- End of Response --",
	},
	"device_data": {
		execFunc:    deviceData,
		respFile:    "response",
		respPattern: "-- End of Response --",
	},
}

// pf_bb_config_cli command IDs
var (
	RESET_MODE_CMD_ID  = []byte{0x2, 0x0}
	AUTO_RESET_CMD_ID  = []byte{0x3, 0x0}
	CLEAR_LOG_CMD_ID   = []byte{0x4, 0x0}
	REG_DUMP_CMD_ID    = []byte{0x6, 0x0}
	MM_READ_CMD_ID     = []byte{0x8, 0x0}
	DEVICE_DATA_CMD_ID = []byte{0x9, 0x0}
)

// CLI command arguments
var (
	CLUSTER_RESET_MODE = []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	PF_FLR_MODE        = []byte{0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0}
	AUTO_RESET_OFF     = []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	AUTO_RESET_ON      = []byte{0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0}
	MM_READ_REG_READ   = []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	VOID_PRIVATE       = []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
)

func ShowHelp() {
	fmt.Println("Usage: ./sriov_fec_daemon -C <command_type> [additional_arguments]")
	fmt.Println("Supported <commands>")
	fmt.Println("\treset_mode <pf_flr|cluster_reset>")
	fmt.Println("\tauto_reset <on|off>")
	fmt.Println("\tclear_log")
	fmt.Println("\treg_dump")
	fmt.Println("\tmm_read <reg_addr>")
	fmt.Println("\tdevice_data")
}

func sendCmd(pciAddr string, cmd []byte, log *logrus.Logger) error {
	conn, err := net.Dial("unix", fmt.Sprintf("/tmp/pf_bb_config.%v.sock", pciAddr))
	if err != nil {
		log.WithField("pciAddr", pciAddr).WithError(err).Error("failed to open socket")
		return err
	}
	defer conn.Close()
	// Clear response log before sending CLI command
	err = clearResponseLog(pciAddr)
	if err != nil {
		log.WithError(err).WithField("pciAddr", pciAddr).Error("error occurred while clearing response log")
		return err
	}
	err = conn.SetWriteDeadline(time.Now().Add(time.Second))
	if err != nil {
		log.WithField("pciAddr", pciAddr).WithError(err).Error("failed to set timeout for request")
		return err
	}
	// Send CLI command
	_, err = conn.Write(cmd)
	if err != nil {
		log.WithField("pciAddr", pciAddr).WithError(err).Error("failed to send request to socket")
		return err
	}
	return nil
}

func resetModeHelp() {
	fmt.Println("Help for reset_mode command:")
	fmt.Println("Valid modes: pf_flr|cluster_reset")
	fmt.Println("\tpf_flr: Physical Function Function Level Reset")
	fmt.Println("\tcluster_reset: Cluster Reset")
}

func resetModeParse(args []string) ([]byte, error) {
	if len(args) == 0 {
		fmt.Println("error: missing argument for command reset_mode")
		resetModeHelp()
		return nil, errors.New("error: missing argument for command reset_mode")
	}
	if args[0] == "pf_flr" {
		return PF_FLR_MODE, nil
	} else if args[0] == "cluster_reset" {
		return CLUSTER_RESET_MODE, nil
	} else {
		fmt.Println("error: invalid reset_mode value")
		resetModeHelp()
		return nil, errors.New("error: invalid reset_mode value")
	}
}

func resetMode(args []string) ([]byte, error) {
	resetModeBytes, err := resetModeParse(args)
	if err != nil {
		return nil, err
	}
	request := append([]byte(RESET_MODE_CMD_ID), 0x8, 0x0) // short id, short len
	request = append(request, VOID_PRIVATE...)   // void *priv;
	request = append(request, resetModeBytes...) // unsigned int mode;
	return request, nil
}

func autoResetHelp() {
	fmt.Println("Help for auto_reset command:")
	fmt.Println("Valid modes: on|off")
	fmt.Println("\ton: Device status will be logged and will do reset procedure")
	fmt.Println("\toff: Device status will be logged, no reset")
}

func autoResetParse(args []string) ([]byte, error) {
	if len(args) == 0 {
		fmt.Println("error: missing argument for command auto_reset")
		autoResetHelp()
		return nil, errors.New("error: missing argument for command auto_reset")
	}
	if args[0] == "on" {
		return AUTO_RESET_ON, nil
	} else if args[0] == "off" {
		return AUTO_RESET_OFF, nil
	} else {
		fmt.Println("error: invalid auto_reset value")
		autoResetHelp()
		return nil, errors.New("error: invalid auto_reset value")
	}
}

func autoReset(args []string) ([]byte, error) {
	autoResetBytes, err := autoResetParse(args)
	if err != nil {
		return nil, err
	}
	request := append([]byte(AUTO_RESET_CMD_ID), 0x8, 0x0)  // short id, short len;
	request = append(request, VOID_PRIVATE...)   // void *priv;
	request = append(request, autoResetBytes...) // unsigned int mode;
	return request, nil
}

func clearLogCli(args []string) ([]byte, error) {
	request := append([]byte(CLEAR_LOG_CMD_ID), 0x0, 0x0) // short id, short len;
	request = append(request, VOID_PRIVATE...) // void *priv;
	return request, nil
}

func regDump(args []string) ([]byte, error) {
	request := append([]byte(REG_DUMP_CMD_ID), 0x8, 0x0)  // short id, short len;
	request = append(request, VOID_PRIVATE...) // void *priv;

	if len(args) < 1 {
		fmt.Println("error: missing argument for reg_dump")
		return nil, errors.New("error: missing argument for reg_dump")
	}

	if device, exists := fecDevices[strings.ToLower(args[0])]; exists {
		request = append(request, device...) // unsigned int device_id
	} else {
		fmt.Println("error: invalid device for reg_dump")
		return nil, errors.New("error: invalid device for reg_dump")
	}
	return request, nil
}

func mmReadHelp() {
	fmt.Println("Help for mm_read command:")
	fmt.Println("mm_read <reg_addr>")
	fmt.Println("\tRegister address must be in hex 0x format")
}

func mmReadParse(args []string) ([]byte, error) {
	if len(args) < 1 {
		fmt.Println("error: missing register address for mm_read")
		mmReadHelp()
		return nil, errors.New("error: missing register address for mm_read")
	}
	if len(args[0]) < 3 {
		fmt.Println("error: invalid input for register address")
		mmReadHelp()
		return nil, errors.New("error: invalid input for register address")
	}
	if args[0][0] != '0' || args[0][1] != 'x' {
		fmt.Println("error: dump address must be HEX")
		mmReadHelp()
		return nil, errors.New("error: dump address must be HEX")
	}

	regAddrUint, err := strconv.ParseUint(args[0], 0, 32)
	regAddrBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(regAddrBytes, uint32(regAddrUint))
	if err != nil {
		fmt.Println("error: ", err)
		return nil, errors.New("error: failed to convert address string to uint")
	}
	return regAddrBytes, nil
}

func mmRead(args []string) ([]byte, error) {
	regAddrBytes, err := mmReadParse(args)
	if err != nil {
		return nil, err
	}
	request := append([]byte(MM_READ_CMD_ID), 0x20, 0x0)  // short id, short len;
	request = append(request, VOID_PRIVATE...)     // void *priv;
	request = append(request, MM_READ_REG_READ...) // unsigned int reg_op_flag;
	request = append(request, regAddrBytes...)     // unsigned int reg_rw_address;
	return request, nil
}

func deviceData(args []string) ([]byte, error) {
	request := append([]byte(DEVICE_DATA_CMD_ID), 0x0, 0x0) // short id, short len;
	request = append(request, VOID_PRIVATE...) // void *priv;
	return request, nil
}

func clearResponseLog(pciAddr string) error {
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

func pollLogFile(pciAddr string, filePath string, searchStr string, log *logrus.Logger) ([]byte, error) {
	var file []byte
	var fileContent string
	err := wait.Poll(time.Millisecond*50, time.Second, func() (done bool, err error) {
		file, err = os.ReadFile(filePath)
		if err != nil {
			log.WithField("pciAddr", pciAddr).WithError(err).Warnf("failed to read log file")
			return false, err
		}
		if searchStr == "" { // Poll until file is empty
			return len(file) == 0, nil
		} else { // Poll until searchStr is found
			fileContent = string(file)
			lines := strings.Split(fileContent, "\n")
			// Get the last 5 lines
			start := len(lines) - 5
			if start < 0 {
				start = 0
			}
			last5Lines := lines[start:]
			// Check if searchStr is present in any of the last 5 lines
			for _, line := range last5Lines {
				if strings.Contains(line, searchStr) {
					return true, nil
				}
			}
		}
		return false, nil
	})
	if err != nil {
		log.WithField("pciAddr", pciAddr).WithError(err).Error("timed out reading response log")
		return nil, err
	}
	return file, nil
}

func StartPfBbConfigCli(nodeName string, ns string, directClient client.Client, cmd string, args []string, log *logrus.Logger) {
	nodeConfig := &fec.SriovFecNodeConfig{}
	err := directClient.Get(context.Background(), client.ObjectKey{Name: nodeName, Namespace: ns}, nodeConfig)
	if err != nil {
		log.WithError(err).WithField("nodeName", nodeName).WithField("namespace", ns).Error("failed to get SriovFecNodeConfig to run CLI command")
		return
	}
	if cmdInfo, exists := cliCommands[cmd]; exists {
		var deviceID string
		var pfPciAddr string
		var pfBbConfigLog string
		for _, acc := range nodeConfig.Status.Inventory.SriovAccelerators {
			if strings.EqualFold(acc.PFDriver, utils.VFIO_PCI) {
				deviceID = acc.DeviceID
				pfPciAddr = acc.PCIAddress
				break
			}
		}
		if cmd == "reg_dump" {
			args = append(args, deviceID)
		}
		request, err := cmdInfo.execFunc(args)
		if err != nil {
			log.WithError(err).WithField("nodeName", nodeName).WithField("namespace", ns).WithField("pciAddr", pfPciAddr).Error(err)
			fmt.Println("Usage: ./sriov_fec_daemon -C <command_type> [additional_arguments]")
			return
		}
		if cmdInfo.respFile == "main" {
			pfBbConfigLog = fmt.Sprintf("/var/log/pf_bb_cfg_%v.log", pfPciAddr)
		} else {
			pfBbConfigLog = fmt.Sprintf("/var/log/pf_bb_cfg_%v_response.log", pfPciAddr)
		}

		err = sendCmd(pfPciAddr, request, log)
		if err != nil {
			log.WithError(err).WithField("nodeName", nodeName).WithField("namespace", ns).WithField("pciAddr", pfPciAddr).Error(err)
			return
		}

		file, err := pollLogFile(pfPciAddr, pfBbConfigLog, cmdInfo.respPattern, log)
		if err != nil {
			log.WithField("pciAddr", pfPciAddr).WithError(err).Error("failed to read response log")
			return
		}
		fmt.Printf("%s", file)
		return
	}
	log.WithField("nodeName", nodeName).WithField("namespace", ns).Error("invalid CLI command")
	ShowHelp()
}
