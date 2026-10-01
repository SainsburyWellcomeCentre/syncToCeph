// This file defines `synctoceph run` (one sync) and `synctoceph schedule`
// (syncs on the configured schedule, used by the service).
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/archive"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/scheduler"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// runFlags are the flags of `run` that override the config file.
type runFlags struct {
	dryRun   bool
	existing string
	verify   string
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
			s, err := loadSettings(g, f.apply(cmd))
			if err != nil {
				return err
			}
			c, err := scheduler.Open(s, scheduler.ModeRun, Version)
			if err != nil {
				return err
			}
			defer c.Close()
			defer stopOnSignal(c)()
			p := printer(cmd, g)
			attachOutput(c, p, g)
			sum := c.RunOnce()
			ui.RunReport(p, sum, s.SettleTime, g.verbose)
			return exitWith(sum.ExitCode)
		},
	}
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false, "show what would be copied; write nothing to the archive")
	cmd.Flags().StringVar(&f.existing, "existing", "", "files already in the archive that differ: skip or replace (default from config: skip)")
	cmd.Flags().StringVar(&f.verify, "verify", "", "what to check with SHA-256: new (files copied now) or all (default from config: new)")
	return cmd
}

// attachOutput shows each phase (and rsync lines with -v) in the terminal.
func attachOutput(c *scheduler.Controller, p *ui.Printer, g *globals) {
	last := ""
	c.Report = func(phase, detail string) {
		if phase == last {
			return
		}
		last = phase
		if line := ui.PhaseLine(phase, detail); line != "" {
			p.Plain(line)
		}
	}
	if g.verbose {
		c.Verbose = func(line string) { p.Plain(ui.Indent + line) }
	}
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
			attachOutput(c, p, g)
			c.AfterRun = func(sum archive.RunSummary) { ui.RunReport(p, sum, s.SettleTime, g.verbose) }
			c.Waiting = func(line string) { p.Plain(line) }
			p.Plain(fmt.Sprintf("Scheduler started for profile %s (%s). Stop with Ctrl-C or: synctoceph stop", s.Profile, c.ScheduleText()))
			return c.Loop(immediate)
		},
	}
	cmd.Flags().BoolVar(&immediate, "immediate", false, "also run once right away (does not use up a daily run)")
	return cmd
}
