// dbaas-controller
// Copyright (C) 2020 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

// Package convertors contains functions to do conversion.
package convertors

import (
	"strconv"
	"strings"
)

// StrToBytes converts string of bytes to integer.
func StrToBytes(s string) int64 {
	multiplier := int64(1)
	if strings.HasSuffix(s, "G") {
		multiplier = 1000 * 1000 * 1000
		s = strings.TrimSuffix(s, "G")
	}
	if b, err := strconv.ParseInt(s, 10, 64); err == nil {
		return b * multiplier
	}
	return 0
}

// StrToMilliCPU converts milli CPU to integer.
func StrToMilliCPU(s string) int32 {
	if !strings.HasSuffix(s, "m") {
		return 0
	}
	s = strings.TrimSuffix(s, "m")
	if b, err := strconv.ParseUint(s, 10, 32); err == nil {
		return int32(b)
	}
	return 0
}

// BytesToStr converts integer of bytes to string.
func BytesToStr(i int64) string {
	return strconv.FormatInt(i, 10)
}

// MilliCPUToStr converts integer of milli CPU to string.
func MilliCPUToStr(i int32) string {
	return strconv.FormatInt(int64(i), 10) + "m"
}
