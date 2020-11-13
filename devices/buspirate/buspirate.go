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

// Package buspirate contains utilities for working with BusPirate.
package buspirate

import (
	"fmt"
	"io"

	"github.com/zyga/oh-flash-tools/ioextra"

	"go.bug.st/serial.v1"
	"go.bug.st/serial.v1/enumerator"
)

// FindBusPirate finds serial port corresponding to the only bus pirate attached to the system.
func FindBusPirate(portInfos []*enumerator.PortDetails) (string, error) {
	names := make([]string, 0, 1)
	for _, portInfo := range portInfos {
		// TODO: add a way to pass serial number as a hint.
		if portInfo.IsUSB && portInfo.VID == "0403" && portInfo.PID == "6001" {
			names = append(names, portInfo.Name)
		}
	}
	if len(names) != 1 {
		return "", fmt.Errorf("cannot find bus pirate serial port, found %d candidates", len(names))
	}
	return names[0], nil
}

// BusPirate provides interaction with the BusPirate v3 board.
type BusPirate struct {
	stream io.ReadWriteCloser
	expect *ioextra.ExpectEngine
}

// OpenBusPirate opens a BusPirate on a specific serial port name.
func OpenBusPirate(serialPortName string) (*BusPirate, error) {
	port, err := serial.Open(serialPortName, &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		return nil, err
	}
	stream := ioextra.NewRestartingReadWriteCloser(port)
	pirate := &BusPirate{
		stream: stream,
		expect: ioextra.NewExpectEngine(stream),
	}
	return pirate, nil
}

// Close closes the stream representing the bus pirate connection.
func (pirate *BusPirate) Close() error {
	return pirate.stream.Close()
}

// EnterPSUMode resets the bus pirate and enters 1-WIRE mode.
// In this mode the 5V and 3V pins can supply up to 150mA of current.
func (pirate *BusPirate) EnterPSUMode() error {
	if _, err := pirate.stream.Write([]byte("#\n")); err != nil {
		return err
	}
	if err := pirate.expect.DiscardUntil([]byte("HiZ>")); err != nil {
		return err
	}
	if _, err := pirate.stream.Write([]byte("m2\n")); err != nil {
		return err
	}
	return pirate.expect.DiscardUntil([]byte("Ready\r\n"))
}

// EnablePower enables the on-board 5V and 3V power supplies.
func (pirate *BusPirate) EnablePower() error {
	if _, err := pirate.stream.Write([]byte("W\n")); err != nil {
		return err
	}
	return pirate.expect.DiscardUntil([]byte("1-WIRE>"))
}

// DisablePower disables the on-board 5V and 3V power supplies.
func (pirate *BusPirate) DisablePower() error {
	if _, err := pirate.stream.Write([]byte("w\n")); err != nil {
		return err
	}
	return pirate.expect.DiscardUntil([]byte("1-WIRE>"))
}
