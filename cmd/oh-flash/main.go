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
	"fmt"
	"os"

	"go.bug.st/serial.v1/enumerator"

	"github.com/zyga/oh-flash-tools/devices/boards"
	"github.com/zyga/oh-flash-tools/devices/buspirate"
	"github.com/zyga/oh-flash-tools/ioextra"
	"github.com/zyga/oh-flash-tools/openharmony"
	"github.com/zyga/oh-flash-tools/ubootshell"
)

func run() error {
	// TODO: make this configurable and verify before loading.
	assets := &openharmony.Assets{
		BootLoaderPath: "u-boot-hi3518ev300.bin",
		KernelPath:     "OHOS_Image.bin",
		RootfsPath:     "rootfs.img",
		UserfsPath:     "userfs.img",
	}

	portInfos, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return err
	}

	fmt.Printf("Looking for bus pirate\n")
	// TODO: make this configurable
	piratePortName, err := buspirate.FindBusPirate(portInfos)
	if err != nil {
		return err
	}
	fmt.Printf("Found bus pirate serial port %s\n", piratePortName)
	pirate, err := buspirate.OpenBusPirate(piratePortName)
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

	// TODO: make this configurable
	board := &boards.Hi3518ev300{}
	fmt.Printf("Looking for hi3518ev300 board\n")
	boardPortName, err := board.FindSerialPort(portInfos)
	if err != nil {
		return err
	}
	fmt.Printf("Found hi3518ev300 serial port %s\n", boardPortName)
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

	if err := pirate.DisablePower(); err != nil {
		return err
	}
	if err := pirate.EnablePower(); err != nil {
		return err
	}
	if err := uboot.InterruptBoot(); err != nil {
		return err
	}
	if err := uboot.ProbePrompt(); err != nil {
		return err
	}
	return board.FlashAssets(uboot, assets)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
