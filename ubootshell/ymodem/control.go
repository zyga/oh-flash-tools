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

package ymodem

import (
	"fmt"
	"io"
)

type controlByte byte

const (
	asciiSOH   = 0x01
	asciiSTX   = 0x02
	asciiEOT   = 0x04
	asciiACK   = 0x06
	asciiNAK   = 0x15
	asciiCAN   = 0x18
	ymodemPOLL = 0x43
)

// String returns the name of the control byte.
func (b controlByte) String() string {
	switch b {
	case asciiSOH:
		return "SOH"
	case asciiSTX:
		return "STX"
	case asciiEOT:
		return "EOT"
	case asciiACK:
		return "ACK"
	case asciiNAK:
		return "NAK"
	case asciiCAN:
		return "CAN"
	case ymodemPOLL:
		return "POLL"
	default:
		return fmt.Sprintf("%#x", b)
	}
}

func readControlByte(reader io.Reader) (controlByte, error) {
	var buf [1]byte
	var cb controlByte
	_, err := reader.Read(buf[:])
	if err != nil {
		return cb, fmt.Errorf("cannot read control byte: %w", err)
	}
	cb = controlByte(buf[0])
	// fmt.Printf("Read control byte %q\n", cb)
	return cb, nil
}

func writeControlByte(writer io.Writer, cb controlByte) error {
	_, err := writer.Write([]byte{byte(cb)})
	if err != nil {
		return fmt.Errorf("cannot write control byte: %w", err)
	}
	// fmt.Printf("Wrote control byte %q\n", cb)
	return nil
}
