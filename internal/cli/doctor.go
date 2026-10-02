// This file defines `synctoceph doctor`, which checks the whole setup and
// prints a fix for every problem. It changes nothing.
package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/engine"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/platform"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/scheduler"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/service"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/state"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// Check statuses in doctor's output.
const (
	checkOK    = "ok"
	checkNote  = "note"
	checkError = "error"
)

// check is one line of doctor's report.
type check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// doctor collects checks.
type doctor struct{ checks []check }

func (d *doctor) add(name, status, message string) {
	d.checks = append(d.checks, check{Name: name, Status: status, Message: message})
}

func (d *doctor) problem(name string, err error) { d.add(name, checkError, ui.Explain(err)) }

// problems counts the checks that failed.
func (d *doctor) problems() int {
	n := 0
	for _, c := range d.checks {
		if c.Status == checkError {
			n++
		}
	}
	return n
}

// print shows every check after its marker.
func (d *doctor) print(p *ui.Printer) {
	marks := map[string]string{checkOK: ui.MarkOK, checkNote: ui.MarkNote, checkError: ui.MarkError}
	for _, c := range d.checks {
		p.Line(marks[c.Status], c.Message)
	}
}

// diagnose runs every check for one profile.
func diagnose(profile string) *doctor {
	d := &doctor{}
	s, cfgErr := loadProfile(profile, nil)
	if cfgErr != nil {
		d.problem("config", cfgErr)
	} else {
		file, _ := config.ConfigFile(profile)
		d.add("config", checkOK, ui.DoctorConfigOK(file, s.Target()))
	}
	d.checkRsync()
	if cfgErr == nil {
		d.checkMount(s)
		d.checkFolders(s)
		if others := sharingProfiles(s); len(others) > 0 {
			d.add("profiles", checkNote, ui.SharedSubfolder(s.Target(), append([]string{profile}, others...)))
		}
	}
	d.checkState(profile)
	d.checkSchedule(s, cfgErr == nil)
	d.checkService(profile)
	return d
}

func newDoctor(g *globals) *cobra.Command {
	var asJSON, everyProfile bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: ui.DoctorShort,
		Long:  ui.DoctorLong,
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if everyProfile {
				return doctorAllProfiles(cmd, g, asJSON)
			}
			d := diagnose(g.profile)
			if asJSON {
				writeJSON(cmd.OutOrStdout(), map[string]any{"schema_version": JSONSchemaVersion,
					"profile": g.profile, "problems": d.problems(), "checks": d.checks})
			} else {
				p := printer(cmd, g)
				d.print(p)
				p.Line(ui.MarkResult, ui.DoctorResult(d.problems()))
			}
			return exitWith(min(d.problems(), 1))
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	cmd.Flags().BoolVar(&everyProfile, "all-profiles", false, "check every profile")
	return cmd
}

func (d *doctor) checkRsync() {
	path, version, ok, err := engine.RsyncVersion()
	switch {
	case err != nil:
		d.problem("rsync", err)
	case !ok:
		d.problem("rsync", ui.RsyncTooOld(path, version))
	default:
		d.add("rsync", checkOK, ui.DoctorRsyncOK(version, path))
	}
}

// checkMount checks require_mount without touching an unmounted share.
func (d *doctor) checkMount(s config.Settings) {
	mounts, err := platform.ReadMounts()
	if err != nil {
		d.problem("mount", ui.MountTableUnreadable(err))
		return
	}
	if s.RequireMount == "" {
		d.add("mount", checkNote, ui.DoctorNoMountCheck)
	} else if !platform.IsMountPoint(mounts, s.RequireMount) {
		d.add("mount", checkError, ui.Explain(ui.NotMounted(s.RequireMount, s.Destination))+"\n"+
			ui.MountCommands(s.RequireMount))
		return
	} else if err := engine.CheckMount(s); err != nil {
		d.problem("mount", err)
		return
	}
	target := s.RequireMount
	if target == "" {
		target = s.Destination
		if real, err := filepath.EvalSymlinks(target); err == nil {
			target = real
		}
	}
	if m, ok := platform.MountFor(mounts, target); ok {
		d.add("filesystem", checkOK, ui.DoctorFilesystem(s.Destination, m.Point, m.FSType, m.Source))
		if platform.IsWindowsDrive(m) {
			d.add("filesystem", checkNote, ui.DoctorWindowsDrive)
		}
	}
}

// checkFolders checks the source and destination folders and their separation.
func (d *doctor) checkFolders(s config.Settings) {
	if info, err := os.Stat(s.Source); err != nil || !info.IsDir() {
		d.problem("source", ui.SourceMissing(s.Source, err))
	} else if unix.Access(s.Source, unix.R_OK|unix.X_OK) != nil {
		d.problem("source", ui.SourceUnreadable(s.Source, os.ErrPermission))
	} else {
		d.add("source", checkOK, ui.DoctorSourceOK(s.Source, ui.AnimalsFound(animalFolders(s.Source))))
	}
	if err := destination.NoSymlinks(s.MetaDir); err != nil {
		d.problem("destination", ui.DestinationSymlink(s.Destination, err))
	} else if info, err := os.Stat(s.Destination); err != nil || !info.IsDir() {
		d.problem("destination", ui.DestinationMissing(s.Destination, err))
	} else if unix.Access(s.Destination, unix.W_OK|unix.X_OK) != nil {
		d.problem("destination", ui.DestinationNotWritable(s.Destination))
	} else {
		d.add("destination", checkOK, ui.DoctorDestinationOK(s.Destination))
	}
	if err := engine.CheckSeparate(s); err != nil {
		d.problem("paths", err)
	}
}

// checkState checks the state folder without creating it.
func (d *doctor) checkState(profile string) {
	dir, err := config.StateDir(profile)
	if err != nil {
		d.problem("state", err)
		return
	}
	if err := state.CheckLocal(dir); err != nil {
		d.problem("state", ui.StateProblem(dir, err))
		return
	}
	if state.Exists(dir) {
		if info, err := os.Stat(dir); err == nil && info.Mode().Perm()&0o022 != 0 {
			d.problem("state", ui.StateUnusable(dir, os.ErrPermission))
			return
		}
	}
	d.add("state", checkOK, ui.DoctorStateOK(dir))
	if state.IsHeld(filepath.Join(dir, state.LockName)) {
		d.add("state", checkNote, ui.DoctorRunning(profile))
	}
}

// checkSchedule checks the time zone and describes the schedule.
func (d *doctor) checkSchedule(s config.Settings, haveConfig bool) {
	if _, err := scheduler.Zone(); err != nil {
		d.problem("time zone", err)
		return
	}
	d.add("time zone", checkOK, ui.DoctorTimeZone(scheduler.ZoneName()))
	if !haveConfig {
		return
	}
	switch {
	case s.Interval > 0:
		d.add("schedule", checkOK, ui.DoctorSchedule("every "+config.FormatDuration(s.Interval)))
	case s.At != "":
		d.add("schedule", checkOK, ui.DoctorSchedule("daily at "+s.At))
	default:
		d.add("schedule", checkNote, ui.DoctorNoSchedule)
	}
}

// checkService reports how automatic runs are (or could be) set up.
func (d *doctor) checkService(profile string) {
	manager := service.Detect()
	unit, _, active := service.SystemdStatus(profile)
	task, _ := service.TaskStatus(profile)
	switch {
	case unit:
		d.add("service", checkOK, ui.DoctorServiceSystemd(service.UnitName(profile), active))
	case task:
		d.add("service", checkOK, ui.DoctorServiceTask(service.TaskName(profile)))
	default:
		d.add("service", checkNote, ui.DoctorNoService(manager, platform.IsWSL()))
	}
}
