// This file answers questions about files that are already on the
// destination:
//
//   - CheckCopied (for `synctoceph check-copied`) reports, from the
//     verified-file record, whether each source file under a path is safely
//     on ceph, so people know what they may delete from the acquisition PC.
//     It reads only metadata and is fast.
//   - VerifyTree (for `synctoceph verify`) re-reads source files and their
//     copies with SHA-256 and updates the verified-file record.
package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
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

// CheckCopied reports the destination state of every source file under sub.
func CheckCopied(ctx context.Context, s config.Settings, sub string) ([]destination.FileStatus, error) {
	scan, err := ScanSource(ctx, s.Source, sub, s.Exclude, NeverDefer, time.Now())
	if err != nil {
		return nil, ui.SourceUnreadable(s.Source, err)
	}
	manifest, err := destination.LoadManifest(s.MetaDir)
	if err != nil {
		return nil, ui.ManifestUnreadable(destination.ManifestPath(s.MetaDir), err)
	}
	checker := destination.NewPathChecker(s.Destination)
	var out []destination.FileStatus
	for _, f := range scan.Files {
		out = append(out, destination.FileStatus{Path: f.Path, Status: copyState(s, checker, manifest, f)})
	}
	for _, p := range scan.ExcludedPaths {
		out = append(out, destination.FileStatus{Path: p, Status: destination.StatusExcluded})
	}
	for _, sk := range scan.Skipped {
		out = append(out, destination.FileStatus{Path: sk.Path, Status: destination.StatusSkipped})
	}
	for _, e := range scan.Errors {
		return out, errors.New(e)
	}
	return out, nil
}

// copyState decides the state of one file from metadata only.
func copyState(s config.Settings, checker *destination.PathChecker, m destination.Manifest, f SourceFile) string {
	target := s.DestRel(f.Path)
	if checker.CheckParents(target) != nil {
		return destination.StatusNotCopied
	}
	info, err := os.Lstat(filepath.Join(s.Destination, target))
	if err != nil || !info.Mode().IsRegular() {
		return destination.StatusNotCopied
	}
	if info.Size() != f.Size || !destination.SameTime(info.ModTime(), f.MTime) {
		return destination.StatusDiffers
	}
	if e, ok := m.Entries[f.Path]; ok && e.Matches(f.Size, f.MTime) {
		return destination.StatusVerified
	}
	return destination.StatusUnverified
}

// VerifyTree re-hashes every source file under sub that has a destination copy
// and records the ones that match. The caller holds the state lock.
func VerifyTree(ctx context.Context, s config.Settings, sub, runID string, log *state.Logger, progress func(done, total int)) (destination.VerifyReport, error) {
	var rep destination.VerifyReport
	if _, err := Preflight(s); err != nil {
		return rep, err
	}
	lock, err := state.Acquire(destination.LockPath(s.MetaDir))
	switch {
	case errors.Is(err, state.ErrLocked):
		return rep, ui.DestinationBusy(s.Subfolder)
	case err == nil:
		defer lock.Release()
	case !errors.Is(err, state.ErrLockUnsupported) && !errors.Is(err, os.ErrNotExist):
		return rep, ui.MetaDirFailed(s.MetaDir, err)
	}
	scan, err := ScanSource(ctx, s.Source, sub, s.Exclude, NeverDefer, time.Now())
	if err != nil {
		return rep, ui.SourceUnreadable(s.Source, err)
	}
	rep.Errors = append(rep.Errors, scan.Errors...)
	var writer *destination.ManifestWriter
	for i, f := range scan.Files {
		if ctx.Err() != nil {
			break
		}
		if progress != nil {
			progress(i+1, len(scan.Files))
		}
		c := VerifyFile(ctx, s, f)
		switch {
		case c.SHA256 != "":
			if writer == nil {
				if writer, err = destination.OpenManifestWriter(s.MetaDir); err != nil {
					return rep, ui.ManifestUnwritable(destination.ManifestPath(s.MetaDir), err)
				}
			}
			// SAFETY: invariant 4 ("copied" means verified): recorded only after a match.
			err = writer.Append(destination.Entry{Path: f.Path, Size: f.Size, MTime: f.MTime.UTC(),
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
			rep.NotCopied = append(rep.NotCopied, f.Path)
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
