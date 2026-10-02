// Package cli defines the synctoceph commands: their flags, and what they
// print. The work itself happens in the other internal packages; each file
// here handles one command.
//
// This file builds the command tree and turns errors into exit codes.
package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/archive"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// Build information, set when building with
// -ldflags "-X github.com/SainsburyWellcomeCentre/syncToCeph/internal/cli.Version=...".
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// globals holds the flags that every command accepts.
type globals struct {
	profile string
	verbose bool
	quiet   bool
	noColor bool
}

// exitError ends the program with a specific exit code after its output has
// already been printed.
type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("exit code %d", e.code) }

// usageError marks a mistake in how the command was typed (exit code 2).
type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }

// Main runs synctoceph with the program's arguments and returns the exit code.
func Main() int {
	root := NewRoot()
	root.SetArgs(os.Args[1:])
	cmd, err := root.ExecuteC()
	if err == nil {
		return archive.ExitOK
	}
	var exit exitError
	if errors.As(err, &exit) {
		return exit.code
	}
	p := ui.NewPrinter(os.Stderr, false, false)
	var usage usageError
	if errors.As(err, &usage) || strings.HasPrefix(err.Error(), "unknown command") {
		p.Line(ui.MarkError, err.Error()+"\n"+ui.SeeHelp(cmd.CommandPath()))
		return archive.ExitUsage
	}
	p.Problem(err)
	return archive.ExitFailed
}

// NewRoot builds the full command tree. It is also used to generate the
// documentation in docs/cli/ and the shell completion scripts.
func NewRoot() *cobra.Command {
	g := &globals{}
	root := &cobra.Command{
		Use:           "synctoceph",
		Short:         ui.RootShort,
		Long:          ui.RootLong,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if g.verbose && g.quiet {
				return usageError{errors.New("--verbose and --quiet cannot be used together")}
			}
			if err := config.ValidateProfile(g.profile); err != nil {
				return usageError{err}
			}
			return nil
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	flags := root.PersistentFlags()
	flags.StringVar(&g.profile, "profile", config.DefaultProfile, "use the profile (job) `NAME`; its settings are in ~/.config/synctoceph/NAME.toml")
	flags.BoolVarP(&g.verbose, "verbose", "v", false, "list every file copied and verified; --verbose=false turns it off (default: the config's verbose setting, true unless changed)")
	flags.BoolVarP(&g.quiet, "quiet", "q", false, "print only the result and errors")
	flags.BoolVar(&g.noColor, "no-color", false, "never use colour (NO_COLOR is also honoured)")
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error { return usageError{err} })
	root.AddCommand(newInit(g), newDoctor(g), newRun(g), newSchedule(g), newStatus(g), newLogs(g),
		newStop(g), newVerify(g), newCheckArchived(g), newHistory(g), newFleet(g), newService(g),
		newCompletion(), newVersion())
	return root
}

// noArgs rejects positional arguments (as a usage error).
func noArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		if cmd.HasSubCommands() {
			return usageError{fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())}
		}
		return usageError{fmt.Errorf("%s takes no arguments", cmd.CommandPath())}
	}
	return nil
}

// argsRange accepts between min and max positional arguments.
func argsRange(min, max int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < min || len(args) > max {
			return usageError{fmt.Errorf("%s needs %s", cmd.CommandPath(), cmd.Use)}
		}
		return nil
	}
}
