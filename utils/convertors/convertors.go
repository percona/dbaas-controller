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
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/pkg/errors"
)

const (
	kiloByte int64 = 1000
	kibiByte int64 = 1024
	megaByte int64 = kiloByte * 1000
	mibiByte int64 = kibiByte * 1024
	gigaByte int64 = megaByte * 1000
	gibiByte int64 = mibiByte * 1024
	teraByte int64 = gigaByte * 1000
	tebiByte int64 = gibiByte * 1024
)

// StrToBytes converts string containing memory as string to number of bytes the string represents.
func StrToBytes(memory string) (int64, error) {
	if len(memory) == 0 {
		return 0, errors.New("can't convert an empty string to a number")
	}
	i := len(memory) - 1
	for i >= 0 && !unicode.IsDigit(rune(memory[i])) {
		i--
	}
	var suffix string
	if i >= 0 {
		suffix = memory[i+1:]
	}
	var coeficient float64
	switch suffix {
	case "m":
		coeficient = 0.001
	case "K":
		coeficient = float64(kiloByte)
	case "Ki":
		coeficient = float64(kibiByte)
	case "M":
		coeficient = float64(megaByte)
	case "Mi":
		coeficient = float64(mibiByte)
	case "G":
		coeficient = float64(gigaByte)
	case "Gi":
		coeficient = float64(gibiByte)
	case "T":
		coeficient = float64(teraByte)
	case "Ti":
		coeficient = float64(tebiByte)
	case "":
		coeficient = 1.0
	default:
		return 0, errors.Errorf("suffix '%s' not supported", suffix)
	}

	if suffix != "" {
		memory = memory[:i+1]
	}
	value, err := strconv.ParseFloat(memory, 64)
	if err != nil {
		return 0, errors.Errorf("given value '%s' is not a number", memory)
	}
	return int64(math.Ceil(value * coeficient)), nil
}

// StrToMilliCPU converts CPU as a string representation to millicpus represented as an integer.
func StrToMilliCPU(cpu string) (int64, error) {
	var millis int64
	if strings.HasSuffix(cpu, "m") {
		cpu = cpu[:len(cpu)-1]
		millis, err := strconv.ParseInt(cpu, 10, 64)
		if err != nil {
			return 0, err
		}
		return millis, nil
	}
	if strings.Contains(cpu, ".") {
		parts := strings.Split(cpu, ".")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return 0, errors.Errorf("incorrect CPU value '%s', both decimal and integer parts have to be present", cpu)
		}
		cpu = parts[0]
		var significance int64 = 100
		for _, decimal := range parts[1] {
			decimalInteger, err := strconv.ParseInt(string(decimal), 10, 64)
			if err != nil {
				return 0, err
			}
			millis += decimalInteger * significance
			significance /= 10
		}
	}
	wholeCPUs, err := strconv.ParseInt(cpu, 10, 64)
	if err != nil {
		return 0, err
	}
	millis += wholeCPUs * 1000
	return millis, nil
}

// BytesToStr converts integer of bytes to string.
func BytesToStr(i int64) string {
	return strconv.FormatInt(i, 10)
}

// MilliCPUToStr converts integer of milli CPU to string.
func MilliCPUToStr(i int32) string {
	return strconv.FormatInt(int64(i), 10) + "m"
}
