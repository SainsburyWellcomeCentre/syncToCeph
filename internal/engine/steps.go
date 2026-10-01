// This file holds the middle steps of a run: recording the scan and the plan
// in the summary, checking files already in the archive, copying with rsync,
// and verifying the copies.
package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/archive"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// rsyncVanished is rsync's exit code for "some source files vanished before
// they could be transferred". Those files are handled by verification.
const rsyncVanished = 24

// recordScan copies the scan results into the summary.
func (r *run) recordScan(scan Scan) {
	r.sum.Scanned = len(scan.Files) + len(scan.Deferred)
	r.sum.ScannedBytes = scan.Bytes
	r.sum.Excluded = scan.Excluded
	r.sum.Deferred = append(r.sum.Deferred, scan.Deferred...)
	r.sum.Skipped = scan.Skipped
	for _, e := range scan.Errors {
		r.fail(errors.New(e))
	}
	for _, s := range scan.Skipped {
		r.log.Info("Skipped %s: %s", s.Path, s.Reason)
	}
}

// recordPlan copies the plan into the summary.
func (r *run) recordPlan(plan Plan) {
	r.sum.Planned = len(plan.Copy)
	r.sum.PlannedBytes = plan.CopyBytes
	r.sum.Replaced = len(plan.Replacing)
	r.sum.Differing = plan.Differing
	for _, c := range plan.Conflicts {
		r.fail(errors.New(c))
	}
	for _, f := range plan.Matching {
		switch {
		case plan.Verified[f.Path] || r.verified[f.Path]:
			r.sum.UpToDate++
		case !r.previous[f.Path]:
			r.sum.Unverified++
		}
	}
}

// checkExisting hashes files that are already in the archive with the same
// size and time, when verify = "all" or when an earlier run copied them
// without verifying. A copy that turns out to differ is replaced (replace
// mode) or listed as differing (skip mode).
func (r *run) checkExisting(ctx context.Context, plan *Plan) {
	var todo []SourceFile
	matching := map[string]bool{}
	for _, f := range plan.Matching {
		matching[f.Path] = true
		if r.s.Verify == config.VerifyAll || (r.previous[f.Path] && !plan.Verified[f.Path]) {
			todo = append(todo, f)
		}
	}
	for p := range r.previous {
		if !matching[p] || plan.Verified[p] {
			r.dropped[p] = true
		}
	}
	if len(todo) == 0 {
		return
	}
	for i, f := range todo {
		if ctx.Err() != nil {
			return
		}
		r.phase("checking existing", fmt.Sprintf("%s of %s", ui.Count(i+1), ui.Files(len(todo))))
		c := VerifyFile(ctx, r.s.Source, r.s.MachineDir, f)
		r.dropped[f.Path] = true
		switch {
		case c.SHA256 != "":
			r.record(f, c.SHA256)
		case c.Defer != "":
			r.deferFile(f, c.Defer)
		case errors.Is(c.Err, errMismatch) && r.s.Existing == config.ExistingReplace:
			plan.add(f, true)
		case errors.Is(c.Err, errMismatch):
			plan.Differing = append(plan.Differing, f.Path)
		case ctx.Err() == nil:
			r.fail(errors.New(ui.VerifyFailed(f.Path, c.Err)))
		}
	}
	// Checked files that were deferred, replaced or found different are no
	// longer "matching".
	checked := map[string]bool{}
	for _, f := range todo {
		checked[f.Path] = true
	}
	var still []SourceFile
	for _, f := range plan.Matching {
		if !checked[f.Path] || r.verified[f.Path] {
			still = append(still, f)
		}
	}
	plan.Matching = still
}

// transfer runs rsync for the files on the copy list. It returns false if
// the run was interrupted or rsync could not be started.
func (r *run) transfer(ctx context.Context, rsyncPath string, plan Plan) bool {
	historyDir := archive.HistoryDir(r.s.MachineDir, r.o.RunID)
	if len(plan.Replacing) > 0 {
		dir, err := archive.MakeHistoryDir(r.s.MachineDir, r.o.RunID)
		if err != nil {
			r.fail(ui.MetaDirFailed(r.s.MachineDir, err))
			return false
		}
		r.sum.HistoryDir = dir
	}
	sizes := map[string]int64{}
	for _, f := range plan.Copy {
		sizes[f.Path] = f.Size
		// Until verified, every file on the copy list counts as pending.
		r.pending[f.Path] = true
	}
	copied := map[string]bool{}
	job := rsyncJob{path: rsyncPath, args: RsyncArgs(r.s, historyDir), files: Paths(plan.Copy),
		env: rsyncEnv(r.s.StateDir), lockFiles: r.o.LockFiles, grace: r.o.Grace}
	job.onStdout = func(line string) {
		r.log.Info("rsync: %s", line)
		if item, ok := ParseItemized(line); ok && item.Received() {
			if _, planned := sizes[item.Name]; planned && !copied[item.Name] {
				copied[item.Name] = true
				r.sum.Copied++
				r.sum.CopiedBytes += sizes[item.Name]
			}
		}
		if r.o.Verbose != nil {
			r.o.Verbose(line)
		}
	}
	job.onStderr = func(line string) {
		r.log.Warn("rsync: %s", line)
		if r.o.Verbose != nil {
			r.o.Verbose(line)
		}
	}
	r.log.Info("Running: %s %s", rsyncPath, strings.Join(job.args, " "))
	code, err := runRsync(ctx, job)
	r.sum.RsyncExitCode = &code
	if err != nil {
		r.fail(ui.RsyncStartFailed(rsyncPath, err))
		return false
	}
	if ctx.Err() != nil {
		r.log.Warn("rsync stopped (exit code %d); this run is not verified", code)
		return false
	}
	if code != 0 && code != rsyncVanished {
		r.fail(ui.RsyncFailed(code))
	}
	return true
}

// verifyCopies checks every file on the copy list with SHA-256.
func (r *run) verifyCopies(ctx context.Context, files []SourceFile) {
	for i, f := range files {
		if ctx.Err() != nil {
			return
		}
		r.phase("verifying", fmt.Sprintf("%s of %s", ui.Count(i+1), ui.Files(len(files))))
		c := VerifyFile(ctx, r.s.Source, r.s.MachineDir, f)
		switch {
		case c.SHA256 != "":
			r.record(f, c.SHA256)
		case c.Defer != "":
			delete(r.pending, f.Path)
			r.deferFile(f, c.Defer)
		case ctx.Err() == nil:
			r.fail(errors.New(ui.VerifyFailed(f.Path, c.Err)))
		}
	}
}

// record adds a verified file to the manifest and the summary.
func (r *run) record(f SourceFile, sum string) {
	// SAFETY: invariant 4 ("archived" means verified): only called after the SHA-256 of
	// source and archive copy matched and the source was unchanged.
	err := r.writer.Append(archive.Entry{Path: f.Path, Size: f.Size, MTime: f.MTime.UTC(),
		SHA256: sum, VerifiedAt: time.Now().UTC(), RunID: r.o.RunID})
	if err != nil {
		r.fail(ui.ManifestUnwritable(archive.ManifestPath(r.s.MachineDir), err))
		return
	}
	r.appended++
	r.verified[f.Path] = true
	r.sum.Verified++
	r.sum.VerifiedBytes += f.Size
}

// deferFile records a file left for a later run.
func (r *run) deferFile(f SourceFile, reason string) {
	r.log.Warn("Deferred %s: %s", f.Path, reason)
	r.sum.Deferred = append(r.sum.Deferred, archive.DeferredFile{Path: f.Path, Reason: reason, ModifiedAt: f.MTime})
}
