// This file answers questions about files that are already archived:
//
//   - CheckArchived (for `synctoceph check-archived`) reports, from the
//     verified-file record, whether each source file under a path is safely
//     in the archive, so people know what they may delete from the
//     acquisition PC. It reads only metadata and is fast.
//   - VerifyTree (for `synctoceph verify`) re-reads source and archive copies
//     with SHA-256 and updates the verified-file record.
package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/archive"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/platform"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/state"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// SourceRelative turns a path given on the command line (absolute, or
// relative to the current folder) into a path relative to the source
// folder. It fails if the path is outside the source.
func SourceRelative(s config.Settings, arg string) (string, error) {
	abs, err := filepath.Abs(arg)
	if err != nil {
		return "", err
	}
	source := resolve(s.Source)
	abs = resolve(abs)
	if !platform.Within(abs, source) {
		return "", ui.OutsideSource(arg, s.Source)
	}
	rel, _ := filepath.Rel(source, abs)
	if rel == "." {
		rel = ""
	}
	return rel, nil
}

// CheckArchived reports the archive state of every source file under sub.
func CheckArchived(ctx context.Context, s config.Settings, sub string) ([]archive.FileStatus, error) {
	scan, err := ScanSource(ctx, s.Source, sub, s.Exclude, NeverDefer, time.Now())
	if err != nil {
		return nil, ui.SourceUnreadable(s.Source, err)
	}
	manifest, err := archive.LoadManifest(s.MachineDir)
	if err != nil {
		return nil, ui.ManifestUnreadable(archive.ManifestPath(s.MachineDir), err)
	}
	checker := archive.NewPathChecker(s.MachineDir)
	var out []archive.FileStatus
	for _, f := range scan.Files {
		out = append(out, archive.FileStatus{Path: f.Path, Status: archiveState(s, checker, manifest, f)})
	}
	for _, p := range scan.ExcludedPaths {
		out = append(out, archive.FileStatus{Path: p, Status: archive.StatusExcluded})
	}
	for _, sk := range scan.Skipped {
		out = append(out, archive.FileStatus{Path: sk.Path, Status: archive.StatusSkipped})
	}
	for _, e := range scan.Errors {
		return out, errors.New(e)
	}
	return out, nil
}

// archiveState decides the state of one file from metadata only.
func archiveState(s config.Settings, checker *archive.PathChecker, m archive.Manifest, f SourceFile) string {
	if checker.CheckParents(f.Path) != nil {
		return archive.StatusNotArchived
	}
	info, err := os.Lstat(filepath.Join(s.MachineDir, f.Path))
	if err != nil || !info.Mode().IsRegular() {
		return archive.StatusNotArchived
	}
	if info.Size() != f.Size || !archive.SameTime(info.ModTime(), f.MTime) {
		return archive.StatusDiffers
	}
	if e, ok := m.Entries[f.Path]; ok && e.Matches(f.Size, f.MTime) {
		return archive.StatusVerified
	}
	return archive.StatusUnverified
}

// VerifyTree re-hashes every source file under sub that has an archive copy
// and records the ones that match. The caller holds the state lock.
func VerifyTree(ctx context.Context, s config.Settings, sub, runID string, log *state.Logger, progress func(done, total int)) (archive.VerifyReport, error) {
	var rep archive.VerifyReport
	if _, err := Preflight(s); err != nil {
		return rep, err
	}
	lock, err := state.Acquire(archive.LockPath(s.MachineDir))
	switch {
	case errors.Is(err, state.ErrLocked):
		return rep, ui.ArchiveBusy(s.MachineDir)
	case err == nil:
		defer lock.Release()
	case !errors.Is(err, state.ErrLockUnsupported) && !errors.Is(err, os.ErrNotExist):
		return rep, ui.MetaDirFailed(s.MachineDir, err)
	}
	scan, err := ScanSource(ctx, s.Source, sub, s.Exclude, NeverDefer, time.Now())
	if err != nil {
		return rep, ui.SourceUnreadable(s.Source, err)
	}
	rep.Errors = append(rep.Errors, scan.Errors...)
	var writer *archive.ManifestWriter
	for i, f := range scan.Files {
		if ctx.Err() != nil {
			break
		}
		if progress != nil {
			progress(i+1, len(scan.Files))
		}
		c := VerifyFile(ctx, s.Source, s.MachineDir, f)
		switch {
		case c.SHA256 != "":
			if writer == nil {
				if writer, err = archive.OpenManifestWriter(s.MachineDir); err != nil {
					return rep, ui.ManifestUnwritable(archive.ManifestPath(s.MachineDir), err)
				}
			}
			// SAFETY: invariant 4 ("archived" means verified): recorded only after a match.
			err = writer.Append(archive.Entry{Path: f.Path, Size: f.Size, MTime: f.MTime.UTC(),
				SHA256: c.SHA256, VerifiedAt: time.Now().UTC(), RunID: runID})
			if err != nil {
				rep.Errors = append(rep.Errors, err.Error())
				continue
			}
			rep.Checked++
			rep.Verified++
			rep.VerifiedBytes += f.Size
		case c.Defer != "":
			rep.Changed = append(rep.Changed, f.Path)
		case errors.Is(c.Err, os.ErrNotExist) || errors.Is(c.Err, errMissing):
			rep.NotArchived = append(rep.NotArchived, f.Path)
		case errors.Is(c.Err, errMismatch):
			rep.Checked++
			rep.Differing = append(rep.Differing, f.Path)
		case ctx.Err() == nil:
			rep.Errors = append(rep.Errors, ui.VerifyFailed(f.Path, c.Err))
		}
		log.Info("verify %s: verified=%t", f.Path, c.SHA256 != "")
	}
	if err := writer.Close(); err != nil {
		rep.Errors = append(rep.Errors, err.Error())
	}
	return rep, ctx.Err()
}
