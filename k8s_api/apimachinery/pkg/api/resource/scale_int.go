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

package resource

import (
	"math"
	"math/big"
	"sync"
)

var (
	// A sync pool to reduce allocation.
	intPool  sync.Pool
	maxInt64 = big.NewInt(math.MaxInt64)
)

func init() {
	intPool.New = func() interface{} {
		return new(big.Int)
	}
}

// scaledValue scales given unscaled value from scale to new Scale and returns
// an int64. It ALWAYS rounds up the result when scale down. The final result might
// overflow.
//
// scale, newScale represents the scale of the unscaled decimal.
// The mathematical value of the decimal is unscaled * 10**(-scale).
func scaledValue(unscaled *big.Int, scale, newScale int) int64 {
	dif := scale - newScale
	if dif == 0 {
		return unscaled.Int64()
	}

	// Handle scale up
	// This is an easy case, we do not need to care about rounding and overflow.
	// If any intermediate operation causes overflow, the result will overflow.
	if dif < 0 {
		return unscaled.Int64() * int64(math.Pow10(-dif))
	}

	// Handle scale down
	// We have to be careful about the intermediate operations.

	// fast path when unscaled < max.Int64 and exp(10,dif) < max.Int64
	const log10MaxInt64 = 19
	if unscaled.Cmp(maxInt64) < 0 && dif < log10MaxInt64 {
		divide := int64(math.Pow10(dif))
		result := unscaled.Int64() / divide
		mod := unscaled.Int64() % divide
		if mod != 0 {
			return result + 1
		}
		return result
	}

	// We should only convert back to int64 when getting the result.
	divisor := intPool.Get().(*big.Int)
	exp := intPool.Get().(*big.Int)
	result := intPool.Get().(*big.Int)
	defer func() {
		intPool.Put(divisor)
		intPool.Put(exp)
		intPool.Put(result)
	}()

	// divisor = 10^(dif)
	// TODO: create loop up table if exp costs too much.
	divisor.Exp(bigTen, exp.SetInt64(int64(dif)), nil)
	// reuse exp
	remainder := exp

	// result = unscaled / divisor
	// remainder = unscaled % divisor
	result.DivMod(unscaled, divisor, remainder)
	if remainder.Sign() != 0 {
		return result.Int64() + 1
	}

	return result.Int64()
}
