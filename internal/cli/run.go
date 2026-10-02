// This file defines `synctoceph run` (one sync, for one profile or for every
// profile in turn) and `synctoceph schedule` (syncs on the configured
// schedule, used by the service).
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/archive"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/scheduler"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// runFlags are the flags of `run`.
type runFlags struct {
	dryRun      bool
	existing    string
	verify      string
	allProfiles bool
}

// apply puts the flags that were given on top of the config file.
func (f *runFlags) apply(cmd *cobra.Command) func(*config.Config) {
	return func(c *config.Config) {
		if cmd.Flags().Changed("dry-run") {
			c.DryRun = f.dryRun
		}
		if cmd.Flags().Changed("existing") {
			c.Existing = f.existing
		}
		if cmd.Flags().Changed("verify") {
			c.Verify = f.verify
		}
	}
}

func newRun(g *globals) *cobra.Command {
	f := &runFlags{}
	cmd := &cobra.Command{
		Use:   "run",
		Short: ui.RunShort,
		Long:  ui.RunLong,
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if f.allProfiles {
				return runAllProfiles(cmd, g, f)
			}
			sum, err := runProfile(cmd, g, f, g.profile)
			if err != nil {
				return err
			}
			return exitWith(sum.ExitCode)
		},
	}
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false, "show what would be copied; write nothing to the archive")
	cmd.Flags().StringVar(&f.existing, "existing", "", "`MODE` for files already in the archive that differ: skip or replace (default: the config's existing setting, skip unless changed)")
	cmd.Flags().StringVar(&f.verify, "verify", "", "`MODE` for SHA-256 checks: new (only files copied in this run) or all (default: the config's verify setting, new unless changed)")
	cmd.Flags().BoolVar(&f.allProfiles, "all-profiles", false, "run every profile, one after another")
	return cmd
}

// runProfile runs one sync for profile and prints its progress and report.
// An error means the run could not start (for example a config problem).
func runProfile(cmd *cobra.Command, g *globals, f *runFlags, profile string) (archive.RunSummary, error) {
	s, err := loadProfile(profile, f.apply(cmd))
	if err != nil {
		return archive.RunSummary{}, err
	}
	c, err := scheduler.Open(s, scheduler.ModeRun, Version)
	if err != nil {
		return archive.RunSummary{}, err
	}
	defer c.Close()
	defer stopOnSignal(c)()
	v := &runView{p: printer(cmd, g), everyFile: listEveryFile(cmd, g, s.Verbose)}
	v.header("run", s)
	v.attach(c)
	sum := c.RunOnce()
	v.report(sum, s.SettleTime)
	return sum, nil
}

// runAllProfiles runs every profile in turn, then prints a summary. A
// profile that cannot start is reported and the next one still runs; a stop
// request (Ctrl-C or synctoceph stop) ends the whole command.
func runAllProfiles(cmd *cobra.Command, g *globals, f *runFlags) error {
	profiles, err := allProfiles(cmd)
	if err != nil {
		return err
	}
	p := printer(cmd, g)
	problems := ui.NewPrinter(cmd.ErrOrStderr(), g.noColor, false)
	var outcomes []ui.ProfileOutcome
	allDry := true
	for i, profile := range profiles {
		if i > 0 {
			p.Blank()
		}
		ui.ProfileHeading(p, profile, i+1, len(profiles))
		sum, err := runProfile(cmd, g, f, profile)
		if err != nil {
			problems.Problem(err)
			first, _, _ := strings.Cut(err.Error(), "\n")
			outcomes = append(outcomes, ui.ProfileOutcome{Profile: profile, Result: archive.ResultFailed,
				Detail: fmt.Sprintf("could not start: %s", first)})
			allDry = false
			continue
		}
		outcomes = append(outcomes, ui.ProfileOutcome{Profile: profile, Result: sum.Result, Detail: ui.BriefResult(sum)})
		allDry = allDry && sum.DryRun
		if sum.Result == archive.ResultInterrupted {
			break
		}
	}
	result := ui.ProfilesSummary(p, outcomes, len(profiles), allDry)
	return exitWith(archive.ExitCode(result))
}

func newSchedule(g *globals) *cobra.Command {
	var immediate bool
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: ui.ScheduleShort,
		Long:  ui.ScheduleLong,
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := loadSettings(g, nil)
			if err != nil {
				return err
			}
			if s.Interval == 0 && s.At == "" {
				return ui.NoSchedule(g.profile)
			}
			if _, err := scheduler.Zone(); err != nil {
				return err
			}
			c, err := scheduler.Open(s, scheduler.ModeSchedule, Version)
			if err != nil {
				return err
			}
			defer c.Close()
			defer stopOnSignal(c)()
			p := printer(cmd, g)
			v := &runView{p: p, everyFile: listEveryFile(cmd, g, s.Verbose)}
			v.header("schedule", s)
			v.attach(c)
			c.AfterRun = func(sum archive.RunSummary) { v.report(sum, s.SettleTime); p.Blank() }
			c.Waiting = func(line string) { p.Plain(line) }
			p.Plain(fmt.Sprintf("Scheduler started for profile %s (%s). Stop with Ctrl-C or: synctoceph stop", s.Profile, c.ScheduleText()))
			return c.Loop(immediate)
		},
	}
	cmd.Flags().BoolVar(&immediate, "immediate", false, "also run once right away (does not use up a daily run)")
	return cmd
}
