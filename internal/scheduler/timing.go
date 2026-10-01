// Package scheduler decides when runs happen and runs them: one run for
// `synctoceph run`, or one run after another for `synctoceph schedule`.
//
// This file does the timing. Two kinds of schedule exist:
//   - interval: the next run starts a fixed time after the previous run ended;
//   - daily: the next run starts at a local time of day, such as 02:00.
//
// Daily times use the TZ environment variable if set, otherwise the system
// time zone. Around daylight-saving changes, a time that does not exist that
// day (the clocks jump over it) is skipped, and a time that happens twice
// (the clocks go back) runs only once.
package scheduler

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// dailySearchLimit bounds the search for the next daily time: it always
// exists within three days, even around daylight-saving changes.
const dailySearchLimit = 3 * 24 * time.Hour

// Zone returns the time zone for daily schedules: TZ if set, otherwise the
// system time zone. An unknown TZ is an error rather than silently UTC.
func Zone() (*time.Location, error) {
	tz, set := os.LookupEnv("TZ")
	if !set || tz == "" {
		return time.Local, nil
	}
	zone, err := time.LoadLocation(strings.TrimPrefix(tz, ":"))
	if err != nil {
		return nil, ui.BadTimeZone(tz)
	}
	return zone, nil
}

// ZoneName returns a readable name for the time zone used by Zone, such as
// "Europe/London".
func ZoneName() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return strings.TrimPrefix(tz, ":")
	}
	if target, err := os.Readlink("/etc/localtime"); err == nil {
		if i := strings.Index(target, "zoneinfo/"); i >= 0 {
			return target[i+len("zoneinfo/"):]
		}
		return filepath.Base(target)
	}
	if data, err := os.ReadFile("/etc/timezone"); err == nil {
		return strings.TrimSpace(string(data))
	}
	name, _ := time.Now().Zone()
	return name
}

// NextDaily returns the first whole minute after now at which the local time
// in zone is hour:minute, on a local date after lastDate (YYYY-MM-DD, or ""
// for no previous run). Checking minute by minute in UTC handles
// daylight-saving changes naturally: a skipped local time never matches, and
// a repeated one matches twice on the same date, of which lastDate lets only
// the first run.
func NextDaily(now time.Time, hour, minute int, zone *time.Location, lastDate string) (time.Time, error) {
	cursor := now.UTC().Truncate(time.Minute).Add(time.Minute)
	for end := cursor.Add(dailySearchLimit); cursor.Before(end); cursor = cursor.Add(time.Minute) {
		local := cursor.In(zone)
		if local.Hour() == hour && local.Minute() == minute && local.Format(time.DateOnly) > lastDate {
			return cursor, nil
		}
	}
	return time.Time{}, errors.New("could not find the next daily run time")
}

// NextInterval returns when the next interval run is due: interval after the
// previous run ended, or now if there was no previous run or it is overdue.
// Missed runs are not replayed: an overdue schedule runs once.
func NextInterval(now, lastEnd time.Time, interval time.Duration) time.Time {
	if lastEnd.IsZero() {
		return now
	}
	due := lastEnd.Add(interval)
	if due.Before(now) {
		return now
	}
	return due
}
