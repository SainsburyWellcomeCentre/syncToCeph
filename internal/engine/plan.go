// This file decides what to do with each source file by comparing it with its
// copy on the destination, <destination>/<animal>/<subfolder>/<rest> (size and
// modification time), and with the verified-file record:
//
//   - not on the destination: copy it;
//   - on the destination and matching: nothing to copy (verify it if asked);
//   - on the destination and different: in "skip" mode list it as
//     differing; in "replace" mode copy it, keeping the old version in history.
//
// Destination paths that pass through a symlink (including the animal folder
// and the subfolder), or where the destination has a folder and the source a
// file (or the reverse), are reported as errors and never written.
package engine

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// Plan lists what a run will do.
type Plan struct {
	Copy      []SourceFile    // files to copy (new, or replacing a different version)
	Replacing map[string]bool // paths in Copy that already exist on the destination
	Matching  []SourceFile    // already on the destination with the same size and time
	Verified  map[string]bool // paths in Matching with a matching verification record
	Differing []string        // different on the destination, left alone (skip mode)
	Conflicts []string        // paths that cannot be written safely
	CopyBytes int64
}

// MakePlan compares the scanned source files with their destination copies.
func MakePlan(s config.Settings, files []SourceFile, manifest destination.Manifest) Plan {
	plan := Plan{Replacing: map[string]bool{}, Verified: map[string]bool{}}
	checker := destination.NewPathChecker(s.Destination)
	for _, f := range files {
		target := s.DestRel(f.Path)
		if err := checker.CheckParents(target); err != nil {
			plan.Conflicts = append(plan.Conflicts, ui.Conflict(f.Path, err))
			continue
		}
		info, err := os.Lstat(filepath.Join(s.Destination, target))
		switch {
		case errors.Is(err, os.ErrNotExist):
			plan.add(f, false)
		case err != nil:
			plan.Conflicts = append(plan.Conflicts, ui.Conflict(f.Path, err))
		case info.Mode()&os.ModeSymlink != 0:
			// SAFETY: invariant 8 (symlinks on destination paths rejected).
			plan.Conflicts = append(plan.Conflicts, ui.Conflict(f.Path, errors.New(ui.DestEntryIsSymlink)))
		case !info.Mode().IsRegular():
			plan.Conflicts = append(plan.Conflicts, ui.Conflict(f.Path, errors.New(ui.DestEntryNotFile)))
		case info.Size() == f.Size && destination.SameTime(info.ModTime(), f.MTime):
			plan.Matching = append(plan.Matching, f)
			if e, ok := manifest.Entries[f.Path]; ok && e.Matches(f.Size, f.MTime) {
				plan.Verified[f.Path] = true
			}
		case s.Existing == config.ExistingReplace:
			plan.add(f, true)
		default:
			plan.Differing = append(plan.Differing, f.Path)
		}
	}
	return plan
}

// add puts a file on the copy list.
func (p *Plan) add(f SourceFile, replacing bool) {
	p.Copy = append(p.Copy, f)
	p.CopyBytes += f.Size
	if replacing {
		p.Replacing[f.Path] = true
	}
}

// Paths returns the relative paths of files.
func Paths(files []SourceFile) []string {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	return paths
}
