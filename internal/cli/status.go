// This file defines `synctoceph status`: what is happening now, the last
// result, and the files that are deferred or differ from the archive. It
// only reads; it never creates the state folder.
package cli

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/state"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// statusJSON is the --json output of `status`.
type statusJSON struct {
	SchemaVersion int          `json:"schema_version"`
	Profile       string       `json:"profile"`
	Running       bool         `json:"running"`
	State         string       `json:"state"`
	Live          *state.Live  `json:"live,omitempty"`
	Status        state.Status `json:"status"`
}

// currentStatus reads status.json and asks a running process what it is
// doing. The lock, not status.json, decides whether anything is running.
func currentStatus(profile string) (statusJSON, error) {
	out := statusJSON{SchemaVersion: JSONSchemaVersion, Profile: profile}
	dir, err := config.StateDir(profile)
	if err != nil {
		return out, err
	}
	st, err := state.ReadStatus(dir, profile)
	if err != nil {
		return out, err
	}
	out.Status = st
	out.Running = state.IsHeld(filepath.Join(dir, state.LockName))
	switch {
	case out.Running:
		out.State = state.StateRunning
		if resp, err := state.SendControl(dir, state.Request{Command: "status"}); err == nil && resp.Live != nil {
			out.Live = resp.Live
			if resp.Live.Phase == "waiting" {
				out.State = state.StateWaiting
			}
		} else if st.State == state.StateWaiting {
			out.State = state.StateWaiting
		}
	case st.State == state.StateRunning || st.State == state.StateWaiting:
		out.State = state.StateStopped // the process ended without updating the file
	case st.State == "":
		out.State = state.StateIdle
	default:
		out.State = st.State
	}
	return out, nil
}

// statusAllJSON is the --json output of `status --all-profiles`.
type statusAllJSON struct {
	SchemaVersion int          `json:"schema_version"`
	Profiles      []statusJSON `json:"profiles"`
}

// statusAllProfiles prints one line per profile (or JSON for all of them).
func statusAllProfiles(cmd *cobra.Command, g *globals, asJSON bool) error {
	profiles, err := allProfiles(cmd)
	if err != nil {
		return err
	}
	all := statusAllJSON{SchemaVersion: JSONSchemaVersion}
	var rows []ui.ProfileRow
	for _, profile := range profiles {
		cur, err := currentStatus(profile)
		if err != nil {
			return err
		}
		all.Profiles = append(all.Profiles, cur)
		row := ui.ProfileRow{Profile: profile, State: cur.State, LastRun: cur.Status.LastRun}
		if s, err := loadProfile(profile, nil); err != nil {
			first, _, _ := strings.Cut(err.Error(), "\n")
			row.Problem = first
		} else {
			row.Source, row.MachineDir = s.Source, s.MachineDir
		}
		rows = append(rows, row)
	}
	if asJSON {
		return writeJSON(cmd.OutOrStdout(), all)
	}
	ui.ProfilesTable(printer(cmd, g), rows, time.Now())
	return nil
}

func newStatus(g *globals) *cobra.Command {
	var differing, deferred, asJSON, everyProfile bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: ui.StatusShort,
		Long:  ui.StatusLong,
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if everyProfile {
				if differing || deferred {
					return usageError{errors.New("--all-profiles cannot be used with --differing or --deferred")}
				}
				return statusAllProfiles(cmd, g, asJSON)
			}
			cur, err := currentStatus(g.profile)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), cur)
			}
			p := printer(cmd, g)
			last := cur.Status.LastRun
			switch {
			case differing:
				if last == nil || len(last.Differing) == 0 {
					p.Plain(ui.NoDifferingFiles)
					return nil
				}
				p.Plain(ui.DifferingHeader(len(last.Differing)))
				for _, path := range last.Differing {
					p.Plain("  " + path)
				}
				return nil
			case deferred:
				if last == nil || len(last.Deferred) == 0 {
					p.Plain(ui.NoDeferredFiles)
					return nil
				}
				now := time.Now()
				for _, d := range last.Deferred {
					p.Plain(ui.DeferredEntry(d, now))
				}
				return nil
			}
			ui.StatusReport(p, cur.Profile, cur.State, cur.Live, cur.Status, time.Now())
			return nil
		},
	}
	cmd.Flags().BoolVar(&differing, "differing", false, "list files in the archive that differ from the source and were not replaced")
	cmd.Flags().BoolVar(&deferred, "deferred", false, "list files left for a later run")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	cmd.Flags().BoolVar(&everyProfile, "all-profiles", false, "show one line for every profile")
	return cmd
}
