// This file scans the source folder. For every file it records the size,
// modification time and inode (the file's identity on disk), which are
// checked again after copying to prove the file did not change. Symlinks and
// special files (pipes, sockets, devices) are skipped and reported, never
// followed. Files matching an exclude pattern are left out, and files
// modified within settle_time are deferred because they are probably still
// being written by the acquisition software.
package engine

import (
	"context"
	"io/fs"
	"math"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/archive"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// SourceFile is a regular file found in the source.
type SourceFile struct {
	Path  string // relative to the source folder, with "/" separators
	Size  int64
	MTime time.Time
	Inode uint64
	Dev   uint64
}

// Scan is the result of scanning the source.
type Scan struct {
	Files    []SourceFile // settled files, ready to plan
	Deferred []archive.DeferredFile
	Skipped  []archive.SkippedFile
	Excluded int
	// ExcludedPaths lists excluded files and folders (a folder's contents
	// are not listed separately).
	ExcludedPaths []string
	Errors        []string
	Bytes         int64 // total size of settled files
}

// NeverDefer is a settle time that defers nothing, for commands that only
// inspect files.
const NeverDefer = time.Duration(math.MinInt64)

// ScanSource walks the source folder without following symlinks. If sub is
// not empty, only that file or folder (relative to source) is scanned; the
// paths found are still relative to source.
func ScanSource(ctx context.Context, source, sub string, exclude []string, settle time.Duration, now time.Time) (Scan, error) {
	var scan Scan
	start := filepath.Join(source, sub)
	err := filepath.WalkDir(start, func(full string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rel, _ := filepath.Rel(source, full)
		if err != nil {
			if full == start {
				return err
			}
			scan.Errors = append(scan.Errors, ui.ScanError(rel, err))
			return nil
		}
		if rel == "." {
			return nil
		}
		return scan.add(rel, d, exclude, settle, now)
	})
	return scan, err
}

// add classifies one entry of the source.
func (scan *Scan) add(rel string, d fs.DirEntry, exclude []string, settle time.Duration, now time.Time) error {
	name := d.Name()
	isDir := d.IsDir()
	switch {
	case name == archive.MetaName || name == archive.PartialName:
		scan.Skipped = append(scan.Skipped, archive.SkippedFile{Path: rel, Reason: ui.ReasonReserved})
	case excluded(rel, name, exclude):
		scan.Excluded++
		scan.ExcludedPaths = append(scan.ExcludedPaths, rel)
	case d.Type()&fs.ModeSymlink != 0:
		// SAFETY: invariant 8 (source symlinks are skipped and reported, never followed).
		scan.Skipped = append(scan.Skipped, archive.SkippedFile{Path: rel, Reason: ui.ReasonSymlink})
		return nil
	case isDir:
		return nil
	case !d.Type().IsRegular():
		scan.Skipped = append(scan.Skipped, archive.SkippedFile{Path: rel, Reason: ui.ReasonSpecial})
		return nil
	default:
		return scan.addFile(rel, d, settle, now)
	}
	if isDir {
		return filepath.SkipDir
	}
	return nil
}

// addFile records a regular file, or defers it if it changed recently.
func (scan *Scan) addFile(rel string, d fs.DirEntry, settle time.Duration, now time.Time) error {
	info, err := d.Info()
	if err != nil {
		scan.Deferred = append(scan.Deferred, archive.DeferredFile{Path: rel, Reason: archive.ReasonVanished})
		return nil
	}
	file := SourceFile{Path: rel, Size: info.Size(), MTime: info.ModTime()}
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		file.Inode, file.Dev = st.Ino, uint64(st.Dev)
	}
	if now.Sub(file.MTime) < settle {
		scan.Deferred = append(scan.Deferred, archive.DeferredFile{Path: rel,
			Reason: archive.ReasonSettling, ModifiedAt: file.MTime})
		return nil
	}
	scan.Files = append(scan.Files, file)
	scan.Bytes += file.Size
	return nil
}

// excluded reports whether an entry matches an exclude pattern. A pattern
// without "/" is matched against the name alone (so "Thumbs.db" matches in
// every folder); a pattern with "/" is matched against the path from the
// source folder.
func excluded(rel, name string, patterns []string) bool {
	for _, pattern := range patterns {
		target := name
		if strings.Contains(pattern, "/") {
			target = rel
		}
		if ok, _ := path.Match(strings.TrimPrefix(pattern, "/"), target); ok {
			return true
		}
	}
	return false
}
