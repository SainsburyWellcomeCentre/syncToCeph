// This file defines `synctoceph version` and `synctoceph completion`.
package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

func newVersion() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: ui.VersionShort,
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"schema_version": JSONSchemaVersion,
					"version": Version, "commit": Commit, "build_date": BuildDate, "go": runtime.Version()})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "synctoceph %s (commit %s, built %s, %s)\n", Version, Commit, BuildDate, runtime.Version())
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return cmd
}

func newCompletion() *cobra.Command {
	return &cobra.Command{
		Use:       "completion bash|zsh",
		Short:     ui.CompletionShort,
		Long:      ui.CompletionLong,
		ValidArgs: []string{"bash", "zsh"},
		Args:      argsRange(1, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(cmd.OutOrStdout(), true)
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			}
			return usageError{fmt.Errorf("unsupported shell %q: use bash or zsh", args[0])}
		},
	}
}
