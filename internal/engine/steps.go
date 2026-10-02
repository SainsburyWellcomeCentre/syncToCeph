// This file holds the middle steps of a run: recording the scan and the plan
// in the summary, checking files already on the destination, copying with
// rsync, and verifying the copies.
package engine

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
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

// checkExisting hashes files that are already on the destination with the same
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
		c := VerifyFile(ctx, r.s, f)
		r.dropped[f.Path] = true
		switch {
		case c.SHA256 != "":
			r.record(f, c.SHA256, i+1, len(todo))
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

// transfer runs rsync for the files on the copy list, once per animal
// folder: <source>/<animal>/ is copied into <destination>/<animal>/<subfolder>/.
// It returns false if the run was interrupted or rsync could not be started.
func (r *run) transfer(ctx context.Context, rsyncPath string, plan Plan) bool {
	byAnimal := map[string][]SourceFile{}
	for _, f := range plan.Copy {
		animal, _ := destination.SplitAnimal(f.Path)
		byAnimal[animal] = append(byAnimal[animal], f)
		// Until verified, every file on the copy list counts as pending.
		r.pending[f.Path] = true
	}
	animals := make([]string, 0, len(byAnimal))
	for animal := range byAnimal {
		animals = append(animals, animal)
	}
	sort.Strings(animals)
	worst := 0
	for _, animal := range animals {
		code, ok := r.transferAnimal(ctx, rsyncPath, animal, byAnimal[animal], plan)
		if code != 0 && (worst == 0 || worst == rsyncVanished) {
			worst = code
		}
		r.sum.RsyncExitCode = &worst
		if !ok {
			return false
		}
	}
	if worst != 0 && worst != rsyncVanished {
		r.fail(ui.RsyncFailed(worst))
	}
	return true
}

// transferAnimal runs rsync for the files of one animal folder and returns
// its exit code. ok is false if the run must stop: it was interrupted, or
// rsync could not be started.
func (r *run) transferAnimal(ctx context.Context, rsyncPath, animal string, files []SourceFile, plan Plan) (code int, ok bool) {
	to, err := destination.MakeAnimalDirs(r.s.Destination, animal, r.s.Subfolder)
	if err != nil {
		// The files of this animal are reported as not copied when verified.
		r.fail(ui.AnimalDirFailed(filepath.Join(r.s.Destination, animal, r.s.Subfolder), err))
		return 0, true
	}
	historyDir := filepath.Join(destination.HistoryDir(r.s.MetaDir, r.o.RunID), animal)
	sizes := map[string]int64{}
	rest := make([]string, len(files))
	replacing := false
	for i, f := range files {
		sizes[f.Path] = f.Size
		_, rest[i] = destination.SplitAnimal(f.Path)
		replacing = replacing || plan.Replacing[f.Path]
	}
	if replacing {
		if historyDir, err = destination.MakeHistoryDir(r.s.MetaDir, r.o.RunID, animal); err != nil {
			r.fail(ui.MetaDirFailed(r.s.MetaDir, err))
			return 0, false
		}
		r.sum.HistoryDir = destination.HistoryDir(r.s.MetaDir, r.o.RunID)
	}
	copied := map[string]bool{}
	job := rsyncJob{path: rsyncPath, files: rest, env: rsyncEnv(r.s.StateDir), lockFiles: r.o.LockFiles,
		grace: r.o.Grace, args: RsyncArgs(r.s.Existing, historyDir, filepath.Join(r.s.Source, animal), to)}
	job.onStdout = func(line string) {
		r.log.Info("rsync: %s", line)
		item, ok := ParseItemized(line)
		if !ok {
			if r.o.Verbose != nil {
				r.o.Verbose(line)
			}
			return
		}
		name := animal + "/" + item.Name
		if _, planned := sizes[name]; planned && item.Received() && !copied[name] {
			copied[name] = true
			r.sum.Copied++
			r.sum.CopiedBytes += sizes[name]
			r.event(Event{Kind: EventCopied, Path: name, Size: sizes[name],
				Done: r.sum.Copied, Total: len(plan.Copy)})
		}
	}
	job.onStderr = func(line string) {
		r.log.Warn("rsync: %s", line)
		if r.o.Verbose != nil {
			r.o.Verbose(line)
		}
	}
	r.log.Info("Running: %s %s", rsyncPath, strings.Join(job.args, " "))
	code, err = runRsync(ctx, job)
	if err != nil {
		r.fail(ui.RsyncStartFailed(rsyncPath, err))
		return code, false
	}
	if ctx.Err() != nil {
		r.log.Warn("rsync stopped (exit code %d); this run is not verified", code)
		return code, false
	}
	return code, true
}

// verifyCopies checks every file on the copy list with SHA-256.
func (r *run) verifyCopies(ctx context.Context, files []SourceFile) {
	for i, f := range files {
		if ctx.Err() != nil {
			return
		}
		r.phase("verifying", fmt.Sprintf("%s of %s", ui.Count(i+1), ui.Files(len(files))))
		c := VerifyFile(ctx, r.s, f)
		switch {
		case c.SHA256 != "":
			r.record(f, c.SHA256, i+1, len(files))
		case c.Defer != "":
			delete(r.pending, f.Path)
			r.deferFile(f, c.Defer)
		case ctx.Err() == nil:
			r.fail(errors.New(ui.VerifyFailed(f.Path, c.Err)))
		}
	}
}

// record adds a verified file to the manifest and the summary. done and
// total say where the file is in the current step, for the progress lines.
func (r *run) record(f SourceFile, sum string, done, total int) {
	// SAFETY: invariant 4 ("copied" means verified): only called after the SHA-256 of
	// source and destination copy matched and the source was unchanged.
	err := r.writer.Append(destination.Entry{Path: f.Path, Size: f.Size, MTime: f.MTime.UTC(),
		SHA256: sum, VerifiedAt: time.Now().UTC(), RunID: r.o.RunID})
	if err != nil {
		r.fail(ui.ManifestUnwritable(destination.ManifestPath(r.s.MetaDir), err))
		return
	}
	r.appended++
	r.verified[f.Path] = true
	r.sum.Verified++
	r.sum.VerifiedBytes += f.Size
	r.event(Event{Kind: EventVerified, Path: f.Path, Size: f.Size, Done: done, Total: total})
}

// deferFile records a file left for a later run.
func (r *run) deferFile(f SourceFile, reason string) {
	r.log.Warn("Deferred %s: %s", f.Path, reason)
	r.sum.Deferred = append(r.sum.Deferred, destination.DeferredFile{Path: f.Path, Reason: reason, ModifiedAt: f.MTime})
	r.event(Event{Kind: EventDeferred, Path: f.Path, Size: f.Size, Reason: reason})
}
