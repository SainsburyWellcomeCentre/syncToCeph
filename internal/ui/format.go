// This file turns raw numbers into text people can read at a glance:
// "38.2 GB" instead of 38200000000, "14m 02s" instead of 842 seconds, and
// "1,204" instead of 1204. Every command uses these helpers, so sizes and
// durations look the same everywhere.
package ui

import (
	"fmt"
	"strconv"
	"time"
)

// Size formats a byte count with decimal units (1 KB = 1000 bytes), the same
// units that file managers and storage vendors use.
func Size(bytes int64) string {
	if bytes < 1000 {
		return fmt.Sprintf("%d B", bytes)
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	value := float64(bytes) / 1000
	unit := 0
	for value >= 999.95 && unit < len(units)-1 {
		value /= 1000
		unit++
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}

// Duration formats a length of time with at most two units, for example
// "3s", "10m", "14m 02s", "2h 05m" or "3d 04h".
func Duration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	seconds := int64(d.Round(time.Second) / time.Second)
	days, seconds := seconds/86400, seconds%86400
	hours, seconds := seconds/3600, seconds%3600
	minutes, seconds := seconds/60, seconds%60
	switch {
	case days > 0:
		return twoUnits(days, "d", hours, "h")
	case hours > 0:
		return twoUnits(hours, "h", minutes, "m")
	case minutes > 0:
		return twoUnits(minutes, "m", seconds, "s")
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

// twoUnits writes "14m 02s", or just "10m" when the smaller unit is zero.
func twoUnits(big int64, bigUnit string, small int64, smallUnit string) string {
	if small == 0 {
		return fmt.Sprintf("%d%s", big, bigUnit)
	}
	return fmt.Sprintf("%d%s %02d%s", big, bigUnit, small, smallUnit)
}

// Count formats a whole number with thousands separators, for example "1,204".
func Count(n int) string {
	text := strconv.Itoa(n)
	sign := ""
	if n < 0 {
		sign, text = "-", text[1:]
	}
	for i := len(text) - 3; i > 0; i -= 3 {
		text = text[:i] + "," + text[i:]
	}
	return sign + text
}

// Files returns "1 file" or "N files" with a formatted count.
func Files(n int) string {
	if n == 1 {
		return "1 file"
	}
	return Count(n) + " files"
}

// Time formats a moment in the local time zone in a fixed, sortable form.
// The zero time is shown as "never".
func Time(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Local().Format("2006-01-02 15:04:05 MST")
}

// Ago describes how long ago a moment was, for example "12s ago". Moments in
// the future (possible when clocks disagree) are described as "in 12s".
func Ago(t time.Time, now time.Time) string {
	if t.IsZero() {
		return "never"
	}
	if t.After(now) {
		return "in " + Duration(t.Sub(now))
	}
	return Duration(now.Sub(t)) + " ago"
}
