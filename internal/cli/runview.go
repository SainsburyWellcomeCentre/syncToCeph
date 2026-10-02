// This file shows a run's progress in the terminal: the header saying which
// profile is copied from where to where, one "==>" line per step, a summary
// after the scan and the plan, and (when verbose) one line for each file
// copied, verified or deferred. The wording is in internal/ui.
package cli

import (
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/engine"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/scheduler"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// runView prints the progress of runs for one profile.
type runView struct {
	p *ui.Printer
	// everyFile lists every file copied, verified or deferred.
	everyFile bool
	// last is the step announced last. Steps report progress many times
	// (file 1 of 40, 2 of 40, ...); each step is announced once.
	last string
}

// header prints which profile is copied from where to where.
func (v *runView) header(command string, s config.Settings) {
	ui.RunHeader(v.p, command, s.Profile, s.Source, s.Target(),
		ui.RunOptions(s.Existing, s.Verify, s.SettleTime, s.DryRun))
}

// attach makes the controller report its progress to this view.
func (v *runView) attach(c *scheduler.Controller) {
	c.Report = v.phase
	c.Event = v.event
	if v.everyFile {
		c.Verbose = func(line string) { ui.RsyncMessage(v.p, line) }
	}
}

// phase announces a step the first time it is reported.
func (v *runView) phase(phase, detail string) {
	if phase == v.last {
		return
	}
	v.last = phase
	if line := ui.PhaseLine(phase, detail); line != "" {
		v.p.Step(line)
	}
}

// event shows the scan and plan summaries, and each file when verbose.
func (v *runView) event(e engine.Event) {
	switch e.Kind {
	case engine.EventScanned:
		v.p.Detail(ui.ScanDetail(e.Summary))
	case engine.EventPlanned:
		v.p.Step(ui.PlanStep)
		v.p.Detail(ui.PlanDetail(e.Summary))
	case engine.EventCopied:
		if v.everyFile {
			ui.FileProgress(v.p, "copied", e.Path, ui.Size(e.Size), e.Done, e.Total)
		}
	case engine.EventVerified:
		if v.everyFile {
			ui.FileProgress(v.p, "verified", e.Path, "", e.Done, e.Total)
		}
	case engine.EventDeferred:
		if v.everyFile {
			ui.FileProgress(v.p, "deferred", e.Path, e.Reason, 0, 0)
		}
	}
}

// report prints the result of a finished run, and gets ready for the next
// one (the scheduler runs many).
func (v *runView) report(sum destination.RunSummary, settle time.Duration) {
	v.last = ""
	v.p.Blank()
	ui.RunReport(v.p, sum, settle, v.everyFile)
}
