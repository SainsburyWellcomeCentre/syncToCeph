// This file holds the wording of every problem synctoceph can report about
// its setup: config, folders, mounts, rsync and locks. Each one says what
// happened, why it matters and what to do next. Change wording here without
// touching any logic. Run-result wording is in messages_run.go, wording for
// setup commands in messages_setup.go, and command help in help.go.
package ui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/state"
)

// GenericFix is shown for unexpected errors that have no specific advice.
const GenericFix = "check the log with `synctoceph logs`, then run `synctoceph doctor`."

// Rules shown when a setting has a wrong value.
const (
	MachineNameRule  = "use 1 to 63 letters, digits, dots, dashes or underscores, starting with a letter or digit"
	ExcludeRule      = "use a simple pattern such as \"*.tmp\" or \"Thumbs.db\" (* ? and [abc] are allowed)"
	AbsolutePathRule = "use a full path starting with /, for example /mnt/d/acquisition"
)

// Reasons a source entry is skipped.
const (
	ReasonReserved = "reserved name (.syncToCeph and .syncToCeph-partial are used by synctoceph)"
	ReasonSymlink  = "symlink (never followed)"
	ReasonSpecial  = "special file (pipe, socket or device)"
)

// Descriptions of archive entries that block a copy.
const (
	ArchiveEntryIsSymlink = "the archive has a symlink at this path"
	ArchiveEntryNotFile   = "the archive has a folder (or other non-file) at this path"
)

// BadProfile: an invalid --profile name.
func BadProfile(name string) error {
	return &Problem{What: fmt.Sprintf("%q is not a valid profile name", name),
		Fix: "use letters, digits, dashes or underscores, for example --profile scope2"}
}

// NoHome: the home folder cannot be found.
func NoHome(err error) error {
	return &Problem{What: "cannot find your home folder", Err: err,
		Why: "config and state files are kept under your home folder",
		Fix: "make sure the HOME environment variable is set to your home folder"}
}

// NoConfig: the profile has no config file yet.
func NoConfig(file string) error {
	return &Problem{What: "no configuration found at " + file,
		Why: "synctoceph has no built-in data folders; it needs to be told what to copy and where",
		Fix: "run `synctoceph init` (add --profile NAME for a profile other than default)"}
}

// BadConfigFile: the config file is not valid TOML.
func BadConfigFile(file string, err error) error {
	return &Problem{What: "cannot read the configuration " + file, Err: err,
		Why: "synctoceph will not guess settings",
		Fix: "fix the line mentioned above (see docs/configuration.md), or re-create the file with `synctoceph init --force`"}
}

// UnknownConfigKeys: the config file has keys synctoceph does not know.
func UnknownConfigKeys(file string, keys []string) error {
	return &Problem{What: fmt.Sprintf("unknown setting(s) in %s: %s", file, strings.Join(keys, ", ")),
		Why: "a misspelt setting would otherwise be silently ignored; extra rsync options are never accepted",
		Fix: "remove or correct them; valid settings are listed in docs/configuration.md"}
}

// BadSetting: a setting has an invalid value.
func BadSetting(key, value, rule string) error {
	return &Problem{What: fmt.Sprintf("the setting %s = %q is not valid", key, value),
		Fix: rule + " (edit the config file, or run `synctoceph init --force`)"}
}

// MissingSetting: a required setting is empty.
func MissingSetting(key string) error {
	return &Problem{What: "the setting " + key + " is missing",
		Why: "synctoceph has no built-in data folders",
		Fix: "run `synctoceph init`, or add " + key + " to the config file"}
}

// BothSchedules: interval and at are both set.
func BothSchedules() error {
	return &Problem{What: "both interval and at are set",
		Fix: "keep one: interval (e.g. \"4h\") for regular runs, or at (e.g. \"02:00\") for a daily run"}
}

// StateNotLocal: the state folder is on a Windows drive or network share.
func StateNotLocal(dir, kind string) error {
	return &Problem{What: "the state folder " + dir + " is on " + kind,
		Why: "locks and the control socket do not work reliably there, so two runs could overlap",
		Fix: "use a folder on the Linux disk: unset XDG_STATE_HOME, or set it to a folder under your Linux home (e.g. ~/.local/state)"}
}

// StateProblem explains why the state folder cannot be used.
func StateProblem(dir string, err error) error {
	var notLocal *state.NotLocalError
	if errors.As(err, &notLocal) {
		return StateNotLocal(dir, notLocal.Kind)
	}
	return StateUnusable(dir, err)
}

// StateUnusable: the state folder cannot be used.
func StateUnusable(dir string, err error) error {
	return &Problem{What: "cannot use the state folder " + dir, Err: err,
		Why: "the status, log and lock live there",
		Fix: "make sure it is a folder you own that others cannot write to, e.g. `chmod 700 " + dir + "`"}
}

// RsyncMissing: rsync is not installed.
func RsyncMissing() error {
	return &Problem{What: "rsync is not installed (or not on PATH)",
		Why: "synctoceph uses rsync to copy files",
		Fix: "install rsync 3.2.4 or newer, e.g. `sudo apt install rsync`"}
}

// RsyncUnknown: rsync's version cannot be determined.
func RsyncUnknown(path string) error {
	return &Problem{What: "cannot tell which rsync version " + path + " is",
		Why: "some systems ship an rsync replacement without the options synctoceph needs",
		Fix: "install the standard rsync 3.2.4 or newer, e.g. `sudo apt install rsync`"}
}

// RsyncTooOld: rsync is older than required.
func RsyncTooOld(path, version string) error {
	return &Problem{What: fmt.Sprintf("rsync %s at %s is too old", version, path),
		Why: "synctoceph needs rsync 3.2.4 or newer (for --fsync, which makes each copy durable before it gets its final name)",
		Fix: "update rsync, e.g. `sudo apt update && sudo apt install rsync`"}
}

// ArchiveSymlink: a symlink on the archive path.
func ArchiveSymlink(path string, err error) error {
	return &Problem{What: "the archive path " + path + " goes through a symlink", Err: err,
		Why: "a symlink could send copies somewhere other than the archive",
		Fix: "set archive (and require_mount) to the real folder; `realpath " + path + "` shows it"}
}

// ArchiveMissing: the archive root does not exist.
func ArchiveMissing(archive string, err error) error {
	return &Problem{What: "the archive folder " + archive + " does not exist or is not reachable", Err: err,
		Why: "synctoceph never creates the archive folder, so that it can never write into an empty mount point by mistake",
		Fix: "check that the lab share is mounted (`synctoceph doctor` shows how), or correct archive in the config"}
}

// MachineDirNotFolder: the machine folder exists but is not a folder.
func MachineDirNotFolder(dir string) error {
	return &Problem{What: dir + " exists but is not a folder",
		Fix: "rename or remove it, or choose another machine_name"}
}

// SourceMissing: the source folder is missing.
func SourceMissing(source string, err error) error {
	return &Problem{What: "the source folder " + source + " does not exist or cannot be opened", Err: err,
		Why: "there is nothing to copy",
		Fix: "check that the data drive is connected (on WSL, e.g. /mnt/d), or correct source in the config"}
}

// SourceUnreadable: the source cannot be scanned.
func SourceUnreadable(source string, err error) error {
	return &Problem{What: "cannot read the source folder " + source, Err: err,
		Fix: "check that the drive is connected and that you can read the folder"}
}

// ScanError: one folder or file in the source could not be read.
func ScanError(rel string, err error) string {
	return fmt.Sprintf("cannot read %s in the source (%v); it was not archived. Check its permissions", rel, err)
}

// MountNotContainingArchive: require_mount does not contain archive.
func MountNotContainingArchive(mount, archive string) error {
	return &Problem{What: "require_mount " + mount + " does not contain the archive " + archive,
		Fix: "set require_mount to the mount point of the lab share, for example /mnt/z"}
}

// MountTableUnreadable: /proc/self/mountinfo cannot be read.
func MountTableUnreadable(err error) error {
	return &Problem{What: "cannot read the list of mounted drives", Err: err,
		Why: "synctoceph must check that the lab share is mounted before writing to it",
		Fix: "run synctoceph on Linux or WSL2"}
}

// NotMounted: the required mount is not mounted.
func NotMounted(mount, archive string) error {
	return &Problem{What: mount + " is not mounted",
		Why: "without the lab share, " + archive + " would be an empty local folder; synctoceph never writes there",
		Fix: "mount the share (`synctoceph doctor` prints the commands), then run again"}
}

// ArchiveOnOtherFilesystem: archive is not on the required mount.
func ArchiveOnOtherFilesystem(mount, archive string) error {
	return &Problem{What: "the archive " + archive + " is not on the filesystem mounted at " + mount,
		Why: "another drive is mounted inside it, so the check that the share is present would be meaningless",
		Fix: "set require_mount to the mount point that actually holds the archive (`findmnt -T " + archive + "` shows it)"}
}

// PathsOverlap: two of source, archive and state are nested.
func PathsOverlap(aName, aPath, bName, bPath string) error {
	return &Problem{What: fmt.Sprintf("the %s (%s) and the %s (%s) overlap", aName, aPath, bName, bPath),
		Why: "one would be copied into the other, or the archive into itself",
		Fix: "use separate folders, none inside another"}
}

// BadTimeZone: TZ names an unknown time zone.
func BadTimeZone(tz string) error {
	return &Problem{What: fmt.Sprintf("the time zone TZ=%q is not known", tz),
		Why: "daily runs would start at the wrong time",
		Fix: "set TZ to a name such as Europe/London, or unset it to use the system time zone"}
}

// AlreadyRunning: another process holds the state lock.
func AlreadyRunning(profile, mode string, pid int) error {
	what := "synctoceph is already running for profile " + profile
	if mode != "" && pid > 0 {
		what += fmt.Sprintf(" (%s, process %d)", mode, pid)
	}
	return &Problem{What: what,
		Why: "only one run per profile may work at a time",
		Fix: "wait for it to finish (`synctoceph status`), or stop it with `synctoceph stop`"}
}

// OutsideSource: a path given on the command line is not in the source.
func OutsideSource(arg, source string) error {
	return &Problem{What: arg + " is not inside the source folder " + source,
		Fix: "give a path inside the source folder, or use --profile for another source"}
}
