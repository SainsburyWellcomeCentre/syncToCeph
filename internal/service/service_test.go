// Tests for the systemd unit and the Task Scheduler command.
package service

import (
	"strings"
	"testing"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
)

func TestUnitContent(t *testing.T) {
	unit := UnitContent("/home/me/.local/bin/synctoceph", "scope2")
	for _, want := range []string{"ExecStart=/home/me/.local/bin/synctoceph schedule --profile scope2",
		"KillSignal=SIGTERM", "TimeoutStopSec=60", "WantedBy=default.target"} {
		if !strings.Contains(unit, want) {
			t.Errorf("unit lacks %q:\n%s", want, unit)
		}
	}
	if !strings.Contains(UnitContent("/opt/my tools/synctoceph", "p"), `ExecStart="/opt/my tools/synctoceph"`) {
		t.Error("paths with spaces must be quoted")
	}
}

func TestTaskArgs(t *testing.T) {
	tests := []struct {
		interval time.Duration
		at       string
		want     string
		ok       bool
	}{
		{4 * time.Hour, "", "/SC HOURLY /MO 4", true},
		{30 * time.Minute, "", "/SC MINUTE /MO 30", true},
		{90 * time.Minute, "", "/SC MINUTE /MO 90", true},
		{48 * time.Hour, "", "/SC DAILY /MO 2", true},
		{0, "02:00", "/SC DAILY /ST 02:00", true},
		{90 * time.Second, "", "", false},
		{25 * time.Hour, "", "", false},
		{0, "", "", false},
	}
	for _, tt := range tests {
		s := config.Settings{Interval: tt.interval, At: tt.at}
		args, err := TaskArgs("/home/me/.local/bin/synctoceph", "default", "Ubuntu", s)
		if (err == nil) != tt.ok || !strings.Contains(strings.Join(args, " "), tt.want) {
			t.Errorf("%v %q: got %v %v", tt.interval, tt.at, args, err)
		}
	}
	args, _ := TaskArgs("/home/me/.local/bin/synctoceph", "default", "Ubuntu", config.Settings{At: "02:00"})
	if !strings.Contains(strings.Join(args, " "), "wsl.exe -d Ubuntu -- /home/me/.local/bin/synctoceph run --profile default") {
		t.Errorf("wrong task command: %v", args)
	}
}
