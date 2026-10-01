// This file prepares a profile's state folder (status, logs, lock and control
// socket). The folder must be on a local Linux disk: locks and sockets do not
// work reliably on Windows drives or network shares, which was a real bug in
// the earlier Python version. It must also be private to the user.
package state

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/platform"
)

// File names inside the state folder.
const (
	LockName    = "lock"
	StatusName  = "status.json"
	LogName     = "sync.log"
	ControlName = "control.sock"
)

// NotLocalError means the state folder is on a Windows drive or a network
// share. Kind says which, in words.
type NotLocalError struct {
	Dir, Kind string
}

func (e *NotLocalError) Error() string { return e.Dir + " is on " + e.Kind }

// CheckLocal returns a *NotLocalError if dir (which may not exist yet) is on a
// Windows drive or a network share.
func CheckLocal(dir string) error {
	existing := dir
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			break
		}
		existing = parent
	}
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		resolved = existing
	}
	rest, _ := filepath.Rel(existing, dir)
	if platform.DriveLetter(dir) != "" || platform.DriveLetter(filepath.Join(resolved, rest)) != "" {
		return &NotLocalError{dir, "a Windows drive"}
	}
	mounts, err := platform.ReadMounts()
	if err != nil {
		return nil // Without a mount table there is nothing more to check.
	}
	if m, ok := platform.MountFor(mounts, resolved); ok && platform.IsNetwork(m) {
		kind := "a network share (" + m.FSType + ")"
		if platform.IsWindowsDrive(m) {
			kind = "a Windows drive"
		}
		return &NotLocalError{dir, kind}
	}
	return nil
}

// Prepare creates the state folder if needed and checks that it is local,
// a real folder (not a symlink), owned by the user and not writable by
// others.
func Prepare(dir string) error {
	if err := CheckLocal(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a folder (it may be a symlink)", dir)
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return fmt.Errorf("%s belongs to another user", dir)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("other users can write to %s", dir)
	}
	return nil
}

// Exists reports whether the state folder exists. Read-only commands use it
// to avoid creating anything.
func Exists(dir string) bool {
	info, err := os.Lstat(dir)
	return err == nil && info.IsDir()
}
