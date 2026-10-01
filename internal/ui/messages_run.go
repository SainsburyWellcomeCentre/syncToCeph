// This file holds the wording of a run's report: what was copied and
// verified, what was deferred or skipped, what differs, and the final RESULT
// line. See messages.go for how the message files are organised.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/archive"
)

// listLimit is how many file names a report shows before "... and N more".
const listLimit = 10

// escalateAfter is how long a file may stay deferred before it is shown as a
// warning rather than a normal deferral.
const EscalateAfter = 24 * time.Hour

// PhaseLine returns the terminal line announcing a phase, or "" for phases
// that are not announced.
func PhaseLine(phase, detail string) string {
	switch phase {
	case "scanning":
		return "Scanning the source..."
	case "copying":
		return "Copying " + detail + "..."
	case "checking existing":
		return "Checking files already in the archive (SHA-256)..."
	case "verifying":
		return "Verifying copies (SHA-256)..."
	}
	return ""
}

// RunReport prints the report of a finished run. settle is the configured
// settle time, mentioned in the deferral explanation.
func RunReport(p *Printer, s archive.RunSummary, settle time.Duration, all bool) {
	took := Duration(s.FinishedAt.Sub(s.StartedAt))
	switch {
	case s.DryRun:
		p.Line(MarkNote, "Dry run: nothing was written to the archive.")
		if len(s.WouldCopy) > 0 {
			p.Line(MarkOK, fmt.Sprintf("Would copy %s (%s):", Files(s.Planned), Size(s.PlannedBytes))+"\n"+
				fileList(s.WouldCopy, len(s.WouldCopy), all, ""))
		} else if s.Result != archive.ResultFailed {
			p.Line(MarkOK, "Nothing to copy.")
		}
	case s.Copied > 0 && s.Copied == s.Verified:
		p.Line(MarkOK, fmt.Sprintf("Copied and verified %s (%s) in %s", Files(s.Copied), Size(s.VerifiedBytes), took))
	case s.Copied == 0 && s.Verified > 0:
		p.Line(MarkOK, fmt.Sprintf("Nothing new to copy; checked %s already in the archive (%s) with SHA-256 in %s: they match.",
			Files(s.Verified), Size(s.VerifiedBytes), took))
	case s.Copied > 0 || s.Verified > 0:
		p.Line(MarkOK, fmt.Sprintf("Copied %s; verified %s (%s) in %s",
			Files(s.Copied), Files(s.Verified), Size(s.VerifiedBytes), took))
	case s.Result == archive.ResultOK || s.Result == archive.ResultPartial:
		p.Line(MarkOK, fmt.Sprintf("Nothing new to copy; %s already archived and verified.", Files(s.UpToDate)))
	}
	if s.DeferredCount > 0 {
		deferredBlock(p, s, settle, all)
	}
	if s.DifferingCount > 0 {
		p.Line(MarkNote, fmt.Sprintf("%s already in the archive %s from the source and %s NOT replaced.",
			Files(s.DifferingCount), verb(s.DifferingCount, "differs", "differ"), verb(s.DifferingCount, "was", "were"))+"\n"+
			"List them:        synctoceph status --differing\n"+
			"Keep both copies: synctoceph run --existing replace   (old versions go to history)")
	}
	if s.Unverified > 0 && s.NewUnverified {
		p.Line(MarkNote, fmt.Sprintf("%s in the archive %s the same size and time as the source, but synctoceph has not\n"+
			"checked %s contents yet (probably copied before synctoceph was used).\n"+
			"Check once with: synctoceph verify", Files(s.Unverified), verb(s.Unverified, "has", "have"), verb(s.Unverified, "its", "their")))
	}
	if s.SkippedCount > 0 {
		p.Line(MarkNote, fmt.Sprintf("%s skipped (never copied):\n", Files(s.SkippedCount))+skippedList(s.Skipped, s.SkippedCount, all))
	}
	for i, e := range s.Errors {
		if i == listLimit && !all {
			p.Line(MarkError, fmt.Sprintf("... and %d more; see synctoceph logs --run %s", s.ErrorCount-listLimit, s.RunID))
			break
		}
		p.Line(MarkError, e)
	}
	p.Line(MarkResult, ResultLine(s))
}

// deferredBlock prints the DEFERRED explanation.
func deferredBlock(p *Printer, s archive.RunSummary, settle time.Duration, all bool) {
	now := s.FinishedAt
	var lines []string
	for i, d := range s.Deferred {
		if i == listLimit && !all {
			lines = append(lines, fmt.Sprintf("  ... and %d more: synctoceph status --deferred", s.DeferredCount-listLimit))
			break
		}
		detail := d.Reason
		if d.Reason == archive.ReasonSettling && !d.ModifiedAt.IsZero() {
			detail = "modified " + Ago(d.ModifiedAt, now)
		}
		lines = append(lines, fmt.Sprintf("  %s   %s", d.Path, detail))
	}
	text := fmt.Sprintf("%s changed recently or during the sync (probably still being acquired):\n", Files(s.DeferredCount)) +
		strings.Join(lines, "\n") + "\n" +
		fmt.Sprintf("What happens next: they will be copied on a later run, once unchanged for %s\n", Duration(settle)) +
		"(setting: settle_time).\n" +
		"What to do: nothing if acquisition is still running. If it has finished and these\n" +
		"files keep appearing, check that no program still has them open."
	p.Line(MarkDeferred, text)
	if oldest := s.OldestDeferral(); !oldest.IsZero() && now.Sub(oldest) > EscalateAfter {
		p.Line(MarkWarning, StuckDeferral(oldest, now))
	}
}

// StuckDeferral warns about files deferred for more than a day.
func StuckDeferral(oldest, now time.Time) string {
	return fmt.Sprintf("Some files have been deferred for %s (since %s).\n", Duration(now.Sub(oldest)), Time(oldest)) +
		"What to do: close any program that may still be writing them, or check that the\n" +
		"computer's clock is right. List them: synctoceph status --deferred"
}

// ResultLine returns the text of the final RESULT line.
func ResultLine(s archive.RunSummary) string {
	switch s.Result {
	case archive.ResultOK:
		if s.DryRun {
			return "OK (dry run): nothing was changed."
		}
		return "OK: everything that needed copying is archived and verified."
	case archive.ResultPartial:
		return fmt.Sprintf("PARTIAL: %s %s not archived yet. Do not delete %s from the source.",
			Files(s.DeferredCount), verb(s.DeferredCount, "is", "are"), verb(s.DeferredCount, "it", "them"))
	case archive.ResultInterrupted:
		return "INTERRUPTED: the run was stopped before it finished; nothing from it is reported as\n" +
			"verified. The next run continues where this one stopped."
	}
	return fmt.Sprintf("FAILED: %s. Files named above are not archived.\nDetails: synctoceph logs --run %s",
		plural(s.ErrorCount, "problem"), s.RunID)
}

// fileList formats up to listLimit paths (all if all is true).
func fileList(paths []string, total int, all bool, indent string) string {
	var lines []string
	for i, path := range paths {
		if i == listLimit && !all {
			lines = append(lines, fmt.Sprintf("%s  ... and %d more (use -v to list all)", indent, total-listLimit))
			break
		}
		lines = append(lines, indent+"  "+path)
	}
	return strings.Join(lines, "\n")
}

// skippedList formats skipped entries with their reasons.
func skippedList(skipped []archive.SkippedFile, total int, all bool) string {
	var lines []string
	for i, s := range skipped {
		if i == listLimit && !all {
			lines = append(lines, fmt.Sprintf("  ... and %d more (see synctoceph logs)", total-listLimit))
			break
		}
		lines = append(lines, fmt.Sprintf("  %s   %s", s.Path, s.Reason))
	}
	return strings.Join(lines, "\n")
}

// verb picks the singular or plural word for n.
func verb(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// plural returns "1 problem" or "N problems".
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return Count(n) + " " + word + "s"
}
