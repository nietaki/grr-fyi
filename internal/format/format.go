// Package format provides human-readable rendering of the numeric values
// used across templates, backed by github.com/dustin/go-humanize.
//
// Sentinel convention: a value of 0 for Bytes (and non-finite values for
// Number) renders as an em dash, signalling "not available" rather than a
// real measurement (e.g. RSS is unmeasurable off-Linux).
package format

import (
	"fmt"
	"math"
	"time"

	"github.com/dustin/go-humanize"
)

// Bytes renders a byte count with SI suffixes (KB, MB, GB, ...).
// Zero or negative input renders as an em dash.
func Bytes(b int64) string {
	if b <= 0 {
		return "\u2014"
	}
	return humanize.Bytes(uint64(b))
}

// Count renders an integer with thousands separators.
func Count(n int64) string {
	return humanize.Comma(n)
}

// Number renders a float with thousands separators and one decimal place.
// NaN and infinities render as an em dash.
func Number(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "\u2014"
	}
	return humanize.CommafWithDigits(f, 1)
}

// SecondsToDuration converts a float number of seconds to a rounded
// time.Duration, for rendering durations from fractional stats values.
func SecondsToDuration(seconds float64) time.Duration {
	return time.Duration(math.Round(seconds)) * time.Second
}

// Uptime renders a duration as "Nd Nh Nm" style uptime text.
func Uptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	days := int64(d.Hours()) / 24
	hours := int64(d.Hours()) % 24
	minutes := int64(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
