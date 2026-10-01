// Package service sets up automatic runs: a systemd user service on Linux
// (and on WSL with systemd), or a Windows Task Scheduler task on WSL without
// systemd. It never uses sudo; when something needs administrator rights it
// tells the user the exact command instead.
//
// This file handles systemd. The service runs `synctoceph schedule`, which
// stays running and starts each sync at the configured time.
package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/platform"
)

// UnitName returns the systemd unit name for a profile.
func UnitName(profile string) string { return "synctoceph-" + profile + ".service" }

// UnitDir returns the folder for systemd user units.
func UnitDir() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "systemd", "user"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding your home folder: %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

// stopTimeoutSeconds is how long systemd waits for synctoceph to stop before
// killing it. It is longer than the 30-second rsync grace period, so a
// running copy is always stopped gracefully first.
const stopTimeoutSeconds = 60

// UnitContent returns the text of the systemd unit for a profile.
func UnitContent(binary, profile string) string {
	return fmt.Sprintf(`# Written by synctoceph service install. Remove with: synctoceph service uninstall --profile %[2]s
[Unit]
Description=synctoceph scheduled archive sync (profile %[2]s)
After=network-online.target

[Service]
Type=simple
ExecStart=%[1]s schedule --profile %[2]s
Restart=on-failure
RestartSec=60
# Stop gracefully: synctoceph asks rsync to stop and waits up to 30 s.
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=%[3]d

[Install]
WantedBy=default.target
`, systemdQuote(binary), profile, stopTimeoutSeconds)
}

// systemdQuote quotes a path for ExecStart if it contains spaces.
func systemdQuote(path string) string {
	if strings.ContainsAny(path, " \t\"'\\") {
		return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(path) + `"`
	}
	return path
}

// run runs a command and returns its combined output, trimmed.
func run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return text, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, text)
	}
	return text, nil
}

// InstallSystemd writes the unit, then enables and starts it. It returns
// notes for the user (for example about lingering).
func InstallSystemd(binary, profile string) ([]string, error) {
	dir, err := UnitDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating %s: %w", dir, err)
	}
	path := filepath.Join(dir, UnitName(profile))
	if err := platform.WriteFileAtomic(path, []byte(UnitContent(binary, profile)), 0o644); err != nil {
		return nil, err
	}
	if _, err := run("systemctl", "--user", "daemon-reload"); err != nil {
		return nil, err
	}
	if _, err := run("systemctl", "--user", "enable", "--now", UnitName(profile)); err != nil {
		return nil, err
	}
	notes := []string{"Wrote " + path}
	if user := os.Getenv("USER"); user != "" {
		if linger, err := run("loginctl", "show-user", user, "--property=Linger", "--value"); err == nil && linger == "no" {
			notes = append(notes, "linger-off")
		}
	}
	return notes, nil
}

// UninstallSystemd stops and disables the unit and removes its file. It
// reports whether a unit was found.
func UninstallSystemd(profile string) (bool, error) {
	dir, err := UnitDir()
	if err != nil {
		return false, err
	}
	path := filepath.Join(dir, UnitName(profile))
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return false, nil
	}
	if _, err := exec.LookPath("systemctl"); err == nil {
		// Ignore errors: the unit may already be stopped or unknown to systemd.
		run("systemctl", "--user", "disable", "--now", UnitName(profile))
	}
	if err := os.Remove(path); err != nil {
		return true, fmt.Errorf("removing %s: %w", path, err)
	}
	if _, err := exec.LookPath("systemctl"); err == nil {
		run("systemctl", "--user", "daemon-reload")
	}
	return true, nil
}

// SystemdStatus reports whether the unit file exists and, if systemctl is
// available, whether it is enabled and active.
func SystemdStatus(profile string) (installed bool, enabled, active string) {
	dir, err := UnitDir()
	if err != nil {
		return false, "", ""
	}
	if _, err := os.Lstat(filepath.Join(dir, UnitName(profile))); err != nil {
		return false, "", ""
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return true, "unknown", "unknown"
	}
	enabled, _ = run("systemctl", "--user", "is-enabled", UnitName(profile))
	active, _ = run("systemctl", "--user", "is-active", UnitName(profile))
	return true, firstLine(enabled), firstLine(active)
}

// firstLine returns the first line of s.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
