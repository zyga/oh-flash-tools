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

func crc16(data []byte) (crc uint16) {
	for _, b := range data {
		for i := byte(0x80); i > 0; i >>= 1 {
			crc = crcUpdate(crc, b&i != 0)
		}
	}
	for i := 0; i < 16; i++ {
		crc = crcUpdate(crc, false)
	}
	return crc
}

func crcUpdate(input uint16, increment bool) (out uint16) {
	const crcPoly = 0x1021
	out = input << 1
	if increment {
		out++
	}
	if input>>15 != 0 {
		out ^= crcPoly
	}
	return out
}
