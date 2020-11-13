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

// BlockKind denotes large or small block size.
type BlockKind bool

const (
	// SmallBlock indicates 128 bytes per block.
	SmallBlock BlockKind = false
	// LargeBlock indicates 1024 bytes per block.
	LargeBlock BlockKind = true
)

// String returns a description of the kind of transfer block.
func (b BlockKind) String() string {
	if b == LargeBlock {
		return "large (1024)"
	}
	return "normal (128)"
}

func (b BlockKind) size() int {
	if b == LargeBlock {
		return 1024
	}
	return 128
}
