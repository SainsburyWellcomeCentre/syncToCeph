// This file holds the wording of the setup commands: init, doctor and
// service. See messages.go for how the message files are organised.
package ui

import (
	"fmt"
	"strings"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/platform"
)

// Questions asked by `synctoceph init`.
const (
	AskSource      = "Source folder holding one folder per animal (e.g. /mnt/d/luminoseData)"
	AskDestination = "Destination folder on ceph that holds the animal folders (e.g. /mnt/ceph/project)"
	AskSubfolder   = "Subfolder for this machine's data inside each animal folder (e.g. behaviour, ephys, histology)"
	AskMount       = `Mount point that must be mounted ("none" for no check)`
	AskSchedule    = `Schedule: an interval (e.g. 4h), a daily time (e.g. 02:00), or "none"`

	ScheduleRule = `use an interval such as 30m or 4h, a daily time such as 02:00, or "none"`
)

// InitNoAnswer: init needs an answer but input ended.
func InitNoAnswer(question string) error {
	return &Problem{What: "no answer for: " + question,
		Fix: "run `synctoceph init` in a terminal, or give every answer as a flag (see `synctoceph init --help`)"}
}

// InitGaveUp: too many invalid answers.
func InitGaveUp(question string) error {
	return &Problem{What: "no valid answer for: " + question, Fix: "run `synctoceph init` again"}
}

// ConfigExists: init would overwrite a config.
func ConfigExists(file string) error {
	return &Problem{What: "a configuration already exists at " + file,
		Why: "init does not overwrite it by accident",
		Fix: "edit the file, or run `synctoceph init --force` to answer the questions again (current values are offered as defaults).\n" +
			"To copy another folder as a separate job, add a profile: synctoceph --profile NAME init"}
}

// ConfigNotSaved: the config file could not be written.
func ConfigNotSaved(file string, err error) error {
	return &Problem{What: "cannot save the configuration to " + file, Err: err,
		Fix: "check that you can write to " + file}
}

// ConfigSaved confirms init and lists the next steps. animals describes the
// animal folders found in the source (see AnimalsFound).
func ConfigSaved(file, target, animals, profile string) string {
	flag := ""
	if profile != "default" {
		flag = " --profile " + profile
	}
	return fmt.Sprintf("Saved %s\n"+
		"Each animal folder in the source will be copied to %s\n"+
		"%s\n"+
		"Next steps:\n"+
		"  synctoceph doctor%[4]s            check the setup\n"+
		"  synctoceph run --dry-run%[4]s     see what would be copied\n"+
		"  synctoceph run%[4]s               copy and verify\n"+
		"  synctoceph service install%[4]s   run automatically on the schedule", file, target, animals, flag)
}

// animalsShown is how many animal folder names AnimalsFound lists.
const animalsShown = 5

// AnimalsFound lists the folders found directly in the source, each of which
// is copied as one animal, so a wrongly chosen source is easy to spot.
func AnimalsFound(names []string) string {
	if len(names) == 0 {
		return "The source has no folders yet; each folder put directly in it is copied as one animal."
	}
	shown := names[:min(len(names), animalsShown)]
	text := fmt.Sprintf("Animal folders in the source (%s): %s", Count(len(names)), strings.Join(shown, ", "))
	if len(names) > animalsShown {
		text += fmt.Sprintf(" and %d more", len(names)-animalsShown)
	}
	return text
}

// NoSchedule: schedule or service install without a schedule.
func NoSchedule(profile string) error {
	return &Problem{What: "no schedule is set for profile " + profile,
		Fix: "add interval = \"4h\" (or at = \"02:00\") to the config file, or run `synctoceph init --force`"}
}

// Doctor lines.
const (
	DoctorNoMountCheck = "No mount check is configured (require_mount is empty).\n" +
		"If the destination is on a network share such as ceph, set require_mount to its mount\n" +
		"point so synctoceph never writes to an empty folder when the share is not mounted."
	DoctorWindowsDrive = "The destination is on a Windows drive (drvfs). See docs/troubleshooting.md for what\n" +
		"was checked on such drives and the known limitations."
	DoctorNoSchedule = "No schedule is set, so the service cannot be installed. Add interval or at to the config."
)

// DoctorConfigOK describes a valid config.
func DoctorConfigOK(file, target string) string {
	return "Configuration " + file + " is valid; data goes to " + target
}

// DoctorRsyncOK describes a suitable rsync.
func DoctorRsyncOK(version, path string) string { return "rsync " + version + " (" + path + ")" }

// MountCommands explains how to mount the share.
func MountCommands(mount string) string {
	if letter := platform.DriveLetter(mount); letter != "" {
		return fmt.Sprintf("If the share is a Windows mapped drive %[1]s:, make it visible in WSL with:\n"+
			"  sudo mkdir -p %[2]s && sudo mount -t drvfs %[1]s: %[2]s\n"+
			"To mount it at every start, add this line to /etc/fstab:\n"+
			"  %[1]s: %[2]s drvfs defaults 0 0\n"+
			"See docs/mounting.md.", letter, mount)
	}
	return fmt.Sprintf("Mount the ceph share at %[1]s. Once, install the SMB tools and create the folder:\n"+
		"  sudo apt install cifs-utils && sudo mkdir -p %[1]s\n"+
		"Then mount it by hand (asks for your password), with SHARE and USER filled in:\n"+
		"  sudo mount -t cifs //ceph-gw02.hpc.swc.ucl.ac.uk/SHARE %[1]s -o username=USER,domain=ad.swc.ucl.ac.uk,uid=$(id -u),gid=$(id -g),vers=3.0\n"+
		"Or, to mount it at every start, add a line like this to /etc/fstab (with a credentials file):\n"+
		"  //ceph-gw02.hpc.swc.ucl.ac.uk/SHARE %[1]s cifs credentials=/home/USER/.swc_credentials,uid=YOUR_UID,gid=YOUR_GID,_netdev,vers=3.0,nofail,x-systemd.automount 0 0\n"+
		"docs/mounting.md explains every option and the credentials file.", mount)
}

// DoctorFilesystem describes the filesystem holding the destination.
func DoctorFilesystem(dest, point, fstype, source string) string {
	return fmt.Sprintf("%s is on %s (type %s, from %s)", dest, point, fstype, source)
}

// DoctorSourceOK describes a readable source and the animal folders in it.
func DoctorSourceOK(source, animals string) string {
	return "Source " + source + " exists and is readable\n" + animals
}

// DestinationNotWritable: the destination folder is read-only for this user.
func DestinationNotWritable(dir string) error {
	return &Problem{What: "you cannot write to " + dir,
		Why: "copies would fail",
		Fix: "check the share's permissions and mount options (uid and gid for cifs, see docs/mounting.md), or ask your IT team"}
}

// DoctorDestinationOK describes a writable destination.
func DoctorDestinationOK(dest string) string {
	return "Destination " + dest + " exists and is writable"
}

// DoctorStateOK describes the state folder.
func DoctorStateOK(dir string) string { return "State folder " + dir + " is on a local disk" }

// DoctorRunning notes that a run or scheduler is active.
func DoctorRunning(profile string) string {
	return "synctoceph is running for profile " + profile + " right now (see synctoceph status)"
}

// DoctorTimeZone names the time zone for daily schedules.
func DoctorTimeZone(name string) string { return "Time zone for daily runs: " + name }

// DoctorSchedule describes the schedule.
func DoctorSchedule(text string) string { return "Schedule: " + text }

// DoctorServiceSystemd describes an installed systemd service.
func DoctorServiceSystemd(unit, active string) string {
	return "Automatic runs: systemd user service " + unit + " (" + active + ")"
}

// DoctorServiceTask describes an installed Task Scheduler task.
func DoctorServiceTask(task string) string {
	return "Automatic runs: Windows Task Scheduler task " + task
}

// DoctorNoService explains how automatic runs could be set up.
func DoctorNoService(manager string, wsl bool) string {
	switch manager {
	case "systemd":
		text := "Automatic runs are not set up. Set them up with: synctoceph service install"
		if wsl {
			text += "\nOn WSL, check docs/scheduling.md first: WSL may shut down when no terminal is open."
		}
		return text
	case "task-scheduler":
		return "Automatic runs are not set up. Set them up (Windows Task Scheduler) with: synctoceph service install"
	}
	return "Automatic runs cannot be set up here: no systemd and no Windows Task Scheduler found.\n" +
		"Run `synctoceph schedule` in a terminal instead, or see docs/scheduling.md."
}

// DoctorResult is doctor's final line.
func DoctorResult(problems int) string {
	if problems == 0 {
		return "No problems found."
	}
	return fmt.Sprintf("%s found; fix the ERROR lines above, then run synctoceph doctor again.", plural(problems, "problem"))
}

// Service messages.
const (
	TaskCommandIntro = "Create the Windows Task Scheduler task with this command (in WSL or a Windows terminal):"
)

// NoServiceManager: neither systemd nor Task Scheduler is available.
func NoServiceManager() error {
	return &Problem{What: "no service manager was found (no systemd, and not WSL with schtasks.exe)",
		Fix: "run `synctoceph schedule` in a terminal, or enable systemd (see docs/scheduling.md)"}
}

// ServiceFailed: installing or removing a service failed.
func ServiceFailed(err error) error {
	return &Problem{What: "setting up automatic runs failed", Err: err,
		Fix: "see docs/scheduling.md; `synctoceph service install --print` shows what would be set up"}
}

// WouldWriteUnit introduces the printed unit file.
func WouldWriteUnit(path string) string { return "Would write " + path + ":\n" }

// SystemdEnableCommands shows how to enable a unit by hand.
func SystemdEnableCommands(unit string) string {
	return "Then run:\n  systemctl --user daemon-reload\n  systemctl --user enable --now " + unit
}

// LingerOff explains lingering.
func LingerOff() string {
	return "Your user services stop when you log out, so scheduled runs would stop too.\n" +
		"To keep them running, ask an administrator to run: sudo loginctl enable-linger $USER"
}

// SystemdInstalled confirms a systemd install.
func SystemdInstalled(unit, profile string) string {
	return "Automatic runs are set up: systemd user service " + unit + " is enabled and started.\n" +
		"Check with: synctoceph status --profile " + profile + "   (logs: synctoceph logs)"
}

// TaskInstalled confirms a Task Scheduler install.
func TaskInstalled(task, profile string) string {
	return "Automatic runs are set up: Windows Task Scheduler task " + task + ".\n" +
		"It runs only while you are logged in to Windows; see docs/scheduling.md to change that.\n" +
		"Check with: synctoceph status --profile " + profile
}

// ServiceRemoved confirms removal.
func ServiceRemoved(name string) string { return "Removed " + name }

// ServiceStatusReport prints `synctoceph service status`.
func ServiceStatusReport(p *Printer, profile, manager string, unit bool, enabled, active string, task bool, detail string) {
	p.Plain("Profile:              " + profile)
	p.Plain("Service manager here: " + manager)
	if unit {
		p.Plain("systemd user service: " + "synctoceph-" + profile + ".service, " + enabled + ", " + active)
	} else {
		p.Plain("systemd user service: not installed")
	}
	if task {
		p.Plain("Task Scheduler task:  synctoceph-" + profile + " " + detail)
	} else if manager == "task-scheduler" {
		p.Plain("Task Scheduler task:  not installed")
	}
}
