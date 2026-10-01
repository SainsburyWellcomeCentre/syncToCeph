// This file defines `synctoceph service install|uninstall|status`, which set
// up automatic runs with systemd or Windows Task Scheduler.
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/service"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

var errMissingTo = errors.New("history restore needs --to DIR")

func newService(g *globals) *cobra.Command {
	cmd := &cobra.Command{Use: "service", Short: ui.ServiceShort, Args: noArgs}
	cmd.AddCommand(newServiceInstall(g), newServiceUninstall(g), newServiceStatus(g))
	return cmd
}

// executable returns the absolute path of the running synctoceph binary.
func executable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding the synctoceph program: %w", err)
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	return path, nil
}

func newServiceInstall(g *globals) *cobra.Command {
	var manager string
	var printOnly bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: ui.ServiceInstallShort,
		Long:  ui.ServiceInstallLong,
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := loadSettings(g, nil)
			if err != nil {
				return err
			}
			if s.Interval == 0 && s.At == "" {
				return ui.NoSchedule(g.profile)
			}
			binary, err := executable()
			if err != nil {
				return err
			}
			if manager == service.ManagerAuto {
				manager = service.Detect()
			}
			p := printer(cmd, g)
			switch manager {
			case service.ManagerSystemd:
				return installSystemd(p, binary, s, printOnly)
			case service.ManagerTask:
				return installTask(p, binary, s, printOnly)
			case service.ManagerNone:
				return ui.NoServiceManager()
			}
			return usageError{fmt.Errorf("--manager must be auto, systemd or task-scheduler, not %q", manager)}
		},
	}
	cmd.Flags().StringVar(&manager, "manager", service.ManagerAuto, "auto, systemd or task-scheduler")
	cmd.Flags().BoolVar(&printOnly, "print", false, "only show what would be set up")
	return cmd
}

// installSystemd sets up (or prints) the systemd user service.
func installSystemd(p *ui.Printer, binary string, s config.Settings, printOnly bool) error {
	if printOnly {
		dir, err := service.UnitDir()
		if err != nil {
			return err
		}
		p.Plain(ui.WouldWriteUnit(filepath.Join(dir, service.UnitName(s.Profile))))
		p.Plain(service.UnitContent(binary, s.Profile))
		p.Plain(ui.SystemdEnableCommands(service.UnitName(s.Profile)))
		return nil
	}
	notes, err := service.InstallSystemd(binary, s.Profile)
	if err != nil {
		return ui.ServiceFailed(err)
	}
	for _, note := range notes {
		if note == "linger-off" {
			p.Line(ui.MarkNote, ui.LingerOff())
		} else {
			p.Plain(note)
		}
	}
	p.Line(ui.MarkOK, ui.SystemdInstalled(service.UnitName(s.Profile), s.Profile))
	return nil
}

// installTask sets up (or prints) the Windows Task Scheduler task.
func installTask(p *ui.Printer, binary string, s config.Settings, printOnly bool) error {
	args, err := service.TaskArgs(binary, s.Profile, service.Distro(), s)
	if err != nil {
		return ui.ServiceFailed(err)
	}
	if printOnly || !service.HasTaskScheduler() {
		p.Plain(ui.TaskCommandIntro)
		p.Plain("  " + service.PrintableTaskCommand(args))
		return nil
	}
	if err := service.InstallTask(args); err != nil {
		return ui.ServiceFailed(err)
	}
	p.Line(ui.MarkOK, ui.TaskInstalled(service.TaskName(s.Profile), s.Profile))
	return nil
}

func newServiceUninstall(g *globals) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: ui.ServiceUninstallShort,
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles := []string{g.profile}
			if all {
				var err error
				if profiles, err = serviceProfiles(); err != nil {
					return err
				}
			}
			p := printer(cmd, g)
			removed := 0
			for _, profile := range profiles {
				if ok, err := service.UninstallSystemd(profile); err != nil {
					return ui.ServiceFailed(err)
				} else if ok {
					removed++
					p.Line(ui.MarkOK, ui.ServiceRemoved(service.UnitName(profile)))
				}
				if ok, err := service.UninstallTask(profile); err != nil {
					return ui.ServiceFailed(err)
				} else if ok {
					removed++
					p.Line(ui.MarkOK, ui.ServiceRemoved(service.TaskName(profile)))
				}
			}
			if removed == 0 {
				p.Line(ui.MarkOK, ui.NoServiceFound)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "remove automatic runs of every profile")
	return cmd
}

// serviceProfiles lists profiles that have a config file or a systemd unit.
func serviceProfiles() ([]string, error) {
	names, err := config.Profiles()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, n := range names {
		seen[n] = true
	}
	if dir, err := service.UnitDir(); err == nil {
		units, _ := filepath.Glob(filepath.Join(dir, "synctoceph-*.service"))
		for _, u := range units {
			name := filepath.Base(u)
			name = name[len("synctoceph-") : len(name)-len(".service")]
			if !seen[name] && config.ValidateProfile(name) == nil {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names, nil
}

func newServiceStatus(g *globals) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: ui.ServiceStatusShort,
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			unit, enabled, active := service.SystemdStatus(g.profile)
			task, detail := service.TaskStatus(g.profile)
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"schema_version": JSONSchemaVersion,
					"profile": g.profile, "detected_manager": service.Detect(),
					"systemd":        map[string]any{"installed": unit, "enabled": enabled, "active": active},
					"task_scheduler": map[string]any{"installed": task, "detail": detail}})
			}
			ui.ServiceStatusReport(printer(cmd, g), g.profile, service.Detect(),
				unit, enabled, active, task, detail)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return cmd
}
