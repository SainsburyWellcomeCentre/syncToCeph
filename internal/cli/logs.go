// This file defines `synctoceph logs` and `synctoceph stop`.
package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/engine"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/state"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

func newLogs(g *globals) *cobra.Command {
	var lines int
	var runID string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "logs",
		Short: ui.LogsShort,
		Long:  ui.LogsLong,
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := config.StateDir(g.profile)
			if err != nil {
				return err
			}
			found, err := state.Tail(dir, lines, runID)
			if err != nil {
				return err
			}
			if asJSON {
				if found == nil {
					found = []string{}
				}
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"schema_version": JSONSchemaVersion, "profile": g.profile, "lines": found})
			}
			if len(found) == 0 {
				printer(cmd, g).Plain(ui.NoLogLines(runID))
				return nil
			}
			for _, line := range found {
				fmt.Fprintln(cmd.OutOrStdout(), line)
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&lines, "lines", "n", 50, "number of lines to show (0 = all)")
	cmd.Flags().StringVar(&runID, "run", "", "show only the lines of this run ID")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return cmd
}

// stopWaitExtra is how long `stop` waits beyond the rsync grace period for
// the process to write its status and exit.
const stopWaitExtra = 15 * time.Second

// stopPoll is how often `stop` checks whether the process has exited.
const stopPoll = 200 * time.Millisecond

func newStop(g *globals) *cobra.Command {
	var timeout int
	cmd := &cobra.Command{
		Use:   "stop",
		Short: ui.StopShort,
		Long:  ui.StopLong,
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if timeout <= 0 {
				return usageError{errNonPositiveTimeout}
			}
			dir, err := config.StateDir(g.profile)
			if err != nil {
				return err
			}
			p := printer(cmd, g)
			lock := filepath.Join(dir, state.LockName)
			if !state.IsHeld(lock) {
				p.Line(ui.MarkOK, ui.NothingRunning(g.profile))
				return nil
			}
			resp, err := state.SendControl(dir, state.Request{Command: "stop", GraceSeconds: timeout})
			if err != nil || !resp.OK {
				st, _ := state.ReadStatus(dir, g.profile)
				return ui.StopUnreachable(g.profile, st.PID)
			}
			p.Plain(ui.StopRequested(g.profile))
			deadline := time.Now().Add(time.Duration(timeout)*time.Second + stopWaitExtra)
			for state.IsHeld(lock) {
				if time.Now().After(deadline) {
					return ui.StopTimedOut(g.profile)
				}
				time.Sleep(stopPoll)
			}
			p.Line(ui.MarkOK, ui.Stopped(g.profile))
			return nil
		},
	}
	cmd.Flags().IntVar(&timeout, "timeout", int(engine.KillGrace/time.Second), "seconds rsync gets to stop before it is killed")
	return cmd
}
