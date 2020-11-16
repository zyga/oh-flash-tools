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

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"go.bug.st/serial.v1/enumerator"

	"github.com/zyga/oh-flash-tools/devices/boards"
	"github.com/zyga/oh-flash-tools/devices/buspirate"
	"github.com/zyga/oh-flash-tools/ioextra"
	"github.com/zyga/oh-flash-tools/openharmony"
	"github.com/zyga/oh-flash-tools/ubootshell"
)

func run() error {
	var boardType string
	var assets openharmony.Assets
	flag.StringVar(&boardType, "board", "", "Type of the board to program")
	flag.StringVar(&assets.BootLoaderPath, "bootloader", "", "Bootloader image to use")
	flag.StringVar(&assets.KernelPath, "kernel", "", "Kernel image to use")
	flag.StringVar(&assets.RootfsPath, "rootfs", "", "Root file system image to use")
	flag.StringVar(&assets.UserfsPath, "userfs", "", "User file system image to use")
	flag.Parse()

	type flashableBoard interface {
		FindSerialPort(portInfos []*enumerator.PortDetails) (string, error)
		OpenSerialPort(portName string) (io.ReadWriteCloser, error)
		FlashAssets(uboot *ubootshell.UBootShell, assets *openharmony.Assets) error
	}
	var board flashableBoard

	switch boardType {
	case "hi3518ev300":
		board = &boards.Hi3518ev300{}
	case "":
		return fmt.Errorf("select board type with -board")
	default:
		return fmt.Errorf("unsupported board type: %q", boardType)
	}
	// TODO: verify assets before loading.

	portInfos, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return err
	}

	var pirate *buspirate.BusPirate
	fmt.Printf("Looking for bus pirate\n")
	// TODO: make this configurable
	piratePortName, err := buspirate.FindBusPirate(portInfos)
	if err != nil {
		fmt.Printf("%s\n", err)
		fmt.Printf("Flashing process will not be unattended\n")
	} else {
		fmt.Printf("Found bus pirate serial port %s\n", piratePortName)
		pirate, err = buspirate.OpenBusPirate(piratePortName)
		if err != nil {
			return err
		}
		defer func() {
			if err := pirate.Close(); err != nil {
				fmt.Printf("cannot close bus pirate serial port: %s", err)
			}
		}()
		fmt.Printf("Entering PSU mode\n")
		if err := pirate.EnterPSUMode(); err != nil {
			return err
		}
	}

	fmt.Printf("Looking for %s board\n", boardType)
	boardPortName, err := board.FindSerialPort(portInfos)
	if err != nil {
		return err
	}
	fmt.Printf("Found %s serial port %s\n", boardType, boardPortName)
	boardPort, err := board.OpenSerialPort(boardPortName)
	if err != nil {
		return err
	}
	defer func() {
		if err := boardPort.Close(); err != nil {
			fmt.Printf("cannot close board serial port: %s", err)
		}
	}()

	// TODO: make this configurable.
	if true {
		boardPort = ioextra.NewIOPreview(boardPort)
		fmt.Printf("Serial port preview enabled, serial port data displayed as follows:\n")
		fmt.Printf("  <<< incoming serial port data\n")
		fmt.Printf("  >>> outgoing serial port data\n")
	}
	uboot := ubootshell.NewUBootShell(context.TODO(), boardPort)

	if pirate != nil {
		if err := pirate.DisablePower(); err != nil {
			return err
		}
		if err := pirate.EnablePower(); err != nil {
			return err
		}
	} else {
		fmt.Printf("NOTE: power-cycle the board manually now\n")
	}
	if err := uboot.InterruptBoot(); err != nil {
		return err
	}
	if err := uboot.ProbePrompt(); err != nil {
		return err
	}
	return board.FlashAssets(uboot, &assets)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
