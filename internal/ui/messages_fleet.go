// This file holds the wording of `synctoceph history` and `synctoceph
// fleet`. See messages.go for how the message files are organised.
package ui

import (
	"fmt"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/archive"
)

// staleAfter is when a machine's last successful sync is flagged as old in
// `fleet`.
const staleAfter = 48 * time.Hour

// HistoryUnavailable: history cannot be read.
func HistoryUnavailable(err error) error {
	return &Problem{What: "cannot read the history", Err: err,
		Fix: "check the run ID with `synctoceph history list`, and that the archive is mounted"}
}

// HistoryRuns lists the runs that kept previous versions.
func HistoryRuns(p *Printer, root string, runs []archive.HistoryRun) {
	if len(runs) == 0 {
		p.Plain("No previous versions are kept (nothing has been replaced).")
		return
	}
	p.Plain("Previous versions in " + root + ":")
	for _, r := range runs {
		p.Plain(fmt.Sprintf("  %s   %s (%s)", r.RunID, Files(r.Files), Size(r.Bytes)))
	}
	p.Plain("List one run:  synctoceph history list RUN_ID\n" +
		"Restore:       synctoceph history restore RUN_ID PATH --to DIR")
}

// HistoryFiles lists the previous versions kept by one run.
func HistoryFiles(p *Printer, runID string, files []archive.HistoryFile) {
	p.Plain("Previous versions kept by run " + runID + ":")
	for _, f := range files {
		p.Plain(fmt.Sprintf("  %s   %s, modified %s", f.Path, Size(f.Size), Time(f.MTime)))
	}
}

// RestoreFailed: a restore did not complete.
func RestoreFailed(err error) error {
	return &Problem{What: "restore did not complete", Err: err,
		Why: "files listed above were restored; the rest were not",
		Fix: "restore into an empty folder outside the archive; existing files are never overwritten"}
}

// Restored confirms a restore.
func Restored(n int, dir string) string {
	return fmt.Sprintf("Restored %s into %s (each checked with SHA-256).", Files(n), dir)
}

// FleetUnavailable: the archive root cannot be listed.
func FleetUnavailable(root string, err error) error {
	return &Problem{What: "cannot read the archive " + root, Err: err,
		Fix: "check that the lab share is mounted, or give the archive with --archive ROOT"}
}

// FleetReport prints one block per machine.
func FleetReport(p *Printer, root string, machines []archive.MachineReport, now time.Time) {
	if len(machines) == 0 {
		p.Plain("No machine in " + root + " has run synctoceph yet.")
		return
	}
	p.Plain(fmt.Sprintf("%s in %s:", plural(len(machines), "machine"), root))
	for _, m := range machines {
		if m.Latest == nil {
			p.Line(MarkError, m.Machine+": cannot read its run summary ("+m.Error+")")
			continue
		}
		r := m.Latest
		success := "never"
		if r.LastSuccessAt != nil {
			success = Ago(*r.LastSuccessAt, now)
		}
		marker := MarkOK
		switch {
		case r.Result == archive.ResultFailed || r.Result == archive.ResultInterrupted:
			marker = MarkError
		case r.LastSuccessAt == nil || now.Sub(*r.LastSuccessAt) > staleAfter:
			marker = MarkWarning
		case r.DeferredCount > 0:
			marker = MarkDeferred
		}
		text := fmt.Sprintf("%s: last run %s %s (host %s); last successful sync %s",
			m.Machine, r.Result, Ago(r.FinishedAt, now), r.Hostname, success)
		if r.DeferredCount > 0 {
			text += fmt.Sprintf("\n%s deferred", Files(r.DeferredCount))
			if oldest := r.OldestDeferral(); !oldest.IsZero() && now.Sub(oldest) > EscalateAfter {
				text += fmt.Sprintf(" (WARNING: some for over 24 hours, since %s)", Time(oldest))
			}
		}
		if r.ErrorCount > 0 {
			text += fmt.Sprintf("\n%s in the last run; first: %s", plural(r.ErrorCount, "problem"), r.Errors[0])
		}
		p.Line(marker, text)
	}
}
