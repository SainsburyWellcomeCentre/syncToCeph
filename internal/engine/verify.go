// This file verifies copies. A file counts as copied only when the SHA-256 of
// the source file and of its destination copy are equal, and the source file
// still has the size, modification time and inode recorded by the scan (so
// it did not change while being copied). A source file that changed is
// deferred to a later run, not treated as a failure.
package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
)

// hashBufferSize is how much of a file is read at a time while hashing.
const hashBufferSize = 1 << 20

// errMismatch means the destination copy's content differs from the source.
var errMismatch = errors.New("the copy on the destination differs from the source (SHA-256 mismatch)")

// errMissing means there is no destination copy.
var errMissing = errors.New("the copy on the destination is missing")

// Check is the outcome of verifying one file.
type Check struct {
	SHA256 string // set when verified
	// Defer is set when the source changed or vanished; the file will be
	// tried again on the next run.
	Defer string
	// Err is set when the destination copy is missing or different.
	Err error
}

// VerifyFile compares the source file f with its copy on the destination.
func VerifyFile(ctx context.Context, s config.Settings, f SourceFile) Check {
	// SAFETY: invariant 4 ("copied" means verified).
	sourceSum, deferReason, err := hashSource(ctx, filepath.Join(s.Source, f.Path), f)
	if err != nil || deferReason != "" {
		return Check{Defer: deferReason, Err: err}
	}
	copyFile, err := destination.OpenRegular(s.Destination, s.DestRel(f.Path))
	if errors.Is(err, os.ErrNotExist) {
		return Check{Err: errMissing}
	}
	if err != nil {
		return Check{Err: fmt.Errorf("opening the copy on the destination: %w", err)}
	}
	defer copyFile.Close()
	copySum, size, err := hashOpen(ctx, copyFile)
	if err != nil {
		return Check{Err: fmt.Errorf("reading the copy on the destination: %w", err)}
	}
	if size != f.Size || copySum != sourceSum {
		return Check{Err: errMismatch}
	}
	return Check{SHA256: sourceSum}
}

// hashSource hashes a source file, checking before and after that it is the
// same file, unchanged since the scan. deferReason is set if it changed.
func hashSource(ctx context.Context, path string, f SourceFile) (sum, deferReason string, err error) {
	// SAFETY: invariant 2 (nothing is deleted from the source): source files
	// are only ever opened read-only.
	file, err := os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		return "", destination.ReasonVanished, nil
	}
	if err != nil {
		return "", destination.ReasonChanged, nil
	}
	defer file.Close()
	if !sameAsScan(file, f) {
		return "", destination.ReasonChanged, nil
	}
	sum, _, err = hashOpen(ctx, file)
	if err != nil {
		if ctx.Err() != nil {
			return "", "", err
		}
		return "", "", fmt.Errorf("reading the source file: %w", err)
	}
	// Check again: the file must not have changed while it was being read,
	// nor been replaced by another file under the same name.
	if !sameAsScan(file, f) {
		return "", destination.ReasonChanged, nil
	}
	if info, err := os.Lstat(path); err != nil || !sameIdentity(info, f) {
		return "", destination.ReasonChanged, nil
	}
	return sum, "", nil
}

// sameAsScan reports whether an open file still matches the scan.
func sameAsScan(file *os.File, f SourceFile) bool {
	info, err := file.Stat()
	return err == nil && info.Mode().IsRegular() && sameIdentity(info, f)
}

// sameIdentity compares size, modification time, inode and device.
func sameIdentity(info os.FileInfo, f SourceFile) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return info.Size() == f.Size && info.ModTime().Equal(f.MTime) &&
		st.Ino == f.Inode && uint64(st.Dev) == f.Dev
}

// hashOpen returns the SHA-256 and size of an open file, stopping early if
// ctx is cancelled.
func hashOpen(ctx context.Context, file *os.File) (string, int64, error) {
	h := sha256.New()
	buf := make([]byte, hashBufferSize)
	var size int64
	for {
		if err := ctx.Err(); err != nil {
			return "", size, err
		}
		n, err := file.Read(buf)
		h.Write(buf[:n])
		size += int64(n)
		if err == io.EOF {
			return hex.EncodeToString(h.Sum(nil)), size, nil
		}
		if err != nil {
			return "", size, err
		}
	}
}
