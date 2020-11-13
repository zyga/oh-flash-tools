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
	"errors"
	"io"
	"syscall"
)

type eintr struct {
	wrapped io.ReadWriteCloser
}

// NewRestartingReadWriteCloser provides a ReadWriteCloser handling EINTR.
//
// This type helps with https://github.com/golang/go/issues/38033
func NewRestartingReadWriteCloser(wrapped io.ReadWriteCloser) io.ReadWriteCloser {
	return &eintr{wrapped: wrapped}
}

// Read reads data from the underlying stream.
//
// If an interrupt occurs before any data is read then the system call is
// automatically restarted.
func (rwc *eintr) Read(p []byte) (n int, err error) {
again:
	n, err = rwc.wrapped.Read(p)
	if errors.Is(err, syscall.EINTR) {
		err = nil
		if n == 0 {
			goto again
		}
	}
	return n, err
}

// Write writes data to the underlying stream.
//
// If an interrupt occurs before any data is written then the system call is
// automatically restarted.
func (rwc *eintr) Write(p []byte) (n int, err error) {
again:
	n, err = rwc.wrapped.Write(p)
	if errors.Is(err, syscall.EINTR) {
		err = nil
		if n == 0 {
			goto again
		}
	}
	return n, err
}

// Close closes the wrapped stream.
func (rwc *eintr) Close() error {
	return rwc.wrapped.Close()
}
