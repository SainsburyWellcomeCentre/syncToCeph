// This file defines `synctoceph init`, which asks for the settings, checks
// each answer, and writes the profile's config file.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/engine"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/platform"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// maxAttempts is how often init asks again after an invalid answer.
const maxAttempts = 3

// initFlags are the answers that can be given on the command line.
type initFlags struct {
	source, dest, subfolder, mount, schedule string
	yes, force                               bool
}

// asker asks one question at a time, or takes the answer from a flag.
type asker struct {
	in  *bufio.Reader
	out io.Writer
	yes bool
}

// ask returns the flag value if given; otherwise prompts (showing the
// default) until check accepts the answer.
func (a *asker) ask(question, flagValue, def string, check func(string) (string, error)) (string, error) {
	if flagValue != "" {
		return check(flagValue)
	}
	if a.yes {
		return check(def)
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if def != "" {
			fmt.Fprintf(a.out, "%s [%s]: ", question, def)
		} else {
			fmt.Fprintf(a.out, "%s: ", question)
		}
		line, err := a.in.ReadString('\n')
		answer := strings.TrimSpace(line)
		if answer == "" {
			answer = def
		}
		if err != nil && answer == "" {
			return "", ui.InitNoAnswer(question)
		}
		value, checkErr := check(answer)
		if checkErr == nil {
			return value, nil
		}
		fmt.Fprintln(a.out, ui.Indent+ui.Explain(checkErr))
		if err != nil {
			return "", checkErr
		}
	}
	return "", ui.InitGaveUp(question)
}

func newInit(g *globals) *cobra.Command {
	f := &initFlags{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: ui.InitShort,
		Long:  ui.InitLong,
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := config.ConfigFile(g.profile)
			if err != nil {
				return err
			}
			cfg := config.Defaults()
			cfg.Exclude = config.SuggestedExclude
			cfg.ExcludeHidden = true
			if _, err := os.Lstat(file); err == nil {
				if !f.force {
					return ui.ConfigExists(file)
				}
				if old, err := config.Load(file); err == nil {
					cfg = old
				}
			}
			a := &asker{in: bufio.NewReader(cmd.InOrStdin()), out: cmd.OutOrStdout(), yes: f.yes}
			if err := askAll(a, f, &cfg); err != nil {
				return err
			}
			s, err := cfg.Resolve(g.profile)
			if err != nil {
				return err
			}
			if err := engine.CheckMount(s); err != nil {
				return err
			}
			if err := engine.CheckSeparate(s); err != nil {
				return err
			}
			if err := cfg.Save(g.profile, file, f.force); err != nil {
				return ui.ConfigNotSaved(file, err)
			}
			p := printer(cmd, g)
			p.Line(ui.MarkOK, ui.ConfigSaved(file, s.Target(), ui.AnimalsFound(animalFolders(s.Source)), g.profile))
			if others := sharingProfiles(s); len(others) > 0 {
				p.Line(ui.MarkNote, ui.SharedSubfolder(s.Target(), append([]string{g.profile}, others...)))
			}
			return nil
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.source, "source", "", "folder `DIR` to copy from, holding one folder per animal")
	fl.StringVar(&f.dest, "destination", "", "folder `DIR` on ceph that holds the animal folders (must already exist)")
	fl.StringVar(&f.subfolder, "subfolder", "", "this machine's folder `NAME` inside every animal folder, e.g. behaviour or ephys")
	fl.StringVar(&f.mount, "require-mount", "", "only run when `DIR` is mounted; \"none\" for no check (default: the network mount holding the destination, if any)")
	fl.StringVar(&f.schedule, "schedule", "", "when to run: `WHEN` is an interval such as 4h, a daily time such as 02:00, or \"none\" (default: 4h)")
	fl.BoolVarP(&f.yes, "yes", "y", false, "do not ask; use flags and defaults")
	fl.BoolVar(&f.force, "force", false, "replace an existing configuration")
	return cmd
}

// askAll asks every question in turn.
func askAll(a *asker, f *initFlags, cfg *config.Config) error {
	var err error
	if cfg.Source, err = a.ask(ui.AskSource, f.source, cfg.Source, checkFolder("source")); err != nil {
		return err
	}
	if cfg.Destination, err = a.ask(ui.AskDestination, f.dest, cfg.Destination, checkDestination); err != nil {
		return err
	}
	if cfg.Subfolder, err = a.ask(ui.AskSubfolder, f.subfolder, cfg.Subfolder, checkSubfolder); err != nil {
		return err
	}
	mountDefault := cfg.RequireMount
	if mountDefault == "" {
		mountDefault = suggestMount(cfg.Destination)
	}
	if cfg.RequireMount, err = a.ask(ui.AskMount, f.mount, mountDefault, checkMountAnswer(cfg.Destination)); err != nil {
		return err
	}
	scheduleDefault := "4h"
	switch {
	case cfg.At != "":
		scheduleDefault = cfg.At
	case cfg.Interval != "":
		scheduleDefault = cfg.Interval
	}
	answer, err := a.ask(ui.AskSchedule, f.schedule, scheduleDefault, checkSchedule)
	if err != nil {
		return err
	}
	cfg.Interval, cfg.At = "", ""
	if _, _, atErr := config.ParseAt(answer); atErr == nil {
		cfg.At = answer
	} else if answer != "none" {
		cfg.Interval = answer
	}
	return nil
}

func checkSubfolder(v string) (string, error) {
	c := config.Config{Subfolder: v, Source: "/", Destination: "/", Existing: "skip", Verify: "new", SettleTime: "0"}
	if _, err := c.Resolve(config.DefaultProfile); err != nil {
		return "", err
	}
	return v, nil
}

// animalFolders lists the folders directly in the source (each is copied as
// one animal), leaving out synctoceph's reserved names.
func animalFolders(source string) []string {
	entries, err := os.ReadDir(source)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != destination.MetaName && e.Name() != destination.PartialName {
			names = append(names, e.Name())
		}
	}
	return names
}

// checkFolder accepts an existing folder and returns its absolute path.
func checkFolder(key string) func(string) (string, error) {
	return func(v string) (string, error) {
		if v == "" {
			return "", ui.MissingSetting(key)
		}
		abs, err := filepath.Abs(v)
		if err != nil {
			return "", err
		}
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			return "", ui.SourceMissing(abs, err)
		}
		return abs, nil
	}
}

// checkDestination accepts an existing destination folder; it is never created.
func checkDestination(v string) (string, error) {
	if v == "" {
		return "", ui.MissingSetting("destination")
	}
	abs, err := filepath.Abs(v)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return "", ui.DestinationMissing(abs, err)
	}
	return abs, nil
}

// suggestMount returns the mount point holding dest if it is a network
// share or Windows drive, or "none" for a local disk.
func suggestMount(dest string) string {
	mounts, err := platform.ReadMounts()
	if err != nil || dest == "" {
		return "none"
	}
	if real, err := filepath.EvalSymlinks(dest); err == nil {
		dest = real
	}
	if m, ok := platform.MountFor(mounts, dest); ok && m.Point != "/" && platform.IsNetwork(m) {
		return m.Point
	}
	return "none"
}

// checkMountAnswer accepts "none" or a mount point that contains dest.
func checkMountAnswer(dest string) func(string) (string, error) {
	return func(v string) (string, error) {
		if v == "none" || v == "" {
			return "", nil
		}
		s := config.Settings{Destination: dest, RequireMount: filepath.Clean(v)}
		if !filepath.IsAbs(v) {
			return "", ui.BadSetting("require_mount", v, ui.AbsolutePathRule)
		}
		return s.RequireMount, engine.CheckMount(s)
	}
}

func checkSchedule(v string) (string, error) {
	if v == "none" {
		return v, nil
	}
	if _, _, err := config.ParseAt(v); err == nil {
		return v, nil
	}
	if d, err := config.ParseDuration(v); err != nil || d <= 0 {
		return "", ui.BadSetting("schedule", v, ui.ScheduleRule)
	}
	return v, nil
}
