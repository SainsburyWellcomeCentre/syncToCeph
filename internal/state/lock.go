// This file implements the lock that allows only one synctoceph run per
// profile at a time. It uses flock, a lock the Linux kernel attaches to an
// open file: it is released automatically when every process holding the
// file closes it or exits, so a crash can never leave a stale lock behind.
// rsync inherits the open lock file, so the lock stays held for as long as
// rsync is still running, even if synctoceph itself was killed.
package state

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// lockRetryWindow is how long Acquire keeps trying when the lock is busy. A
// command such as `status` briefly takes the lock to test it, so a run that
// starts at the same instant should wait a moment rather than fail.
const lockRetryWindow = time.Second

// lockRetryPause is the pause between attempts within lockRetryWindow.
const lockRetryPause = 100 * time.Millisecond

// ErrLocked means another process holds the lock.
var ErrLocked = errors.New("lock is held by another process")

// ErrLockUnsupported means the filesystem does not support flock (some
// network filesystems). Callers decide whether that is acceptable.
var ErrLockUnsupported = errors.New("the filesystem does not support locks")

// Lock is a held lock. Keep it until the protected work is finished, then
// call Release.
type Lock struct {
	// File is the open lock file. Pass it to child processes (rsync) so the
	// lock stays held while they run.
	File *os.File
}

// Acquire takes the lock at path, creating the lock file if needed. It
// returns ErrLocked if another process holds it.
func Acquire(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening lock file %s: %w", path, err)
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("lock file %s is not a regular file", path)
	}
	deadline := time.Now().Add(lockRetryWindow)
	for {
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return &Lock{File: f}, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) || time.Now().After(deadline) {
			break
		}
		time.Sleep(lockRetryPause)
	}
	f.Close()
	switch {
	case errors.Is(err, unix.EWOULDBLOCK):
		return nil, ErrLocked
	case errors.Is(err, unix.ENOLCK), errors.Is(err, unix.EOPNOTSUPP), errors.Is(err, unix.ENOSYS):
		return nil, fmt.Errorf("locking %s: %w", path, ErrLockUnsupported)
	default:
		return nil, fmt.Errorf("locking %s: %w", path, err)
	}
}

// Release frees the lock. rsync children that inherited the file keep it
// locked until they exit.
func (l *Lock) Release() {
	if l != nil && l.File != nil {
		l.File.Close()
		l.File = nil
	}
}

// IsHeld reports whether some process holds the lock at path. It never
// creates the lock file, so read-only commands leave no trace.
func IsHeld(path string) bool {
	f, err := os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return false
	}
	defer f.Close()
	err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err == nil {
		unix.Flock(int(f.Fd()), unix.LOCK_UN)
		return false
	}
	return errors.Is(err, unix.EWOULDBLOCK)
}
