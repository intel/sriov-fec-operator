// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2020-2025 Intel Corporation

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
	vrb "github.com/intel/sriov-fec-operator/api/sriovvrb/v1"
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
	ResetModeCmdID  = []byte{0x2, 0x0}
	AutoResetCmdID  = []byte{0x3, 0x0}
	ClearLogCmdID   = []byte{0x4, 0x0}
	RegDumpCmdID    = []byte{0x6, 0x0}
	MmReadCmdID     = []byte{0x8, 0x0}
	DeviceDataCmdID = []byte{0x9, 0x0}
)

// CLI command arguments
var (
	ClusterResetMode = []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	PfFlrMode        = []byte{0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0}
	AutoResetOff     = []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	AutoResetOn      = []byte{0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0}
	MmReadRegRead    = []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	VoidPrivate      = []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
)

func ShowHelp() {
	fmt.Println("Usage: ./sriov_fec_daemon -C <command_type> [additional_arguments] [-P <pci_address>]")
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

	switch args[0] {
	case "pf_flr":
		return PfFlrMode, nil
	case "cluster_reset":
		return ClusterResetMode, nil
	default:
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
	request := append([]byte(ResetModeCmdID), 0x8, 0x0) // short id, short len
	request = append(request, VoidPrivate...)           // void *priv;
	request = append(request, resetModeBytes...)        // unsigned int mode;
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
	switch args[0] {
	case "on":
		return AutoResetOn, nil
	case "off":
		return AutoResetOff, nil
	default:
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
	request := append([]byte(AutoResetCmdID), 0x8, 0x0) // short id, short len;
	request = append(request, VoidPrivate...)           // void *priv;
	request = append(request, autoResetBytes...)        // unsigned int mode;
	return request, nil
}

func clearLogCli(args []string) ([]byte, error) {
	request := append([]byte(ClearLogCmdID), 0x0, 0x0) // short id, short len;
	request = append(request, VoidPrivate...)          // void *priv;
	return request, nil
}

func regDump(args []string) ([]byte, error) {
	request := append([]byte(RegDumpCmdID), 0x8, 0x0) // short id, short len;
	request = append(request, VoidPrivate...)         // void *priv;

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
	request := append([]byte(MmReadCmdID), 0x20, 0x0) // short id, short len;
	request = append(request, VoidPrivate...)         // void *priv;
	request = append(request, MmReadRegRead...)       // unsigned int reg_op_flag;
	request = append(request, regAddrBytes...)        // unsigned int reg_rw_address;
	return request, nil
}

func deviceData(args []string) ([]byte, error) {
	request := append([]byte(DeviceDataCmdID), 0x0, 0x0) // short id, short len;
	request = append(request, VoidPrivate...)            // void *priv;
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

func StartPfBbConfigCli(nodeName string, ns string, directClient client.Client, cmd string, args []string, pciAddr string, log *logrus.Logger) {
	var deviceID, pfPciAddr string
	var err error

	if pciAddr != "" {
		deviceID, pfPciAddr, err = findDeviceByPciAddr(nodeName, ns, directClient, pciAddr, log)
		if err != nil {
			log.WithError(err).WithField("nodeName", nodeName).WithField("namespace", ns).WithField("pciAddr", pciAddr).Error("no device found")
			return
		}
	} else {
		deviceID, pfPciAddr, err = findDevice(nodeName, ns, directClient)
		if err != nil {
			log.WithError(err).WithField("nodeName", nodeName).WithField("namespace", ns).Error(err)
			return
		}
	}

	if cmd == "reg_dump" {
		args = append(args, deviceID)
	}

	if err := executeCommand(cmd, args, pfPciAddr, log); err != nil {
		log.WithError(err).WithField("nodeName", nodeName).WithField("namespace", ns).WithField("pciAddr", pfPciAddr).Error(err)
		ShowHelp()
		return
	}
}

/******************************************************************************
 * Function: findDevice
 * Description: Searches for a device in the fec.SriovFecNodeConfig configuration.
 *              If a device is found, it returns the deviceID and pfPciAddr.
 *              If no device is found, it calls findVrbDevice to search in the
 *              vrb.SriovVrbNodeConfig configuration.
 *****************************************************************************/
func findDevice(nodeName, ns string, directClient client.Client) (string, string, error) {
	var deviceID, pfPciAddr string

	// Fetch the fec.SriovFecNodeConfig for the given node and namespace
	nodeConfig := &fec.SriovFecNodeConfig{}
	err := directClient.Get(context.Background(), client.ObjectKey{Name: nodeName, Namespace: ns}, nodeConfig)
	if err == nil && len(nodeConfig.Spec.PhysicalFunctions) > 0 {
		// Iterate through the SriovAccelerators in the nodeConfig
		for _, acc := range nodeConfig.Status.Inventory.SriovAccelerators {
			// Check if the deviceID exists in fecDevices and the PFDriver is vfio-pci
			if _, exists := fecDevices[acc.DeviceID]; exists && strings.EqualFold(acc.PFDriver, utils.VfioPci) {
				// If a device is already found, return an error indicating multiple devices found
				if deviceID != "" {
					return "", "", fmt.Errorf("multiple devices found. Please specify PCI address using -P flag")
				} else {
					// Set the deviceID and pfPciAddr
					deviceID = acc.DeviceID
					pfPciAddr = acc.PCIAddress
				}
			}
		}
	}

	// If a device is found, return the deviceID and pfPciAddr
	if deviceID != "" {
		return deviceID, pfPciAddr, nil
	}

	// If no device is found, call findVrbDevice to search in vrb.SriovVrbNodeConfig
	return findVrbDevice(nodeName, ns, directClient)
}

/******************************************************************************
 * Function: findVrbDevice
 * Description: Searches for a device in the vrb.SriovVrbNodeConfig configuration.
 *              If a device is found, it returns the deviceID and pfPciAddr.
 *              If no device is found, it returns an error indicating no device found.
 *****************************************************************************/
func findVrbDevice(nodeName, ns string, directClient client.Client) (string, string, error) {
	var deviceID, pfPciAddr string

	// Fetch the vrb.SriovVrbNodeConfig for the given node and namespace
	vrbNodeConfig := &vrb.SriovVrbNodeConfig{}
	err := directClient.Get(context.Background(), client.ObjectKey{Name: nodeName, Namespace: ns}, vrbNodeConfig)
	if err == nil && len(vrbNodeConfig.Spec.PhysicalFunctions) > 0 {
		// Iterate through the SriovAccelerators in the vrbNodeConfig
		for _, acc := range vrbNodeConfig.Status.Inventory.SriovAccelerators {
			// Check if the deviceID exists in fecDevices and the PFDriver is vfio-pci
			if _, exists := fecDevices[acc.DeviceID]; exists && strings.EqualFold(acc.PFDriver, utils.VfioPci) {
				// If a device is already found, return an error indicating multiple devices found
				if deviceID != "" {
					return "", "", fmt.Errorf("multiple devices found. Please specify PCI address using -P flag")
				} else {
					// Set the deviceID and pfPciAddr
					deviceID = acc.DeviceID
					pfPciAddr = acc.PCIAddress
				}
			}
		}
	}

	// If no device is found, return an error indicating no device found
	if deviceID == "" {
		return "", "", fmt.Errorf("no device found")
	}

	// Return the found deviceID and pfPciAddr
	return deviceID, pfPciAddr, nil
}

func findDeviceByPciAddr(nodeName, ns string, directClient client.Client, pciAddr string, log *logrus.Logger) (string, string, error) {
	nodeConfig := &fec.SriovFecNodeConfig{}
	err := directClient.Get(context.Background(), client.ObjectKey{Name: nodeName, Namespace: ns}, nodeConfig)
	if err != nil {
		log.WithError(err).WithField("nodeName", nodeName).WithField("namespace", ns).Warn("failed to get SriovFecNodeConfig to run CLI command")
	} else {
		for _, acc := range nodeConfig.Status.Inventory.SriovAccelerators {
			if acc.PCIAddress == pciAddr && strings.EqualFold(acc.PFDriver, utils.VfioPci) {
				return acc.DeviceID, acc.PCIAddress, nil
			}
		}
	}

	vrbNodeConfig := &vrb.SriovVrbNodeConfig{}
	err = directClient.Get(context.Background(), client.ObjectKey{Name: nodeName, Namespace: ns}, vrbNodeConfig)
	if err != nil {
		log.WithError(err).WithField("nodeName", nodeName).WithField("namespace", ns).Warn("failed to get SriovVrbNodeConfig to run CLI command")
	} else {
		for _, acc := range vrbNodeConfig.Status.Inventory.SriovAccelerators {
			if acc.PCIAddress == pciAddr && strings.EqualFold(acc.PFDriver, utils.VfioPci) {
				return acc.DeviceID, acc.PCIAddress, nil
			}
		}
	}

	return "", "", fmt.Errorf("no device found with PCI address %s", pciAddr)
}

func executeCommand(cmd string, args []string, pfPciAddr string, log *logrus.Logger) error {
	cmdInfo, exists := cliCommands[cmd]
	if !exists {
		return fmt.Errorf("invalid CLI command")
	}

	request, err := cmdInfo.execFunc(args)
	if err != nil {
		return err
	}

	pfBbConfigLog := getLogFilePath(cmdInfo.respFile, pfPciAddr)
	if err := sendCmd(pfPciAddr, request, log); err != nil {
		return err
	}

	file, err := pollLogFile(pfPciAddr, pfBbConfigLog, cmdInfo.respPattern, log)
	if err != nil {
		log.WithField("pciAddr", pfPciAddr).WithError(err).Error("failed to read response log")
		return err
	}

	fmt.Printf("%s", file)
	return nil
}

func getLogFilePath(respFile, pfPciAddr string) string {
	if respFile == "main" {
		return fmt.Sprintf("/var/log/pf_bb_cfg_%v.log", pfPciAddr)
	}
	return fmt.Sprintf("/var/log/pf_bb_cfg_%v_response.log", pfPciAddr)
}
