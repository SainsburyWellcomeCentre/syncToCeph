// This file runs syncs through the controller: RunOnce for a single run and
// Loop for the scheduler. After each run it updates the bookkeeping kept
// between runs (when each deferred file was first deferred, files still
// waiting for verification) and saves the run summary locally and in the
// archive.
package scheduler

import (
	"os"
	"sort"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/archive"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/engine"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/state"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// waitPoll is the longest the scheduler sleeps before re-checking the wall
// clock. Timers can pause while a computer sleeps, so waiting is done in
// short steps compared against the real time of day.
const waitPoll = 10 * time.Second

// RunOnce performs one sync and records its result.
func (c *Controller) RunOnce() archive.RunSummary {
	runID := archive.NewRunID(time.Now())
	c.Log.SetRun(runID)
	defer c.Log.SetRun("")
	c.mu.Lock()
	c.live.RunID = runID
	c.status.State, c.status.NextRunAt = state.StateRunning, nil
	pending := c.status.PendingVerify
	c.writeStatusLocked()
	c.mu.Unlock()

	out := engine.Run(c.ctx, engine.Options{Settings: c.Settings, RunID: runID, Version: c.Version,
		Log: c.Log, LockFiles: []*os.File{c.lock.File}, PendingVerify: pending,
		Progress: c.setPhase, Verbose: c.Verbose, Grace: c.graceNow})
	sum := out.Summary

	c.mu.Lock()
	defer c.mu.Unlock()
	if sum.Scanned > 0 || sum.Succeeded() {
		c.status.DeferredSince = stampDeferred(sum.Deferred, c.status.DeferredSince, sum.FinishedAt)
	}
	sortDeferred(sum.Deferred)
	if !sum.DryRun && sum.Result != archive.ResultInterrupted {
		sum.NewUnverified = sum.Unverified > 0 && sum.Unverified != c.status.UnverifiedReported
		c.status.UnverifiedReported = sum.Unverified
	}
	c.status.PendingVerify = out.PendingVerify
	c.status.AddRun(sum)
	sum.LastSuccessAt = c.status.LastSuccessAt
	c.status.LastRun = &sum
	c.live.RunID, c.live.Phase = "", "finished"
	c.status.Phase = ""
	if !sum.DryRun {
		// A summary is written only once preflight has created the metadata
		// folder; SAFETY: invariant 9: never for a dry run.
		if info, err := os.Lstat(archive.RunsDir(c.Settings.MachineDir)); err == nil && info.IsDir() {
			if err := archive.WriteRunSummary(c.Settings.MachineDir, sum); err != nil {
				c.Log.Warn("%s", ui.SummaryNotWritten(err))
			}
		}
	}
	c.writeStatusLocked()
	return sum
}

// stampDeferred gives each deferred file the time it was first deferred,
// keeping earlier times from previous runs.
func stampDeferred(deferred []archive.DeferredFile, previous map[string]time.Time, now time.Time) map[string]time.Time {
	since := map[string]time.Time{}
	for i := range deferred {
		first, ok := previous[deferred[i].Path]
		if !ok {
			first = now
		}
		deferred[i].Since = first
		since[deferred[i].Path] = first
	}
	return since
}

// sortDeferred puts the longest-waiting files first, then sorts by path.
func sortDeferred(d []archive.DeferredFile) {
	sort.SliceStable(d, func(i, j int) bool {
		if !d[i].Since.Equal(d[j].Since) {
			return d[i].Since.Before(d[j].Since)
		}
		return d[i].Path < d[j].Path
	})
}

// Loop runs syncs on the configured schedule until a stop is requested. With
// immediate, the first run starts at once without using up that day's daily
// slot.
func (c *Controller) Loop(immediate bool) error {
	zone, err := Zone()
	if err != nil {
		return err
	}
	for first := true; c.ctx.Err() == nil; first = false {
		due, err := c.nextDue(first && immediate, zone)
		if err != nil {
			return err
		}
		c.mu.Lock()
		c.status.State, c.status.NextRunAt = state.StateWaiting, &due
		c.live.Phase = "waiting"
		c.writeStatusLocked()
		c.mu.Unlock()
		if time.Until(due) > 0 {
			c.Log.Info("Next run at %s", ui.Time(due))
			if c.Waiting != nil {
				c.Waiting(ui.NextRunLine(due))
			}
		}
		if !c.waitUntil(due) {
			return nil
		}
		if c.Settings.At != "" && !(first && immediate) {
			c.mu.Lock()
			c.status.LastDailyDate = due.In(zone).Format(time.DateOnly)
			c.status.LastDailyAt = c.Settings.At
			c.mu.Unlock()
		}
		sum := c.RunOnce()
		if c.AfterRun != nil {
			c.AfterRun(sum)
		}
	}
	return nil
}

// nextDue returns when the next run should start.
func (c *Controller) nextDue(now bool, zone *time.Location) (time.Time, error) {
	if now {
		return time.Now(), nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Settings.Interval > 0 {
		var lastEnd time.Time
		if c.status.LastRun != nil {
			lastEnd = c.status.LastRun.FinishedAt
		}
		return NextInterval(time.Now(), lastEnd, c.Settings.Interval), nil
	}
	hour, minute, err := config.ParseAt(c.Settings.At)
	if err != nil {
		return time.Time{}, err
	}
	last := ""
	if c.status.LastDailyAt == c.Settings.At {
		last = c.status.LastDailyDate
	}
	return NextDaily(time.Now(), hour, minute, zone, last)
}

// waitUntil sleeps until due. It returns false if a stop was requested.
func (c *Controller) waitUntil(due time.Time) bool {
	for {
		remaining := time.Until(due.Round(0))
		if remaining <= 0 {
			return true
		}
		timer := time.NewTimer(min(remaining, waitPoll))
		select {
		case <-c.ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}
	}
}

// Verify re-hashes archived files under sub (see engine.VerifyTree).
func (c *Controller) Verify(sub string, progress func(done, total int)) (archive.VerifyReport, error) {
	runID := archive.NewRunID(time.Now())
	c.Log.SetRun(runID)
	defer c.Log.SetRun("")
	c.setPhase("verifying", "")
	return engine.VerifyTree(c.ctx, c.Settings, sub, runID, c.Log, progress)
}
