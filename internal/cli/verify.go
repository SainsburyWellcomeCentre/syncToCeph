// This file defines `synctoceph verify` (re-hash copied files) and
// `synctoceph check-copied` (is it safe to delete these source files?).
package cli

import (
	"context"
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/engine"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/scheduler"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

var errNonPositiveTimeout = errors.New("--timeout must be a positive number of seconds")

func newVerify(g *globals) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "verify [PATH]",
		Short: ui.VerifyShort,
		Long:  ui.VerifyLong,
		Args:  argsRange(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := loadSettings(g, nil)
			if err != nil {
				return err
			}
			sub := ""
			if len(args) == 1 {
				if sub, err = engine.SourceRelative(s, args[0]); err != nil {
					return err
				}
			}
			c, err := scheduler.Open(s, scheduler.ModeVerify, Version)
			if err != nil {
				return err
			}
			defer c.Close()
			defer stopOnSignal(c)()
			p := printer(cmd, g)
			if !asJSON {
				p.Plain(ui.VerifyStarting)
			}
			rep, err := c.Verify(sub, nil)
			interrupted := errors.Is(err, context.Canceled)
			if err != nil && !interrupted {
				return err
			}
			if asJSON {
				writeJSON(cmd.OutOrStdout(), map[string]any{"schema_version": JSONSchemaVersion,
					"profile": g.profile, "interrupted": interrupted, "report": rep})
			} else {
				ui.VerifyReport(p, rep, interrupted, g.verbose)
			}
			switch {
			case interrupted:
				return exitWith(130)
			case len(rep.Differing)+len(rep.NotCopied)+len(rep.Changed)+len(rep.Errors) > 0:
				return exitWith(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return cmd
}

func newCheckCopied(g *globals) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "check-copied PATH",
		Short: ui.CheckCopiedShort,
		Long:  ui.CheckCopiedLong,
		Args:  argsRange(1, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := loadSettings(g, nil)
			if err != nil {
				return err
			}
			sub, err := engine.SourceRelative(s, args[0])
			if err != nil {
				return err
			}
			if err := engine.CheckMount(s); err != nil {
				return err
			}
			if info, err := os.Stat(s.Destination); err != nil || !info.IsDir() {
				return ui.DestinationMissing(s.Destination, err)
			}
			files, err := engine.CheckCopied(context.Background(), s, sub)
			if err != nil {
				return err
			}
			safe := len(files) > 0
			for _, f := range files {
				safe = safe && f.Status == destination.StatusVerified
			}
			if asJSON {
				if files == nil {
					files = []destination.FileStatus{}
				}
				writeJSON(cmd.OutOrStdout(), map[string]any{"schema_version": JSONSchemaVersion,
					"profile": g.profile, "path": args[0], "all_verified": safe, "files": files})
			} else {
				ui.CheckCopiedReport(printer(cmd, g), args[0], files, safe, g.verbose)
			}
			if !safe {
				return exitWith(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return cmd
}
