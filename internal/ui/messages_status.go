// This file holds the wording of the read-only reports: status, stop, logs,
// verify, check-copied, history and fleet. See messages.go for how the
// message files are organised.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/state"
)

// Short messages.
const (
	NoDifferingFiles = "No files differ between the source and ceph (as of the last run)."
	NoDeferredFiles  = "No files are deferred (as of the last run)."
	VerifyStarting   = "Verifying source files against their copies on ceph (SHA-256)..."
	NoServiceFound   = "No automatic runs were found."
)

// SeeHelp points to a command's help after a usage error.
func SeeHelp(command string) string { return "See: " + command + " --help" }

// NextRunLine announces the next scheduled run.
func NextRunLine(due time.Time) string {
	return fmt.Sprintf("Next run at %s (%s).", Time(due), Ago(due, time.Now()))
}

// DifferingHeader introduces the list of differing files.
func DifferingHeader(n int) string {
	return fmt.Sprintf("%s on ceph %s from the source and %s NOT replaced\n"+
		"(keep both copies with: synctoceph run --existing replace):", Files(n), verb(n, "differs", "differ"), verb(n, "was", "were"))
}

// DeferredEntry describes one deferred file.
func DeferredEntry(d destination.DeferredFile, now time.Time) string {
	line := fmt.Sprintf("  %s   %s", d.Path, d.Reason)
	if !d.Since.IsZero() {
		line += ", deferred since " + Time(d.Since)
		if now.Sub(d.Since) > EscalateAfter {
			line += "  (WARNING: over 24 hours)"
		}
	}
	return line
}

// NoLogLines says the log has nothing to show.
func NoLogLines(runID string) string {
	if runID != "" {
		return "No log lines for run " + runID + "."
	}
	return "The log is empty; nothing has run for this profile yet."
}

// NothingRunning says there is nothing to stop.
func NothingRunning(profile string) string {
	return "Nothing is running for profile " + profile + "."
}

// StopRequested confirms that a stop was requested.
func StopRequested(profile string) string {
	return "Asked synctoceph (profile " + profile + ") to stop; waiting for it to finish cleanly..."
}

// Stopped confirms that the process ended.
func Stopped(profile string) string { return "Stopped (profile " + profile + ")." }

// StopUnreachable: the lock is held but nothing answers on the socket.
func StopUnreachable(profile string, pid int) error {
	fix := "if a copy is still finishing, wait and try again"
	if pid > 0 {
		fix += fmt.Sprintf("; otherwise stop it with `kill -TERM %d`", pid)
	}
	return &Problem{What: "synctoceph is running for profile " + profile + " but does not answer on its control socket",
		Why: "the stop request could not be delivered", Fix: fix}
}

// StopTimedOut: the process did not exit in time.
func StopTimedOut(profile string) error {
	return &Problem{What: "synctoceph (profile " + profile + ") has not stopped yet",
		Why: "rsync may still be finishing or exiting",
		Fix: "check with `synctoceph status`, or run `synctoceph stop` again"}
}

// StatusReport prints `synctoceph status`.
func StatusReport(p *Printer, profile, current string, live *state.Live, st state.Status, now time.Time) {
	p.Plain(fmt.Sprintf("Profile:    %s", profile))
	stateText := current
	if live != nil && live.Phase != "" && live.Phase != "waiting" {
		stateText += " (" + live.Mode + ": " + live.Phase
		if live.Progress != "" {
			stateText += ", " + live.Progress
		}
		stateText += ")"
	}
	p.Plain("State:      " + stateText)
	if st.Schedule != "" {
		p.Plain("Schedule:   " + st.Schedule)
	}
	if current == state.StateWaiting && st.NextRunAt != nil {
		p.Plain("Next run:   " + Time(*st.NextRunAt) + " (" + Ago(*st.NextRunAt, now) + ")")
	}
	if st.LastSuccessAt != nil {
		p.Plain("Last successful sync: " + Time(*st.LastSuccessAt) + " (" + Ago(*st.LastSuccessAt, now) + ")")
	}
	r := st.LastRun
	if r == nil {
		p.Plain("Last run:   none yet")
		return
	}
	kind := ""
	if r.DryRun {
		kind = " (dry run)"
	}
	p.Plain(fmt.Sprintf("Last run:   %s%s, finished %s (run %s)", r.Result, kind, Ago(r.FinishedAt, now), r.RunID))
	p.Plain(fmt.Sprintf("            copied %s (%s), verified %s, already on ceph %s",
		Files(r.Copied), Size(r.CopiedBytes), Files(r.Verified), Files(r.UpToDate)))
	if r.DeferredCount > 0 {
		p.Line(MarkDeferred, fmt.Sprintf("%s not copied yet. List: synctoceph status --deferred", Files(r.DeferredCount)))
		if oldest := r.OldestDeferral(); !oldest.IsZero() && now.Sub(oldest) > EscalateAfter {
			p.Line(MarkWarning, StuckDeferral(oldest, now))
		}
	}
	if r.DifferingCount > 0 {
		p.Line(MarkNote, fmt.Sprintf("%s %s from the copy on ceph and %s not replaced. List: synctoceph status --differing",
			Files(r.DifferingCount), verb(r.DifferingCount, "differs", "differ"), verb(r.DifferingCount, "was", "were")))
	}
	if r.Unverified > 0 {
		p.Line(MarkNote, fmt.Sprintf("%s on ceph not verified yet. Check: synctoceph verify", Files(r.Unverified)))
	}
	for i, e := range r.Errors {
		if i == listLimit {
			p.Line(MarkError, fmt.Sprintf("... and %d more; see synctoceph logs --run %s", r.ErrorCount-listLimit, r.RunID))
			break
		}
		p.Line(MarkError, e)
	}
}

// VerifyReport prints the result of `synctoceph verify`.
func VerifyReport(p *Printer, r destination.VerifyReport, interrupted, all bool) {
	if r.Verified > 0 {
		p.Line(MarkOK, fmt.Sprintf("Verified %s (%s): source and ceph copies match.", Files(r.Verified), Size(r.VerifiedBytes)))
	}
	if len(r.Differing) > 0 {
		p.Line(MarkError, fmt.Sprintf("%s on ceph %s from the source:\n", Files(len(r.Differing)),
			verb(len(r.Differing), "differs", "differ"))+
			fileList(r.Differing, len(r.Differing), all, "")+"\n"+
			"Keep both copies with: synctoceph run --existing replace --verify all")
	}
	if len(r.NotCopied) > 0 {
		p.Line(MarkNote, fmt.Sprintf("%s not on ceph yet (the next run copies them):\n", Files(len(r.NotCopied)))+
			fileList(r.NotCopied, len(r.NotCopied), all, ""))
	}
	if len(r.Changed) > 0 {
		p.Line(MarkDeferred, fmt.Sprintf("%s changed while being checked; run verify again later:\n", Files(len(r.Changed)))+
			fileList(r.Changed, len(r.Changed), all, ""))
	}
	for _, e := range r.Errors {
		p.Line(MarkError, e)
	}
	switch {
	case interrupted:
		p.Line(MarkResult, "INTERRUPTED: verification was stopped; files checked so far are recorded.")
	case len(r.Differing)+len(r.NotCopied)+len(r.Changed)+len(r.Errors) == 0:
		p.Line(MarkResult, "OK: every file checked is verified.")
	default:
		p.Line(MarkResult, "NOT ALL VERIFIED: see the lines above.")
	}
}

// CheckCopiedReport prints the result of `synctoceph check-copied`.
func CheckCopiedReport(p *Printer, path string, files []destination.FileStatus, safe, all bool) {
	counts := map[string]int{}
	for _, f := range files {
		counts[f.Status]++
		if f.Status != destination.StatusVerified || all {
			p.Plain(fmt.Sprintf("  %-13s %s", strings.ToUpper(f.Status), f.Path))
		}
	}
	if safe {
		p.Line(MarkResult, fmt.Sprintf("SAFE: every file under %s (%s) is copied to ceph and verified.\n"+
			"It can be deleted from this computer.", path, Files(len(files))))
		return
	}
	if len(files) == 0 {
		p.Line(MarkResult, "NOT SAFE: no files were found under "+path+".")
		return
	}
	p.Line(MarkResult, fmt.Sprintf("NOT SAFE: %s of %s under %s %s not verified on ceph.\n"+
		"Do not delete %s. %s", Count(len(files)-counts[destination.StatusVerified]), Files(len(files)), path,
		verb(len(files)-counts[destination.StatusVerified], "is", "are"), path, checkCopiedAdvice(counts)))
}

// checkCopiedAdvice suggests what to do about files that are not verified.
func checkCopiedAdvice(counts map[string]int) string {
	var tips []string
	if counts[destination.StatusNotCopied] > 0 {
		tips = append(tips, "Not copied: run synctoceph run.")
	}
	if counts[destination.StatusUnverified] > 0 {
		tips = append(tips, "Unverified: run synctoceph verify PATH.")
	}
	if counts[destination.StatusDiffers] > 0 {
		tips = append(tips, "Differs: ceph has another version; see synctoceph status --differing.")
	}
	if counts[destination.StatusExcluded]+counts[destination.StatusSkipped] > 0 {
		tips = append(tips, "Excluded/skipped entries are never copied; keep or move them yourself.")
	}
	return strings.Join(tips, "\n")
}
