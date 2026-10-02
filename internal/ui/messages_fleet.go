// This file holds the wording of `synctoceph history` and `synctoceph
// fleet`. See messages.go for how the message files are organised.
package ui

import (
	"fmt"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
)

// staleAfter is when a subfolder's last successful sync is flagged as old in
// `fleet`.
const staleAfter = 48 * time.Hour

// HistoryUnavailable: history cannot be read.
func HistoryUnavailable(err error) error {
	return &Problem{What: "cannot read the history", Err: err,
		Fix: "check the run ID with `synctoceph history list`, and that ceph is mounted"}
}

// HistoryRuns lists the runs that kept previous versions.
func HistoryRuns(p *Printer, root string, runs []destination.HistoryRun) {
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
func HistoryFiles(p *Printer, runID string, files []destination.HistoryFile) {
	p.Plain("Previous versions kept by run " + runID + ":")
	for _, f := range files {
		p.Plain(fmt.Sprintf("  %s   %s, modified %s", f.Path, Size(f.Size), Time(f.MTime)))
	}
}

// RestoreFailed: a restore did not complete.
func RestoreFailed(err error) error {
	return &Problem{What: "restore did not complete", Err: err,
		Why: "files listed above were restored; the rest were not",
		Fix: "restore into an empty folder outside the destination; existing files are never overwritten"}
}

// Restored confirms a restore.
func Restored(n int, dir string) string {
	return fmt.Sprintf("Restored %s into %s (each checked with SHA-256).", Files(n), dir)
}

// FleetUnavailable: the destination cannot be read.
func FleetUnavailable(root string, err error) error {
	return &Problem{What: "cannot read the destination " + root, Err: err,
		Fix: "check that ceph is mounted, or give the folder with --destination DIR"}
}

// FleetReport prints one block per subfolder (usually one per acquisition
// machine).
func FleetReport(p *Printer, root string, subfolders []destination.SubfolderReport, now time.Time) {
	if len(subfolders) == 0 {
		p.Plain("No machine has copied into " + root + " with synctoceph yet.")
		return
	}
	p.Plain(fmt.Sprintf("%s in %s:", plural(len(subfolders), "subfolder"), root))
	for _, m := range subfolders {
		if m.Latest == nil {
			p.Line(MarkError, m.Subfolder+": cannot read its run summary ("+m.Error+")")
			continue
		}
		r := m.Latest
		success := "never"
		if r.LastSuccessAt != nil {
			success = Ago(*r.LastSuccessAt, now)
		}
		marker := MarkOK
		switch {
		case r.Result == destination.ResultFailed || r.Result == destination.ResultInterrupted:
			marker = MarkError
		case r.LastSuccessAt == nil || now.Sub(*r.LastSuccessAt) > staleAfter:
			marker = MarkWarning
		case r.DeferredCount > 0:
			marker = MarkDeferred
		}
		text := fmt.Sprintf("%s: last run %s %s (host %s); last successful sync %s",
			m.Subfolder, r.Result, Ago(r.FinishedAt, now), r.Hostname, success)
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
