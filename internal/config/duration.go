// This file reads the time values used in the config file: durations such as
// "10m", "4h" or "1d", and daily times such as "02:00".
package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// dayPattern matches a whole number of days, such as "1d" or "7d". Go's own
// duration parser does not know days, so they are handled here.
var dayPattern = regexp.MustCompile(`^([0-9]+)d$`)

// ParseDuration reads a duration such as "30s", "10m", "4h", "1h30m" or "1d".
// Negative durations are rejected.
func ParseDuration(text string) (time.Duration, error) {
	text = strings.TrimSpace(text)
	if text == "0" {
		return 0, nil
	}
	if m := dayPattern.FindStringSubmatch(text); m != nil {
		days, err := strconv.Atoi(m[1])
		if err != nil || days > 3650 {
			return 0, fmt.Errorf("%q is too long", text)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(text)
	if err != nil {
		return 0, fmt.Errorf("%q is not a duration such as 30s, 10m, 4h or 1d", text)
	}
	if d < 0 {
		return 0, fmt.Errorf("%q is negative", text)
	}
	return d, nil
}

// FormatDuration writes a duration the way a person would type it in the
// config file, for example "10m" rather than Go's "10m0s".
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	if d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", d/(24*time.Hour))
	}
	text := d.String()
	if strings.HasSuffix(text, "m0s") {
		text = strings.TrimSuffix(text, "0s")
	}
	if strings.HasSuffix(text, "h0m") {
		text = strings.TrimSuffix(text, "0m")
	}
	return text
}

// atPattern matches a 24-hour time of day, HH:MM.
var atPattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)

// ParseAt reads a daily time such as "02:00" and returns the hour and minute.
func ParseAt(text string) (hour, minute int, err error) {
	m := atPattern.FindStringSubmatch(text)
	if m == nil {
		return 0, 0, fmt.Errorf("%q is not a 24-hour time such as 02:00", text)
	}
	hour, _ = strconv.Atoi(m[1])
	minute, _ = strconv.Atoi(m[2])
	return hour, minute, nil
}
