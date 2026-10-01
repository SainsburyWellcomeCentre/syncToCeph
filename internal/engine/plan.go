// This file decides what to do with each source file by comparing it with the
// archive copy (size and modification time) and with the verified-file
// record:
//
//   - not in the archive: copy it;
//   - in the archive and matching: nothing to copy (verify it if asked);
//   - in the archive and different: in "skip" mode list it as differing; in
//     "replace" mode copy it, keeping the old version in history.
//
// Paths in the archive that pass through a symlink, or where the archive has
// a folder and the source a file (or the reverse), are reported as errors
// and never written.
package engine

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/archive"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// Plan lists what a run will do.
type Plan struct {
	Copy      []SourceFile    // files to copy (new, or replacing a different version)
	Replacing map[string]bool // paths in Copy that already exist in the archive
	Matching  []SourceFile    // already in the archive with the same size and time
	Verified  map[string]bool // paths in Matching with a matching verification record
	Differing []string        // different in the archive, left alone (skip mode)
	Conflicts []string        // paths that cannot be written safely
	CopyBytes int64
}

// MakePlan compares the scanned source files with the machine folder.
func MakePlan(s config.Settings, files []SourceFile, manifest archive.Manifest) Plan {
	plan := Plan{Replacing: map[string]bool{}, Verified: map[string]bool{}}
	checker := archive.NewPathChecker(s.MachineDir)
	for _, f := range files {
		if err := checker.CheckParents(f.Path); err != nil {
			plan.Conflicts = append(plan.Conflicts, ui.Conflict(f.Path, err))
			continue
		}
		info, err := os.Lstat(filepath.Join(s.MachineDir, f.Path))
		switch {
		case errors.Is(err, os.ErrNotExist):
			plan.add(f, false)
		case err != nil:
			plan.Conflicts = append(plan.Conflicts, ui.Conflict(f.Path, err))
		case info.Mode()&os.ModeSymlink != 0:
			// SAFETY: invariant 8 (symlinks on archive paths rejected).
			plan.Conflicts = append(plan.Conflicts, ui.Conflict(f.Path, errors.New(ui.ArchiveEntryIsSymlink)))
		case !info.Mode().IsRegular():
			plan.Conflicts = append(plan.Conflicts, ui.Conflict(f.Path, errors.New(ui.ArchiveEntryNotFile)))
		case info.Size() == f.Size && archive.SameTime(info.ModTime(), f.MTime):
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
