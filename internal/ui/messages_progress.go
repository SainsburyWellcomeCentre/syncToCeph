// This file holds the wording of what a run shows while it works: the header
// (which profile, from where to where), one "==>" line per step, a summary
// line after the scan and the plan, and, when verbose, one line per file
// copied, verified or deferred. See messages.go for how the message files
// are organised.
package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
)

// fileWordWidth pads "copied", "verified" and "deferred" so the paths after
// them line up.
const fileWordWidth = 9

// RunHeader prints the block that opens a run or a scheduler: the command,
// the profile, where data comes from and goes to, and the main settings.
func RunHeader(p *Printer, command, profile, source, target, options string) {
	p.Plain(p.paint(styleBold, fmt.Sprintf("synctoceph %s (profile %s)", command, profile)))
	for _, row := range [][2]string{{"From", source + "/<animal>"}, {"To", target}, {"Options", options}} {
		p.Plain("  " + p.paint(styleDim, fmt.Sprintf("%-8s", row[0])) + "  " + row[1])
	}
	p.Blank()
}

// RunOptions describes the settings that change what a run does.
func RunOptions(existing, verify string, settle time.Duration, dryRun bool) string {
	text := fmt.Sprintf("existing=%s  verify=%s  settle_time=%s", existing, verify, Duration(settle))
	if dryRun {
		text += "  (dry run: nothing is written)"
	}
	return text
}

// PhaseLine returns the step line announcing a phase, or "" for phases that
// are not announced.
func PhaseLine(phase, detail string) string {
	switch phase {
	case "preflight":
		return "Checking the destination, the source and rsync"
	case "scanning":
		return "Scanning the source"
	case "checking existing":
		return "Checking files already on ceph (SHA-256)"
	case "copying":
		return "Copying " + detail
	case "verifying":
		return "Verifying copies (SHA-256)"
	}
	return ""
}

// PlanStep announces the copy list. It is printed when the list is ready
// (after any SHA-256 check of existing files), together with PlanDetail.
const PlanStep = "Comparing with ceph"

// ScanDetail summarises what the scan found. The size counts only files
// ready to copy, not those still changing.
func ScanDetail(s destination.RunSummary) string {
	parts := []string{fmt.Sprintf("Found %s (%s)", Files(s.Scanned), Size(s.ScannedBytes))}
	if n := len(s.Deferred); n > 0 {
		parts = []string{"Found " + Files(s.Scanned),
			fmt.Sprintf("%s ready (%s)", Count(s.Scanned-n), Size(s.ScannedBytes)),
			Count(n) + " changed recently (left for a later run)"}
	}
	if s.Excluded > 0 {
		parts = append(parts, Count(s.Excluded)+" excluded")
	}
	if n := len(s.Skipped); n > 0 {
		parts = append(parts, Count(n)+" skipped (see the end of the report)")
	}
	return strings.Join(parts, "; ")
}

// PlanDetail summarises the copy list.
func PlanDetail(s destination.RunSummary) string {
	var parts []string
	if s.Planned > 0 {
		what := "to copy"
		if s.DryRun {
			what = "would be copied"
		}
		parts = append(parts, fmt.Sprintf("%s %s (%s)", Files(s.Planned), what, Size(s.PlannedBytes)))
	} else {
		parts = append(parts, "Nothing to copy")
	}
	if s.Replaced > 0 {
		parts = append(parts, fmt.Sprintf("%s replace an older copy on ceph (kept in history)", Count(s.Replaced)))
	}
	if s.UpToDate > 0 {
		parts = append(parts, Count(s.UpToDate)+" already copied and verified")
	}
	if s.Unverified > 0 {
		parts = append(parts, Count(s.Unverified)+" on ceph but not verified yet")
	}
	if n := len(s.Differing); n > 0 {
		parts = append(parts, fmt.Sprintf("%s %s from the copy on ceph (left as %s)", Count(n), verb(n, "differs", "differ"), verb(n, "it is", "they are")))
	}
	return strings.Join(parts, "; ")
}

// FileProgress prints one file line, e.g. "[ 3/40] copied    a/b.tif  1.2 MB".
// extra is the size or the reason; done and total may be 0 (no counter).
func FileProgress(p *Printer, kind, path, extra string, done, total int) {
	style := styleGood
	if kind == "deferred" {
		style = styleWait
	}
	line := ""
	if total > 0 {
		width := len(strconv.Itoa(total))
		line = p.paint(styleDim, fmt.Sprintf("[%*d/%d]", width, done, total)) + " "
	}
	line += p.paint(style, fmt.Sprintf("%-*s", fileWordWidth, kind)) + " " + showPath(path)
	if extra != "" {
		line += "  " + p.paint(styleDim, extra)
	}
	p.Detail(line)
}

// RsyncMessage prints a message from rsync (a warning or an error) under the
// current step.
func RsyncMessage(p *Printer, line string) {
	p.Detail(p.paint(styleDim, "rsync: "+line))
}

// showPath prints a file name on one line: names containing line breaks or
// other control characters are shown quoted, with the characters escaped.
func showPath(path string) string {
	for _, r := range path {
		if r < 0x20 || r == 0x7f {
			return strconv.Quote(path)
		}
	}
	return path
}
