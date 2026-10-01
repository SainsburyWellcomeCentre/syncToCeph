// This file checks paths inside the archive before anything is written to or
// read from them. A symlink in the archive could redirect a copy to somewhere
// else entirely, so synctoceph refuses any path in the archive that passes
// through a symlink, and any place where the archive has a file but the
// source has a folder (or the other way round).
package archive

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// ErrConflict marks a path that cannot be written safely.
var ErrConflict = errors.New("archive path conflict")

// conflictf returns an ErrConflict with a description.
func conflictf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrConflict, fmt.Sprintf(format, args...))
}

// NoSymlinks checks that no part of an absolute path is a symlink, from the
// filesystem root down to the path itself. Missing parts are fine.
func NoSymlinks(path string) error {
	// SAFETY: invariant 8 (symlinks on archive paths rejected).
	current := "/"
	for _, part := range strings.Split(strings.Trim(filepath.Clean(path), "/"), "/") {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("checking %s: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return conflictf("%s is a symlink", current)
		}
	}
	return nil
}

// PathChecker checks folders below a machine folder, remembering results so
// that each folder is examined only once per run.
type PathChecker struct {
	root string
	seen map[string]error
}

// NewPathChecker returns a checker for paths below root (a machine folder,
// already checked with NoSymlinks).
func NewPathChecker(root string) *PathChecker {
	return &PathChecker{root: root, seen: map[string]error{}}
}

// CheckParents checks every folder between the root and the relative path
// rel: each must be missing or a real folder (not a symlink, not a file).
func (c *PathChecker) CheckParents(rel string) error {
	// SAFETY: invariant 8 (symlinks on archive paths rejected).
	dir := filepath.Dir(rel)
	if dir == "." {
		return nil
	}
	parts := strings.Split(dir, "/")
	for i := range parts {
		prefix := strings.Join(parts[:i+1], "/")
		result, known := c.seen[prefix]
		if !known {
			result = c.checkFolder(prefix)
			c.seen[prefix] = result
		}
		if result == errMissing {
			return nil // everything below a missing folder is missing too
		}
		if result != nil {
			return result
		}
	}
	return nil
}

// errMissing marks a folder that does not exist yet (not an error).
var errMissing = errors.New("missing")

func (c *PathChecker) checkFolder(rel string) error {
	full := filepath.Join(c.root, rel)
	info, err := os.Lstat(full)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return errMissing
	case err != nil:
		return fmt.Errorf("checking %s: %w", full, err)
	case info.Mode()&os.ModeSymlink != 0:
		return conflictf("archive folder %s is a symlink", full)
	case !info.IsDir():
		return conflictf("the archive has a file at %s where the source has a folder", full)
	}
	return nil
}

// OpenRegular opens an archive file for reading without following symlinks,
// after checking its folders. It fails if the file is not a regular file.
func OpenRegular(root, rel string) (*os.File, error) {
	if err := NewPathChecker(root).CheckParents(rel); err != nil {
		return nil, err
	}
	full := filepath.Join(root, rel)
	f, err := os.OpenFile(full, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		f.Close()
		return nil, conflictf("%s is not a regular file", full)
	}
	return f, nil
}
