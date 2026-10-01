// This file runs one sync from start to finish and produces its summary. The
// steps are: preflight checks, scan the source, plan, check existing files
// (when asked to verify everything), copy with rsync, verify each copy, and
// decide the result (OK, PARTIAL, FAILED or INTERRUPTED). Each step lives in
// its own file; this one only puts them in order.
package engine

import (
	"context"
	"errors"
	"os"
	"sort"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/archive"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/state"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// Options describe one run.
type Options struct {
	Settings config.Settings
	RunID    string
	Version  string
	Log      *state.Logger
	// LockFiles are handed to rsync so the locks stay held while it runs.
	LockFiles []*os.File
	// PendingVerify lists files an earlier run copied but did not verify.
	PendingVerify []string
	// Progress, if set, is told about each phase and progress within it.
	Progress func(phase, detail string)
	// Verbose, if set, receives every rsync output line.
	Verbose func(line string)
	// Grace returns how long rsync gets after SIGINT before SIGKILL.
	Grace func() time.Duration
}

// Outcome is what a run leaves behind.
type Outcome struct {
	Summary archive.RunSummary
	// PendingVerify lists files that were copied but not verified (the run
	// was interrupted or verification failed); the next run checks them.
	PendingVerify []string
}

// run holds the working state of one run.
type run struct {
	o        Options
	s        config.Settings
	sum      archive.RunSummary
	log      *state.Logger
	manifest archive.Manifest
	writer   *archive.ManifestWriter
	previous map[string]bool // files earlier runs copied but did not verify
	pending  map[string]bool // files this run leaves unverified
	verified map[string]bool // verified in this run
	dropped  map[string]bool // previous entries this run settled otherwise
	appended int             // manifest lines added in this run
}

// Run performs one sync. It never returns an error: every problem is
// recorded in the summary, whose Result says how the run ended.
func Run(ctx context.Context, o Options) Outcome {
	r := &run{o: o, s: o.Settings, log: o.Log, previous: map[string]bool{},
		pending: map[string]bool{}, verified: map[string]bool{}, dropped: map[string]bool{}}
	if r.o.Grace == nil {
		r.o.Grace = func() time.Duration { return KillGrace }
	}
	for _, p := range o.PendingVerify {
		r.previous[p] = true
	}
	host, _ := os.Hostname()
	r.sum = archive.RunSummary{SchemaVersion: archive.SummarySchemaVersion, RunID: o.RunID,
		Machine: r.s.MachineName, Hostname: host, Profile: r.s.Profile, Version: o.Version,
		Source: r.s.Source, Archive: r.s.Archive, StartedAt: time.Now().UTC(), DryRun: r.s.DryRun,
		Existing: r.s.Existing, Verify: r.s.Verify}
	r.log.Info("Run started: %s -> %s (existing=%s, verify=%s, dry_run=%t)",
		r.s.Source, r.s.MachineDir, r.s.Existing, r.s.Verify, r.s.DryRun)
	r.execute(ctx)
	return r.finish(ctx)
}

// execute runs the steps in order, stopping at the first one that fails.
func (r *run) execute(ctx context.Context) {
	r.phase("preflight", "")
	rsyncPath, err := Preflight(r.s)
	if err != nil {
		r.fail(err)
		return
	}
	if !r.s.DryRun {
		release, ok := r.lockArchive()
		if !ok {
			return
		}
		defer release()
	}
	r.phase("scanning", "")
	scan, err := ScanSource(ctx, r.s.Source, "", r.s.Exclude, r.s.SettleTime, time.Now())
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		r.fail(ui.SourceUnreadable(r.s.Source, err))
		return
	}
	r.recordScan(scan)
	if r.manifest, err = archive.LoadManifest(r.s.MachineDir); err != nil {
		r.fail(ui.ManifestUnreadable(archive.ManifestPath(r.s.MachineDir), err))
		return
	}
	r.phase("planning", "")
	plan := MakePlan(r.s, scan.Files, r.manifest)
	if r.s.DryRun {
		r.recordPlan(plan)
		r.sum.WouldCopy = Paths(plan.Copy)
		return
	}
	if r.writer, err = archive.OpenManifestWriter(r.s.MachineDir); err != nil {
		r.fail(ui.ManifestUnwritable(archive.ManifestPath(r.s.MachineDir), err))
		return
	}
	r.checkExisting(ctx, &plan)
	r.recordPlan(plan)
	if ctx.Err() != nil {
		return
	}
	if len(plan.Copy) > 0 {
		r.phase("copying", ui.Files(len(plan.Copy))+" ("+ui.Size(plan.CopyBytes)+")")
		if !r.transfer(ctx, rsyncPath, plan) {
			return
		}
	}
	r.verifyCopies(ctx, plan.Copy)
}

// lockArchive creates the machine folder's metadata folders and takes the
// same-machine archive lock. The returned function releases it.
func (r *run) lockArchive() (func(), bool) {
	if err := archive.EnsureMetaDirs(r.s.Archive, r.s.MachineDir); err != nil {
		r.fail(ui.MetaDirFailed(r.s.MachineDir, err))
		return nil, false
	}
	lock, err := state.Acquire(archive.LockPath(r.s.MachineDir))
	switch {
	case errors.Is(err, state.ErrLocked):
		r.fail(ui.ArchiveBusy(r.s.MachineDir))
		return nil, false
	case errors.Is(err, state.ErrLockUnsupported):
		r.log.Warn("%s", ui.ArchiveLockUnsupported(archive.LockPath(r.s.MachineDir)))
		return func() {}, true
	case err != nil:
		r.fail(ui.MetaDirFailed(r.s.MachineDir, err))
		return nil, false
	}
	r.o.LockFiles = append(r.o.LockFiles, lock.File)
	return lock.Release, true
}

// finish closes the manifest and decides the result.
func (r *run) finish(ctx context.Context) Outcome {
	if err := r.writer.Close(); err != nil {
		r.fail(ui.ManifestUnwritable(archive.ManifestPath(r.s.MachineDir), err))
	}
	if r.appended > 0 {
		r.manifest.Lines += r.appended
		if r.manifest.NeedsCompaction() {
			if err := archive.Compact(r.s.MachineDir); err != nil {
				r.log.Warn("Could not compact the manifest: %v", err)
			}
		}
	}
	r.sum.DeferredCount = len(r.sum.Deferred)
	r.sum.DifferingCount = len(r.sum.Differing)
	r.sum.SkippedCount = len(r.sum.Skipped)
	r.sum.ErrorCount = len(r.sum.Errors)
	switch {
	case ctx.Err() != nil:
		r.sum.Result = archive.ResultInterrupted
	case r.sum.ErrorCount > 0:
		r.sum.Result = archive.ResultFailed
	case r.sum.DeferredCount > 0:
		r.sum.Result = archive.ResultPartial
	default:
		r.sum.Result = archive.ResultOK
	}
	r.sum.ExitCode = archive.ExitCode(r.sum.Result)
	r.sum.FinishedAt = time.Now().UTC()
	r.log.Info("Run finished: %s (copied %d, verified %d, deferred %d, differing %d, errors %d)",
		r.sum.Result, r.sum.Copied, r.sum.Verified, r.sum.DeferredCount, r.sum.DifferingCount, r.sum.ErrorCount)
	out := Outcome{Summary: r.sum}
	for p := range r.previous {
		if !r.dropped[p] {
			r.pending[p] = true
		}
	}
	for p := range r.pending {
		if !r.verified[p] {
			out.PendingVerify = append(out.PendingVerify, p)
		}
	}
	sort.Strings(out.PendingVerify)
	return out
}

// fail records a problem that stops or fails the run. A Problem keeps its
// full explanation (why it matters, what to do) for the report.
func (r *run) fail(err error) {
	text := err.Error()
	var p *ui.Problem
	if errors.As(err, &p) {
		text = ui.Explain(err)
	}
	r.sum.Errors = append(r.sum.Errors, text)
	r.log.Error("%s", err.Error())
}

// phase announces the start of a step.
func (r *run) phase(name, detail string) {
	if detail != "" {
		r.log.Info("Phase: %s (%s)", name, detail)
	} else {
		r.log.Info("Phase: %s", name)
	}
	if r.o.Progress != nil {
		r.o.Progress(name, detail)
	}
}
