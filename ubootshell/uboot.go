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

package ubootshell

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/zyga/oh-flash-tools/ioextra"
	"github.com/zyga/oh-flash-tools/ubootshell/ymodem"
)

// UBootShell allow interaction with u-boot shell environment.
type UBootShell struct {
	rwc    io.ReadWriteCloser
	reader *bufio.Reader
	writer *bufio.Writer
	prompt []byte // prompt of a particular build
}

// NewUBootShell returns an UBootShell over the given serial port.
// The given context can be used to control maximum duration of the negotiation process.
func NewUBootShell(ctx context.Context, rwc io.ReadWriteCloser) *UBootShell {
	return &UBootShell{
		rwc:    rwc,
		reader: bufio.NewReader(rwc),
		writer: bufio.NewWriter(rwc),
	}
}

// InterruptBoot waits for the message "Hit any key to stop autoboot" and sends a newline.
func (uboot *UBootShell) InterruptBoot() error {
	fmt.Printf("Waiting for u-boot auto-boot prompt\n")

	// Scan input until u-boot announces auto-boot.
	if err := uboot.discardUntil([]byte("Hit any key to stop autoboot")); err != nil {
		return fmt.Errorf("cannot find u-boot autoboot message")
	}
	fmt.Printf("Interrupting Boot Process\n")

	// Interrupt auto-boot process.
	if _, err := fmt.Fprintf(uboot.writer, "\n"); err != nil {
		return err
	}
	if err := uboot.writer.Flush(); err != nil {
		return err
	}
	// Wait until we get a shell prompt. Note that u-boot is still writing
	// auto-boot counter, so we need to completely drain that before starting
	// prompt detection.
	if err := uboot.discardUntil([]byte("\n")); err != nil {
		return fmt.Errorf("cannot find u-boot shell prompt")
	}
	return nil
}

// ProbePrompt probes u-boot shell prompt.
func (uboot *UBootShell) ProbePrompt() error {
	fmt.Printf("Sending newline to see u-boot prompt\n")
	var prompt []byte
	for i := 0; i < 3; i++ {
		// Send a newline and detect the complete prompt.
		if _, err := fmt.Fprintf(uboot.writer, "\n"); err != nil {
			return err
		}
		if err := uboot.writer.Flush(); err != nil {
			return err
		}
		line, err := uboot.reader.ReadBytes('\n')
		if err != nil {
			return err
		}
		prompt = bytes.TrimRight(line, "\r\n")
		if len(prompt) != 0 {
			break
		}
	}
	if len(prompt) == 0 {
		return fmt.Errorf("cannot auto-discover u-boot prompt")
	}
	fmt.Printf("Auto-discovered u-boot prompt as %q\n", prompt)
	uboot.prompt = prompt
	return nil
}

// Command sends the given text to u-boot prompt.
func (uboot *UBootShell) Command(cmd string) (string, error) {
	return uboot.regularCmd(cmd)
}

// WaitForPrompt discards output until prompt re-appears.
func (uboot *UBootShell) WaitForPrompt() error {
	return uboot.discardUntil(uboot.prompt)
}

// SpecialCommand sends the given text to u-boot prompt and waits for special reponse.
func (uboot *UBootShell) SpecialCommand(cmd, waitFor string) error {
	return uboot.specialCmd(cmd, waitFor)
}

// Reset resets the board.
func (uboot *UBootShell) Reset() error {
	return uboot.specialCmd("reset", "resetting ..")
}

// SetEnv sets u-boot environment variable.
func (uboot *UBootShell) SetEnv(key, value string) error {
	// TODO: escape values correctly
	if _, err := uboot.regularCmd(fmt.Sprintf(`setenv %s "%s"`, key, value)); err != nil {
		return err
	}
	return nil
}

// SaveEnv writes u-boot environment to persistent storage.
func (uboot *UBootShell) SaveEnv() error {
	if _, err := uboot.regularCmd("saveenv"); err != nil {
		return err
	}
	return nil
}

func (uboot *UBootShell) regularCmd(cmd string) (string, error) {
	if len(uboot.prompt) == 0 {
		panic("cannot send command without knowing u-boot prompt")
	}
	fmt.Printf("Execute in uboot: %s\n", cmd)
	if _, err := fmt.Fprintf(uboot.writer, "%s\n", cmd); err != nil {
		return "", err
	}
	if err := uboot.writer.Flush(); err != nil {
		return "", err
	}
	echo := []byte(cmd + "\r\n")
	if err := uboot.discardUntil(echo); err != nil {
		return "", err
	}
	output, err := uboot.collectUntil(uboot.prompt)
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func (uboot *UBootShell) specialCmd(cmd, after string) error {
	if len(uboot.prompt) == 0 {
		panic("cannot send command without knowing u-boot prompt")
	}
	fmt.Printf("Execute in uboot: %s\n", cmd)
	if _, err := fmt.Fprintf(uboot.writer, "%s\n", cmd); err != nil {
		return err
	}
	if err := uboot.writer.Flush(); err != nil {
		return err
	}
	echo := []byte(cmd + "\r\n")
	if err := uboot.discardUntil(echo); err != nil {
		return err
	}
	if err := uboot.discardUntil([]byte(after)); err != nil {
		return err
	}
	return nil
}

func (uboot *UBootShell) collectUntil(expected []byte) ([]byte, error) {
	var buf bytes.Buffer
	i := 0
	for {
		if i == len(expected) {
			collected := buf.Bytes()
			return collected[:len(collected)-len(expected)], nil
		}
		b, err := uboot.reader.ReadByte()
		if err != nil {
			return nil, err
		}
		buf.WriteByte(b) // error is always nil
		if i < len(expected) && expected[i] == b {
			i++
		} else {
			i = 0
		}
	}
}

func (uboot *UBootShell) discardUntil(expected []byte) error {
	i := 0
	for {
		if i == len(expected) {
			return nil
		}
		b, err := uboot.reader.ReadByte()
		if err != nil {
			return err
		}
		if i < len(expected) && expected[i] == b {
			// fmt.Printf("following expected %q\n", b)
			i++
		} else if i > 0 {
			// fmt.Printf("re-setting after unexpected %q\n", b)
			i = 0
			uboot.reader.UnreadByte()
		} else {
			// fmt.Printf("skipping unmatched %q\n", b)
		}
	}
}

// XXX: this belongs in a different layer.
type transferObserver struct{}

func (*transferObserver) Start(name string, size int64) {
	fmt.Printf("Sending file %q (%d bytes)\n", name, size)
}
func (*transferObserver) Progress(bytesSent, bytesTotal int64) {
	fmt.Printf("\x1b[2KSent %d of %d bytes\r", bytesSent, bytesTotal)
}
func (*transferObserver) Finish() {
	fmt.Printf("\n")
}

// SendFile sends a file using the ymodem protocol.
//
// U-boot must be already in an appropriate receive mode. You must use
// SpecialCommand to enter such mode yourself.
func (uboot *UBootShell) SendFile(fileName string) error {
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	tr, err := ymodem.NewTransfer(file)
	if err != nil {
		return err
	}
	tr = tr.WithBlockKind(ymodem.LargeBlock).WithObserver(&transferObserver{}).WithRetryCount(10)
	if preview, ok := uboot.rwc.(*ioextra.IOPreview); ok {
		preview.DisableLineBuffering()
		preview.DisablePreview()
		defer preview.EnableLineBuffering()
		defer preview.EnablePreview()
	}
	if err := tr.SendTo(uboot.rwc); err != nil {
		return err
	}
	if err := uboot.WaitForPrompt(); err != nil {
		return err
	}
	return nil
}
