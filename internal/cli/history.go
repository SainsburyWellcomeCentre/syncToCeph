// This file defines `synctoceph history list` and `history restore`, and
// `synctoceph fleet`.
package cli

import (
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

func newHistory(g *globals) *cobra.Command {
	cmd := &cobra.Command{Use: "history", Short: ui.HistoryShort, Args: noArgs}
	var asJSON bool
	list := &cobra.Command{
		Use:   "list [RUN_ID]",
		Short: ui.HistoryListShort,
		Args:  argsRange(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := loadSettings(g, nil)
			if err != nil {
				return err
			}
			p := printer(cmd, g)
			if len(args) == 1 {
				files, err := destination.ListHistoryFiles(s.MetaDir, args[0])
				if err != nil {
					return ui.HistoryUnavailable(err)
				}
				if asJSON {
					return writeJSON(cmd.OutOrStdout(), map[string]any{"schema_version": JSONSchemaVersion,
						"run_id": args[0], "files": files})
				}
				ui.HistoryFiles(p, args[0], files)
				return nil
			}
			runs, err := destination.ListHistory(s.MetaDir)
			if err != nil {
				return ui.HistoryUnavailable(err)
			}
			if asJSON {
				if runs == nil {
					runs = []destination.HistoryRun{}
				}
				return writeJSON(cmd.OutOrStdout(), map[string]any{"schema_version": JSONSchemaVersion, "runs": runs})
			}
			ui.HistoryRuns(p, destination.HistoryRoot(s.MetaDir), runs)
			return nil
		},
	}
	list.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	var to string
	restore := &cobra.Command{
		Use:   "restore RUN_ID PATH --to DIR",
		Short: ui.HistoryRestShort,
		Long:  ui.HistoryRestLong,
		Args:  argsRange(2, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" {
				return usageError{errMissingTo}
			}
			s, err := loadSettings(g, nil)
			if err != nil {
				return err
			}
			dir, err := filepath.Abs(to)
			if err != nil {
				return err
			}
			restored, err := destination.Restore(s.Destination, s.MetaDir, args[0], args[1], dir)
			p := printer(cmd, g)
			for _, path := range restored {
				p.Plain("  " + path)
			}
			if err != nil {
				return ui.RestoreFailed(err)
			}
			p.Line(ui.MarkOK, ui.Restored(len(restored), dir))
			return nil
		},
	}
	restore.Flags().StringVar(&to, "to", "", "restore into folder `DIR` (must be outside the destination)")
	cmd.AddCommand(list, restore)
	return cmd
}

func newFleet(g *globals) *cobra.Command {
	var root string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "fleet",
		Short: ui.FleetShort,
		Long:  ui.FleetLong,
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if root == "" {
				file, err := config.ConfigFile(g.profile)
				if err != nil {
					return err
				}
				cfg, err := config.Load(file)
				if err != nil {
					return err
				}
				root = cfg.Destination
			}
			reports, err := destination.Fleet(root)
			if err != nil {
				return ui.FleetUnavailable(root, err)
			}
			if asJSON {
				if reports == nil {
					reports = []destination.SubfolderReport{}
				}
				return writeJSON(cmd.OutOrStdout(), map[string]any{"schema_version": JSONSchemaVersion,
					"destination": root, "subfolders": reports})
			}
			ui.FleetReport(printer(cmd, g), root, reports, time.Now())
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "destination", "", "read the destination `DIR` instead of the one in the config")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return cmd
}
