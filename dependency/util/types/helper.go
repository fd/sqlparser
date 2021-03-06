// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"math"

	"github.com/juju/errors"
	"github.com/fd/sqlparser/dependency/mysql"
)

// RoundFloat rounds float val to the nearest integer value with float64 format, like MySQL Round function.
// RoundFloat uses default rounding mode, see https://dev.mysql.com/doc/refman/5.7/en/precision-math-rounding.html
// so rounding use "round half away from zero".
// e.g, 1.5 -> 2, -1.5 -> -2.
func RoundFloat(f float64) float64 {
	if math.Abs(f) < 0.5 {
		return 0
	}

	return math.Trunc(f + math.Copysign(0.5, f))
}

// Round rounds the argument f to dec decimal places.
// dec defaults to 0 if not specified. dec can be negative
// to cause dec digits left of the decimal point of the
// value f to become zero.
func Round(f float64, dec int) float64 {
	shift := math.Pow10(dec)
	f = f * shift
	f = RoundFloat(f)
	return f / shift
}

func getMaxFloat(flen int, decimal int) float64 {
	intPartLen := flen - decimal
	f := math.Pow10(intPartLen)
	f -= math.Pow10(-decimal)
	return f
}

func truncateFloat(f float64, decimal int) float64 {
	pow := math.Pow10(decimal)
	t := (f - math.Floor(f)) * pow

	round := RoundFloat(t)

	f = math.Floor(f) + round/pow
	return f
}

// TruncateFloat tries to truncate f.
// If the result exceeds the max/min float that flen/decimal allowed, returns the max/min float allowed.
func TruncateFloat(f float64, flen int, decimal int) (float64, error) {
	if math.IsNaN(f) {
		// nan returns 0
		return 0, nil
	}

	maxF := getMaxFloat(flen, decimal)

	if !math.IsInf(f, 0) {
		f = truncateFloat(f, decimal)
	}

	if f > maxF {
		f = maxF
	} else if f < -maxF {
		f = -maxF
	}

	return f, nil
}

// CalculateSum adds v to sum.
func CalculateSum(sum Datum, v Datum) (Datum, error) {
	// for avg and sum calculation
	// avg and sum use decimal for integer and decimal type, use float for others
	// see https://dev.mysql.com/doc/refman/5.7/en/group-by-functions.html
	var (
		data Datum
		err  error
	)

	switch v.Kind() {
	case KindNull:
	case KindInt64, KindUint64:
		var d *mysql.MyDecimal
		d, err = v.ToDecimal()
		if err == nil {
			data = NewDecimalDatum(d)
		}
	case KindMysqlDecimal:
		data = v
	default:
		var f float64
		f, err = v.ToFloat64()
		if err == nil {
			data = NewFloat64Datum(f)
		}
	}

	if err != nil {
		return data, errors.Trace(err)
	}
	if data.IsNull() {
		return sum, nil
	}
	switch sum.Kind() {
	case KindNull:
		return data, nil
	case KindFloat64, KindMysqlDecimal:
		return ComputePlus(sum, data)
	default:
		return data, errors.Errorf("invalid value %v for aggregate", sum.Kind())
	}
}
