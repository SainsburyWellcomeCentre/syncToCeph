// This file has small helpers shared by the commands: loading the settings,
// making a printer, writing JSON, and catching Ctrl-C.
package cli

import (
	"encoding/json"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/engine"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/scheduler"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// JSONSchemaVersion is the version of every --json output format. Change it
// (and the docs) when the meaning of a field changes.
const JSONSchemaVersion = 1

// loadSettings reads the profile's config file, lets adjust apply
// command-line flags (which take precedence), and checks the result.
func loadSettings(g *globals, adjust func(*config.Config)) (config.Settings, error) {
	file, err := config.ConfigFile(g.profile)
	if err != nil {
		return config.Settings{}, err
	}
	cfg, err := config.Load(file)
	if err != nil {
		return config.Settings{}, err
	}
	if adjust != nil {
		adjust(&cfg)
	}
	return cfg.Resolve(g.profile)
}

// printer returns a printer for the command's standard output.
func printer(cmd *cobra.Command, g *globals) *ui.Printer {
	return ui.NewPrinter(cmd.OutOrStdout(), g.noColor, g.quiet)
}

// writeJSON prints v as indented JSON.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// stopOnSignal asks the controller to stop gracefully on Ctrl-C (SIGINT) or
// SIGTERM. The returned function stops listening.
func stopOnSignal(c *scheduler.Controller) func() {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case sig := <-signals:
			c.Log.Info("Received %s", sig)
			// SAFETY: invariant 11 (stopping is graceful): SIGINT to rsync, SIGKILL after 30 s.
			c.RequestStop(engine.KillGrace)
		case <-done:
		}
	}()
	return func() {
		signal.Stop(signals)
		close(done)
	}
}

// exitWith returns an error that ends the program with code, or nil for 0.
func exitWith(code int) error {
	if code == 0 {
		return nil
	}
	return exitError{code}
}
