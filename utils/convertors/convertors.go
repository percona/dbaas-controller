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
	suffixMapping := map[string]float64{
		"m":  0.001,
		"K":  float64(kiloByte),
		"Ki": float64(kibiByte),
		"M":  float64(megaByte),
		"Mi": float64(mibiByte),
		"G":  float64(gigaByte),
		"Gi": float64(gibiByte),
		"T":  float64(teraByte),
		"Ti": float64(tebiByte),
		"":   1.0,
	}
	coeficient, ok := suffixMapping[suffix]
	if !ok {
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
	if strings.HasSuffix(cpu, "m") {
		cpu = cpu[:len(cpu)-1]
		millis, err := strconv.ParseInt(cpu, 10, 64)
		if err != nil {
			return 0, err
		}
		return millis, nil
	}
	floatCPU, err := strconv.ParseFloat(cpu, 64)
	if err != nil {
		return 0, err
	}
	return int64(floatCPU * 1000), nil
}

// BytesToStr converts integer of bytes to string.
func BytesToStr(i int64) string {
	return strconv.FormatInt(i, 10)
}

// MilliCPUToStr converts integer of milli CPU to string.
func MilliCPUToStr(i int32) string {
	return strconv.FormatInt(int64(i), 10) + "m"
}
