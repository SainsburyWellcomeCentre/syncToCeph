// Tests for schedule timing, including daylight-saving changes.
package scheduler

import (
	"testing"
	"time"
)

func mustZone(t *testing.T, name string) *time.Location {
	t.Helper()
	zone, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("time zone %s not available: %v", name, err)
	}
	return zone
}

func TestNextDaily(t *testing.T) {
	london := mustZone(t, "Europe/London")
	utc := func(y int, mo time.Month, d, h, mi int) time.Time { return time.Date(y, mo, d, h, mi, 0, 0, time.UTC) }
	tests := []struct {
		name     string
		now      time.Time
		at       [2]int
		lastDate string
		want     time.Time
	}{
		{"later today", utc(2026, 6, 1, 0, 0), [2]int{2, 0}, "", utc(2026, 6, 1, 1, 0)},
		{"tomorrow once passed", utc(2026, 6, 1, 3, 0), [2]int{2, 0}, "", utc(2026, 6, 2, 1, 0)},
		{"already ran today", utc(2026, 6, 1, 0, 0), [2]int{2, 0}, "2026-06-01", utc(2026, 6, 2, 1, 0)},
		// Clocks go forward at 01:00 on 29 March 2026: 01:30 does not exist that day.
		{"skipped hour runs nothing that day", utc(2026, 3, 29, 0, 0), [2]int{1, 30}, "", utc(2026, 3, 30, 0, 30)},
		// Clocks go back at 02:00 on 25 October 2026: 01:30 happens twice.
		{"repeated hour: first occurrence", utc(2026, 10, 25, 0, 0), [2]int{1, 30}, "", utc(2026, 10, 25, 0, 30)},
		{"repeated hour runs once", utc(2026, 10, 25, 0, 31), [2]int{1, 30}, "2026-10-25", utc(2026, 10, 26, 1, 30)},
	}
	for _, tt := range tests {
		got, err := NextDaily(tt.now, tt.at[0], tt.at[1], london, tt.lastDate)
		if err != nil || !got.Equal(tt.want) {
			t.Errorf("%s: got %v (%v), want %v", tt.name, got, err, tt.want)
		}
	}
}

func TestNextInterval(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	if got := NextInterval(now, time.Time{}, 4*time.Hour); !got.Equal(now) {
		t.Errorf("first run should be now, got %v", got)
	}
	if got := NextInterval(now, now.Add(-time.Hour), 4*time.Hour); !got.Equal(now.Add(3 * time.Hour)) {
		t.Errorf("interval counts from the end of the last run, got %v", got)
	}
	if got := NextInterval(now, now.Add(-10*time.Hour), 4*time.Hour); !got.Equal(now) {
		t.Errorf("an overdue run happens once, now; got %v", got)
	}
}

func TestZoneRejectsUnknownTZ(t *testing.T) {
	t.Setenv("TZ", "Mars/Olympus_Mons")
	if _, err := Zone(); err == nil {
		t.Fatal("an unknown TZ must be an error, not silently UTC")
	}
	t.Setenv("TZ", "Europe/London")
	if z, err := Zone(); err != nil || z.String() != "Europe/London" {
		t.Fatalf("got %v %v", z, err)
	}
}
