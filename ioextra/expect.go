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

package ioextra

import (
	"bufio"
	"bytes"
	"io"
)

// ExpectEngine allow to look for patterns in stream input.
type ExpectEngine struct {
	reader *bufio.Reader
}

// NewExpectEngine returns an expect engine reading from a given reader.
func NewExpectEngine(reader io.Reader) *ExpectEngine {
	return &ExpectEngine{
		reader: bufio.NewReader(reader),
	}
}

// CollectUntil buffers and returns data read until the expected bytes arrive.
//
// The return value does not repeat the expected bytes.
func (expect *ExpectEngine) CollectUntil(expected []byte) ([]byte, error) {
	var buf bytes.Buffer
	i := 0
	for {
		if i == len(expected) {
			collected := buf.Bytes()
			return collected[:len(collected)-len(expected)], nil
		}
		b, err := expect.reader.ReadByte()
		if err != nil {
			return nil, err
		}
		buf.WriteByte(b) // error is always nil
		if i < len(expected) && expected[i] == b {
			i++
		} else if i > 0 {
			i = 0
			expect.reader.UnreadByte()
		}
	}
}

// DiscardUntil skips data read until the expected bytes arrive.
func (expect *ExpectEngine) DiscardUntil(expected []byte) error {
	i := 0
	for {
		if i == len(expected) {
			return nil
		}
		b, err := expect.reader.ReadByte()
		if err != nil {
			return err
		}
		if i < len(expected) && expected[i] == b {
			i++
		} else if i > 0 {
			i = 0
			expect.reader.UnreadByte()
		}
	}
}
