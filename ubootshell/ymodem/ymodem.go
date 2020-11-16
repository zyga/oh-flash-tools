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

// Package ymodem contains implementation of YMODEM protocol for u-boot.
package ymodem

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Transfer encapsulates state of an ymodem file transfer.
type Transfer struct {
	// file is the file being sent
	file     *os.File
	fileInfo os.FileInfo

	blockKind  BlockKind
	retryCount int
	observer   Observer

	fileBytesSent int64
}

// NewTransfer returns a transfer object for sending the given file.
func NewTransfer(file *os.File) (*Transfer, error) {
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	tr := &Transfer{
		file:     file,
		fileInfo: fileInfo,
	}
	return tr, nil
}

// WithRetryCount returns a transfer with the given number of retry attempts.
func (tr *Transfer) WithRetryCount(retryCount int) *Transfer {
	tr.retryCount = retryCount
	return tr
}

// Observer is the interface for observing file transfers.
type Observer interface {
	Start(file string, size int64)
	Progress(bytesSent, bytesTotal int64)
	Finish()
}

// WithObserver returns a transfer with an observer that is notified of progress.
func (tr *Transfer) WithObserver(observer Observer) *Transfer {
	tr.observer = observer
	return tr
}

// WithBlockKind returns a transfer with a given transfer block kind.
func (tr *Transfer) WithBlockKind(blockKind BlockKind) *Transfer {
	tr.blockKind = blockKind
	return tr
}

// SendTo completes the file transfer using the ymodem protocol.
func (tr *Transfer) SendTo(stream io.ReadWriter) (err error) {
	defer func() {
		// If we fail, tell the other side to abort.
		if err != nil {
			_, _ = stream.Write([]byte{asciiCAN, asciiCAN})
		}
	}()
	errPrefix := "cannot send file"

	if err := tr.sendFileInfo(stream); err != nil {
		return err
	}
	if err := tr.sendFileData(stream); err != nil {
		return err
	}
	// Termination dance.
	if err := writeControlByte(stream, asciiEOT); err != nil {
		return err
	}
	cmd, err := readControlByte(stream)
	if err != nil {
		return err
	}
	if cmd != asciiACK {
		return fmt.Errorf("%s: expected 1st termination ACK, got %q", errPrefix, cmd)
	}
	cmd, err = readControlByte(stream)
	if err != nil {
		return err
	}
	if cmd != asciiACK {
		return fmt.Errorf("%s: expected 2nd termination ACK, got %q", errPrefix, cmd)
	}
	cmd, err = readControlByte(stream)
	if err != nil {
		return err
	}
	if cmd != ymodemPOLL {
		return fmt.Errorf("%s: termination POLL, got %q", errPrefix, cmd)
	}
	// Send empty block to indicate completion.
	if err := sendBlock(stream, tr.blockKind, 0, nil, 0); err != nil {
		return err
	}
	cmd, err = readControlByte(stream)
	if err != nil {
		return err
	}
	if cmd != asciiACK {
		return fmt.Errorf("%s: expected final termination ACK, got %q", errPrefix, cmd)
	}
	return nil
}

func (tr *Transfer) sendFileInfo(stream io.ReadWriter) error {
	errPrefix := "cannot send file info"

	// Wait for the receiver to request a file by sending 'C'
	cmd, err := readControlByte(stream)
	if err != nil {
		return err
	}
	if cmd != ymodemPOLL {
		return fmt.Errorf("%s: expected initial POLL, got %q", errPrefix, cmd)
	}

	// Prepare the block with file name and size.
	infoBlock, err := infoBlockForFile(tr.file)
	if err != nil {
		return err
	}
	// Keep trying, we count retry attempts inside.
	for {
		// Send the initial block with zero-byte padding. This is different from
		// actual data blocks which are padded with 0x1A instead. Both values
		// were determined by scanning USB traffic with Wireshark.
		if err = sendBlock(stream, tr.blockKind, 0, infoBlock, 0); err != nil {
			return err
		}
		// Did the bootloader acknowledge the request?
		cmd, err := readControlByte(stream)
		if err != nil {
			return err
		}
		switch cmd {
		case asciiACK:
			return nil
		case asciiNAK:
			tr.retryCount--
			if tr.retryCount < 0 {
				return fmt.Errorf("%s: too many failed attempts", errPrefix)
			}
		case asciiCAN:
			var tmpBuf [3]byte
			if _, err = stream.Read(tmpBuf[:]); err != nil {
				return err
			}
			return fmt.Errorf("%s: transfer rejected by recepient", errPrefix)
		default:
			return fmt.Errorf("%s: expected ACK, NAK or CAN, got %q", errPrefix, cmd)
		}
	}
	return nil
}

func (tr *Transfer) sendFileData(stream io.ReadWriter) error {
	errPrefix := "cannot send file data"

	// The recepient agreed to the file.
	// Wait until we are asked to send data blocks
	cmd, err := readControlByte(stream)
	if err != nil {
		return err
	}
	if cmd != ymodemPOLL {
		return fmt.Errorf("%s: expected POLL, got %q", errPrefix, cmd)
	}

	// Send the blocks, one by one, until we are done.
	blockSize := tr.blockKind.size()
	fileSize := tr.fileInfo.Size()
	numBlocks := (fileSize + int64(blockSize) - 1) / int64(blockSize)
	if tr.observer != nil {
		tr.observer.Start(tr.file.Name(), tr.fileInfo.Size())
	}
	blockData := make([]byte, tr.blockKind.size())
	for blockIdx := int64(0); blockIdx < numBlocks; blockIdx++ {
		n, err := tr.file.Read(blockData)
		if err != nil && err != io.EOF {
			return err
		}
		// Keep trying, we count retry attempts inside.
		for {
			// Note: ymodem uses 1-based indexing of block numbers. The
			// important property is for those counters to increment (and
			// eventually wrap over). They don't have to be able to cover the
			// whole range of the data that needs sending.
			if err = sendBlock(stream, tr.blockKind, uint8(blockIdx+1), blockData[:n], 0x1A); err != nil {
				return err
			}
			// Wait for the recepient to ack the block. If we didn't succeed, try again.
			cmd, err := readControlByte(stream)
			if err != nil {
				return err
			}
			if cmd != asciiACK {
				tr.retryCount--
				if tr.retryCount == 0 {
					return fmt.Errorf("%s too many failed attempts", errPrefix)
				}
			}
			break
		}
		tr.fileBytesSent += int64(n)
		if tr.observer != nil {
			tr.observer.Progress(tr.fileBytesSent, fileSize)
		}
	}
	if tr.observer != nil {
		tr.observer.Finish()
	}
	return nil
}

// infoBlockForFile returns the 0-index block with file meta-data.
func infoBlockForFile(file *os.File) ([]byte, error) {
	var buf bytes.Buffer
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	buf.WriteString(filepath.Base(file.Name()))
	buf.WriteByte(0x0)
	buf.WriteString(fmt.Sprintf("%d", fileInfo.Size()))
	return buf.Bytes(), nil
}

func sendBlock(stream io.ReadWriter, blockKind BlockKind, blockIdx uint8, data []byte, padding byte) error {
	var buf bytes.Buffer
	blockSize := blockKind.size()
	buf.Grow(blockSize + 5)
	// Start of block frame
	if blockKind == SmallBlock {
		buf.WriteByte(asciiSOH)
	} else {
		buf.WriteByte(asciiSTX)
	}
	// Block index and its complement.
	buf.WriteByte(blockIdx)
	buf.WriteByte(^blockIdx)
	// The actual data
	buf.Write(data)
	// Fill the block with padding bytes.
	for i := len(data); i < blockSize; i++ {
		buf.WriteByte(padding)
	}
	// Compute the crc16 of the data, including any padding we added.
	crc := crc16(buf.Bytes()[3:])
	buf.Write([]byte{uint8(crc >> 8)})
	buf.Write([]byte{uint8(crc & 0x0FF)})
	frame := buf.Bytes()
	// Send the block.
	for sent := 0; sent < len(frame); {
		n, err := stream.Write(frame[sent:])
		sent += n
		if err != nil {
			return fmt.Errorf("cannot send block %d: %w", blockIdx, err)
		}
	}
	return nil
}
