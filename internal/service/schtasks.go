// This file handles Windows Task Scheduler, for WSL without systemd. Windows
// starts `wsl.exe -d <distro> -- synctoceph run` on the schedule; each start
// is one run. Task Scheduler does not start a new copy while the previous one
// is still running, and the synctoceph lock guards against overlap as well.
// Daily times follow the Windows time zone, not TZ inside WSL.
package service

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
)

// schtasks is the Windows Task Scheduler command, reachable from WSL.
const schtasks = "schtasks.exe"

// TaskName returns the Task Scheduler task name for a profile.
func TaskName(profile string) string { return "synctoceph-" + profile }

// maxTaskCommand is the longest command Task Scheduler accepts (/TR).
const maxTaskCommand = 261

// TaskArgs returns the schtasks.exe arguments that create the task.
func TaskArgs(binary, profile, distro string, s config.Settings) ([]string, error) {
	if distro == "" {
		return nil, fmt.Errorf("the WSL distribution name is unknown (WSL_DISTRO_NAME is not set)")
	}
	command := fmt.Sprintf(`wsl.exe -d %s -- %s run --profile %s --quiet`, distro, binary, profile)
	if strings.ContainsAny(binary, " \"") || len(command) > maxTaskCommand {
		return nil, fmt.Errorf("the program path %q cannot be used in a Task Scheduler command; install synctoceph to a path without spaces", binary)
	}
	args := []string{"/Create", "/F", "/TN", TaskName(profile), "/TR", command}
	switch {
	case s.At != "":
		return append(args, "/SC", "DAILY", "/ST", s.At), nil
	case s.Interval <= 0:
		return nil, fmt.Errorf("no schedule is configured (set interval or at)")
	case s.Interval%time.Minute != 0:
		return nil, fmt.Errorf("the Task Scheduler needs an interval in whole minutes, not %s", config.FormatDuration(s.Interval))
	case s.Interval%(24*time.Hour) == 0:
		return append(args, "/SC", "DAILY", "/MO", fmt.Sprint(int(s.Interval/(24*time.Hour)))), nil
	case s.Interval%time.Hour == 0 && s.Interval < 24*time.Hour:
		return append(args, "/SC", "HOURLY", "/MO", fmt.Sprint(int(s.Interval/time.Hour))), nil
	case s.Interval < 24*time.Hour:
		return append(args, "/SC", "MINUTE", "/MO", fmt.Sprint(int(s.Interval/time.Minute))), nil
	}
	return nil, fmt.Errorf("the Task Scheduler cannot repeat every %s; use whole hours, whole days, or fewer than 24 hours", config.FormatDuration(s.Interval))
}

// PrintableTaskCommand returns the schtasks.exe command as one line that can
// be pasted into a terminal.
func PrintableTaskCommand(args []string) string {
	parts := []string{schtasks}
	for _, a := range args {
		if strings.ContainsAny(a, " ") {
			a = `"` + a + `"`
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

// HasTaskScheduler reports whether schtasks.exe can be run from here.
func HasTaskScheduler() bool {
	_, err := exec.LookPath(schtasks)
	return err == nil
}

// InstallTask creates (or replaces) the Task Scheduler task.
func InstallTask(args []string) error {
	_, err := run(schtasks, args...)
	return err
}

// UninstallTask deletes the task. It reports whether a task was found.
func UninstallTask(profile string) (bool, error) {
	if !HasTaskScheduler() {
		return false, nil
	}
	if _, err := run(schtasks, "/Query", "/TN", TaskName(profile)); err != nil {
		return false, nil
	}
	_, err := run(schtasks, "/Delete", "/TN", TaskName(profile), "/F")
	return true, err
}

// TaskStatus reports whether the task exists and its next run time.
func TaskStatus(profile string) (installed bool, detail string) {
	if !HasTaskScheduler() {
		return false, ""
	}
	out, err := run(schtasks, "/Query", "/TN", TaskName(profile), "/FO", "LIST")
	if err != nil {
		return false, ""
	}
	var fields []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(line, "Next Run Time:") || strings.HasPrefix(line, "Status:") {
			fields = append(fields, line)
		}
	}
	return true, strings.Join(fields, "; ")
}

// Distro returns the name of the current WSL distribution.
func Distro() string { return os.Getenv("WSL_DISTRO_NAME") }
