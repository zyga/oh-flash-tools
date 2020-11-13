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
	"bytes"
	"fmt"
	"io"
)

// IOPreview is a ReadWriteCloser which previews serial I/O in a readable manner
type IOPreview struct {
	wrapped    io.ReadWriteCloser
	inDisplay  bytes.Buffer
	outDisplay bytes.Buffer
	inPrompt   string
	outPrompt  string
	disabled   bool
	immediate  bool
}

// NewIOPreview returns a ReadWriteCloser that shows serial port traffic.
func NewIOPreview(wrapped io.ReadWriteCloser) *IOPreview {
	return &IOPreview{
		wrapped:   wrapped,
		inPrompt:  "   <<<",
		outPrompt: "   >>>",
	}
}

// DisablePreview disables buffering and display of transmitted data.
func (preview *IOPreview) DisablePreview() {
	preview.disabled = true
}

// EnablePreview enables buffering and display of transmitted data.
func (preview *IOPreview) EnablePreview() {
	preview.disabled = false
}

// DisableLineBuffering disables internal buffering of complete lines.
//
// Buffering only affects the preview stream, not the real IO.
func (preview *IOPreview) DisableLineBuffering() {
	preview.immediate = true
}

// EnableLineBuffering enables internal buffering of complete lines.
//
// Buffering only affects the preview stream, not the real IO.
func (preview *IOPreview) EnableLineBuffering() {
	preview.immediate = false
}

func (preview *IOPreview) Read(p []byte) (n int, err error) {
	n, err = preview.wrapped.Read(p)
	// fmt.Printf("read %d bytes: %q\n", n, p[:n])
	if n > 0 && !preview.disabled {
		preview.inDisplay.Write(p[:n]) // buffer writes panic on failure
		display(&preview.inDisplay, preview.inPrompt, preview.immediate)
	}
	return n, err
}

func (preview *IOPreview) Write(p []byte) (n int, err error) {
	n, err = preview.wrapped.Write(p)
	// fmt.Printf("wrote %d bytes: %q\n", n, p[:n])
	if n > 0 && !preview.disabled {
		preview.outDisplay.Write(p[:n]) // buffer writes panic on failure
		display(&preview.outDisplay, preview.outPrompt, preview.immediate)
	}
	return n, err
}

// Close displays the remainder of the buffered communication, even if unterminated.
//
// Close implements io.Closer
func (preview *IOPreview) Close() error {
	display(&preview.outDisplay, preview.outPrompt, true)
	preview.outDisplay.Reset()
	display(&preview.inDisplay, preview.inPrompt, true)
	preview.inDisplay.Reset()
	return nil
}

func display(buf *bytes.Buffer, prompt string, immediate bool) {
	if immediate {
		blob := buf.Bytes()
		fmt.Printf("%s % #x\n", prompt, blob)
		buf.Reset()
	} else {
		var line []byte
		var err error
		for {
			line, err = buf.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Printf("display error: %v\n", err)
				}
				break
			}
			fmt.Printf("%s %q\n", prompt, line)
		}
		if len(line) > 0 {
			// In non-immediate mode buffer it for the next time.
			*buf = *bytes.NewBuffer(line)
		}
	}
}
