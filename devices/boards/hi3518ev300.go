/*
Copyright 2020 Huawei Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package boards

import (
	"fmt"
	"io"
	"strings"

	"go.bug.st/serial.v1"
	"go.bug.st/serial.v1/enumerator"

	"github.com/zyga/oh-flash-tools/ioextra"
	"github.com/zyga/oh-flash-tools/openharmony"
	"github.com/zyga/oh-flash-tools/ubootshell"
)

// Hi3518ev300 is a development board for IP Cameras
type Hi3518ev300 struct{}

// FindSerialPort finds a serial port appropriate for interacting with the bootloader.
//
// The adapter bundled with the development kit is a generic Prolific Technology Inc USB to Serial converter
// using USB vendor 0x067b and USB product 0x2303 without a serial number.
func (board *Hi3518ev300) FindSerialPort(portInfos []*enumerator.PortDetails) (string, error) {
	names := make([]string, 0, 1)
	for _, portInfo := range portInfos {
		// XXX: Windows / Linux differ in capitalization of those fields. It's unfortunate they are strings.
		if portInfo.IsUSB && strings.ToLower(portInfo.VID) == "067b" && portInfo.PID == "2303" && portInfo.SerialNumber == "" {
			names = append(names, portInfo.Name)
		}
	}
	if len(names) != 1 {
		return "", fmt.Errorf("cannot find hi3518ev300 serial port, found %d candidates", len(names))
	}
	return names[0], nil
}

// OpenSerialPort opens the given serial port.
func (board *Hi3518ev300) OpenSerialPort(portName string) (io.ReadWriteCloser, error) {
	port, err := serial.Open(portName, &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		return nil, err
	}
	return ioextra.NewRestartingReadWriteCloser(port), nil
}

// FlashAssets flashes an hi3518ev300 board with given assets.
func (board *Hi3518ev300) FlashAssets(uboot *ubootshell.UBootShell, assets *openharmony.Assets) error {
	ver, err := uboot.Command("getinfo version")
	if err != nil {
		return err
	}
	// TODO: validate expected u-boot version.
	fmt.Printf("u-boot version: %q\n", strings.TrimSpace(ver))
	if _, err := uboot.Command("sf probe 0"); err != nil {
		return err
	}
	if err := board.flashBootLoader(uboot, assets.BootLoaderPath); err != nil {
		return err
	}
	if err := board.flashKernel(uboot, assets.KernelPath); err != nil {
		return err
	}
	if err := board.flashRootfs(uboot, assets.RootfsPath); err != nil {
		return err
	}
	if err := board.flashUserfs(uboot, assets.UserfsPath); err != nil {
		return err
	}
	// XXX: should we reboot first that the new uboot has a chance to saveenv?
	if err := board.configureUBoot(uboot); err != nil {
		return err
	}
	if err := uboot.Reset(); err != nil {
		return err
	}
	return nil
}

func (board *Hi3518ev300) configureUBoot(uboot *ubootshell.UBootShell) error {
	const loadAddr = 0x40_000_000 // load everything at this address in memory
	const flashAddr = 0x100_000   // from this address in flash
	const loadSize = 0x600_000    // load exactly this many bytes
	bootcmd := fmt.Sprintf("sf probe 0; sf read %#x %#x %#x; go %#x", loadAddr, flashAddr, loadSize, loadAddr)
	if err := uboot.SetEnv("bootcmd", bootcmd); err != nil {
		return err
	}
	// XXX: those should be related to the constants above
	bootargs := fmt.Sprintf("console=ttyAMA0,115200n8 root=flash fstype=jffs2 rw rootaddr=5M rootsize=7M")
	if err := uboot.SetEnv("bootargs", bootargs); err != nil {
		return err
	}
	if err := uboot.SaveEnv(); err != nil {
		return err
	}
	return nil
}

func (board *Hi3518ev300) flashBootLoader(uboot *ubootshell.UBootShell, bootLoaderPath string) error {
	const flashAddr = 0x0       // at this address
	const eraseSize = 0x100_000 // erase the entire "partition"
	const writeSize = 0x40_000  // and write this amount
	return board.flashAsset(uboot, bootLoaderPath, flashAddr, eraseSize, writeSize)
}

func (board *Hi3518ev300) flashKernel(uboot *ubootshell.UBootShell, kernelPath string) error {
	const flashAddr = 0x100_000 // at this address
	const eraseSize = 0x600_000 // erase the entire "partition"
	const writeSize = 0x3f0_000 // and write this amount
	return board.flashAsset(uboot, kernelPath, flashAddr, eraseSize, writeSize)
}

func (board *Hi3518ev300) flashRootfs(uboot *ubootshell.UBootShell, rootfsPath string) error {
	const flashAddr = 0x700_000 // at this address
	const eraseSize = 0x800_000 // erase the entire "partition"
	const writeSize = 0x670_000 // and write this amount
	return board.flashAsset(uboot, rootfsPath, flashAddr, eraseSize, writeSize)
}

func (board *Hi3518ev300) flashUserfs(uboot *ubootshell.UBootShell, userfsPath string) error {
	const flashAddr = 0xf00_000 // at this address
	const eraseSize = 0x100_000 // erase the entire "partition"
	const writeSize = 0x10_000  // and write this amount
	return board.flashAsset(uboot, userfsPath, flashAddr, eraseSize, writeSize)
}

func (board *Hi3518ev300) flashAsset(uboot *ubootshell.UBootShell, assetPath string, flashAddr, eraseSize, writeSize uint64) error {
	const loadAddr = 0x41_000_000
	// Assets are entirely optional.
	if assetPath == "" {
		return nil
	}

	// Write 0xFF to memory region where we will copy the data
	if _, err := uboot.Command(fmt.Sprintf("mw.b %#x 0xff %#x", loadAddr, writeSize)); err != nil {
		return err
	}
	// Copy the file from local disk to device memory with ymodem
	if err := uboot.SpecialCommand(
		fmt.Sprintf("loady %#x", loadAddr),
		fmt.Sprintf("## Ready for binary (ymodem) download to %#x at 115200 bps...\r\n", loadAddr),
	); err != nil {
		return err
	}
	if err := uboot.SendFile(assetPath); err != nil {
		return err
	}
	// Erase flash memory
	if _, err := uboot.Command(fmt.Sprintf("sf erase %#x %#x", flashAddr, eraseSize)); err != nil {
		return err
	}
	// Program flash memory
	if _, err := uboot.Command(fmt.Sprintf("sf write %#x %#x %#x", loadAddr, flashAddr, writeSize)); err != nil {
		return err
	}
	return nil
}
