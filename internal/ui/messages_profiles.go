// This file holds the wording for working with several profiles (several
// config files, each with its own source and archive folder): the heading
// and closing summary of `run --all-profiles`, the table printed by
// `status --all-profiles`, and the related problems. See messages.go for how
// the message files are organised.
package ui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/archive"
)

// NoProfiles: --all-profiles was given but no config file exists.
func NoProfiles(configDir string) error {
	return &Problem{What: "no profiles found in " + configDir,
		Why: "--all-profiles works on every config file there, and there is none yet",
		Fix: "run `synctoceph init` to create the first one, and `synctoceph --profile NAME init` for more"}
}

// ProfileWithAllProfiles: both --profile and --all-profiles were given.
const ProfileWithAllProfiles = "--profile and --all-profiles cannot be used together"

// SharedMachineFolder warns that several profiles copy into one folder.
func SharedMachineFolder(machineDir string, profiles []string) string {
	sorted := slices.Sorted(slices.Values(profiles))
	return fmt.Sprintf("Profiles %s all copy into %s.\n", strings.Join(sorted, ", "), machineDir) +
		"Files with the same path in their sources would meet in one place: in skip mode the\n" +
		"later one is reported as differing; in replace mode it replaces the other (which is\n" +
		"kept in history). If the sources are unrelated, give each profile its own\n" +
		"machine_name (for example scope-01-ephys and scope-01-video)."
}

// ProfileOutcome is one profile's line in the summary printed after
// `run --all-profiles`.
type ProfileOutcome struct {
	Profile string
	Result  string // OK, PARTIAL, FAILED or INTERRUPTED
	Detail  string
}

// ProfileHeading separates the profiles of `--all-profiles` output.
func ProfileHeading(p *Printer, profile string, n, total int) {
	p.Plain(p.paint(styleBold, fmt.Sprintf("--- Profile %s (%d of %d) ---", profile, n, total)))
}

// BriefResult describes a run in a few words, for the summary of profiles.
func BriefResult(s archive.RunSummary) string {
	switch {
	case s.Result == archive.ResultFailed:
		return plural(s.ErrorCount, "problem") + "; see the report above"
	case s.Result == archive.ResultInterrupted:
		return "stopped before it finished"
	case s.DryRun:
		return fmt.Sprintf("would copy %s (%s)", Files(s.Planned), Size(s.PlannedBytes))
	}
	text := "nothing new to copy"
	if s.Copied > 0 {
		text = fmt.Sprintf("copied %s (%s)", Files(s.Copied), Size(s.CopiedBytes))
	}
	if s.DeferredCount > 0 {
		text += fmt.Sprintf("; %s left for a later run", Files(s.DeferredCount))
	}
	return text
}

// ProfilesSummary prints the closing summary of `run --all-profiles` and
// its RESULT line. It returns the overall result: the worst of all profiles.
// dryRun is true when every profile ran as a dry run.
func ProfilesSummary(p *Printer, outcomes []ProfileOutcome, total int, dryRun bool) string {
	p.Blank()
	p.Step(fmt.Sprintf("Summary of %s", plural(total, "profile")))
	width := 0
	for _, o := range outcomes {
		width = max(width, len(o.Profile))
	}
	counts := map[string]int{}
	for _, o := range outcomes {
		counts[o.Result]++
		result := p.paint(resultStyle(o.Result), o.Result) + strings.Repeat(" ", len(archive.ResultInterrupted)-len(o.Result))
		p.Detail(fmt.Sprintf("%-*s  %s  %s", width, o.Profile, result, o.Detail))
	}
	var result, text string
	switch {
	case counts[archive.ResultInterrupted] > 0:
		result = archive.ResultInterrupted
		text = fmt.Sprintf("INTERRUPTED: stopped after %d of %s; the rest were not run.", len(outcomes), plural(total, "profile"))
	case counts[archive.ResultFailed] > 0:
		result = archive.ResultFailed
		text = fmt.Sprintf("FAILED: %d of %s failed; see %s report above.", counts[archive.ResultFailed],
			plural(total, "profile"), verb(counts[archive.ResultFailed], "its", "their"))
	case counts[archive.ResultPartial] > 0:
		result = archive.ResultPartial
		text = fmt.Sprintf("PARTIAL: %d of %s left files for a later run. Do not delete those from the source.",
			counts[archive.ResultPartial], plural(total, "profile"))
	case dryRun:
		result = archive.ResultOK
		text = fmt.Sprintf("OK (dry run): all %s checked; nothing was changed.", plural(total, "profile"))
	default:
		result = archive.ResultOK
		text = fmt.Sprintf("OK: all %s finished; everything that needed copying is archived and verified.", plural(total, "profile"))
	}
	p.Line(MarkResult, text)
	return result
}

// ProfileRow is one line of `status --all-profiles`.
type ProfileRow struct {
	Profile    string
	State      string // idle, running, waiting or stopped
	LastRun    *archive.RunSummary
	Source     string
	MachineDir string
	Problem    string // set instead of Source and MachineDir when the config has a problem
}

// ProfilesTable prints `status --all-profiles`: one line per profile.
func ProfilesTable(p *Printer, rows []ProfileRow, now time.Time) {
	table := [][]string{{"PROFILE", "STATE", "LAST RUN", "SOURCE -> ARCHIVE FOLDER"}}
	for _, r := range rows {
		last := "none yet"
		if r.LastRun != nil {
			last = r.LastRun.Result
			if r.LastRun.DryRun {
				last += " (dry run)"
			}
			last += ", " + Ago(r.LastRun.FinishedAt, now)
		}
		where := r.Source + " -> " + r.MachineDir
		if r.Problem != "" {
			where = "config problem: " + r.Problem
		}
		table = append(table, []string{r.Profile, r.State, last, where})
	}
	widths := make([]int, len(table[0]))
	for _, row := range table {
		for i, cell := range row {
			widths[i] = max(widths[i], len(cell))
		}
	}
	for i, row := range table {
		cells := make([]string, len(row))
		for j, cell := range row {
			if j < len(row)-1 {
				cell = fmt.Sprintf("%-*s", widths[j], cell)
			}
			cells[j] = cell
		}
		line := strings.Join(cells, "  ")
		if i == 0 {
			line = p.paint(styleBold, line)
		}
		p.Plain(line)
	}
	p.Blank()
	p.Plain("Details of one profile: synctoceph --profile NAME status")
}

// ServiceSkipped: `service install --all-profiles` left out a profile,
// usually because it has no schedule.
func ServiceSkipped(profile string, err error) string {
	return "Profile " + profile + " was skipped: " + Explain(err)
}
