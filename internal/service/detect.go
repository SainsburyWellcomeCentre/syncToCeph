// This file chooses how automatic runs are set up on this computer.
package service

import (
	"os/exec"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/platform"
)

// Service managers synctoceph can use.
const (
	ManagerAuto    = "auto"
	ManagerSystemd = "systemd"
	ManagerTask    = "task-scheduler"
	ManagerNone    = "none"
)

// Detect returns the service manager to use: systemd when it is running
// (Linux, or WSL with systemd enabled), otherwise Windows Task Scheduler on
// WSL, otherwise none.
func Detect() string {
	if _, err := exec.LookPath("systemctl"); err == nil && platform.HasSystemd() {
		return ManagerSystemd
	}
	if platform.IsWSL() && HasTaskScheduler() {
		return ManagerTask
	}
	return ManagerNone
}
