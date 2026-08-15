// Package util contains small helper functions shared across the Angel One
// broker implementation. It is internal to the angelone broker package.
package util

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"
)

// DateFormat is the layout used for all Angel One historical date range params.
const DateFormat = "2006-01-02 15:04"

// DaysBetween returns the number of calendar days spanned by [from, to],
// rounding up any partial day.
func DaysBetween(from, to time.Time) int {
	hours := to.Sub(from).Hours()
	days := int(hours / 24)
	if hours > float64(days)*24 {
		days++
	}
	return days
}

// JoinErrors joins error strings with "; " for readable merge-failure messages.
func JoinErrors(errs []string) string {
	return strings.Join(errs, "; ")
}

// ParseInt32 parses s as an int32, returning 0 on empty or invalid input.
func ParseInt32(s string) int32 {
	v, _ := strconv.ParseInt(s, 10, 32)
	return int32(v)
}

// ParseFloat64 parses s as a float64, returning 0 on empty or invalid input.
func ParseFloat64(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// ToFloat64OrZero returns val as a float64, or 0 if it isn't one.
func ToFloat64OrZero(val any) float64 {
	v, _ := toFloat64(val)
	return v
}

// ToInt64OrZero returns val as an int64, or 0 if it isn't one.
func ToInt64OrZero(val any) int64 {
	v, _ := toInt64(val)
	return v
}

// OIToInt64 converts an open-interest JSON number into an int64. Angel One
// returns integer-valued floats (e.g. "1.567111E7", "214800.0") that cannot
// be decoded directly into an int64, so this falls back to a float parse and
// rounds to the nearest integer.
func OIToInt64(n json.Number) int64 {
	if n == "" {
		return 0
	}
	if i, err := n.Int64(); err == nil {
		return i
	}
	if f, err := n.Float64(); err == nil {
		return int64(math.Round(f))
	}
	return 0
}

// toFloat64 returns val as a float64.
func toFloat64(val any) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	default:
		return 0, false
	}
}

// toInt64 returns val as an int64.
func toInt64(val any) (int64, bool) {
	switch v := val.(type) {
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}
