// Package util contains small helper functions shared across the Zerodha
// broker implementation. It is internal to the zerodha broker package.
package util

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DateFormat is the layout used for the zerodha historical date range inputs.
// It matches the broker-agnostic format used across marketconnector.
const DateFormat = "2006-01-02 15:04"

// TimestampFormat is the layout used for candle timestamps in responses.
const TimestampFormat = "2006-01-02 15:04:05"

// ist is the Indian Standard Time zone. Kite Connect expects historical date
// ranges in IST, so inputs are parsed in this zone regardless of the host's
// local timezone.
var ist = time.FixedZone("IST", 5*60*60+30*60)

// ParseTime parses a broker-agnostic date string ("2006-01-02 15:04") as IST.
func ParseTime(s string) (time.Time, error) {
	t, err := time.ParseInLocation(DateFormat, s, ist)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: %w", s, err)
	}
	return t, nil
}

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

// ParseFloat64 parses s as a float64, returning 0 on empty or invalid input.
func ParseFloat64(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
