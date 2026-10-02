// This file holds the help text of every command (what `synctoceph <command>
// --help` prints). docs/cli/ is generated from it with scripts/gen-docs.sh,
// so run that script after changing anything here.
package ui

// Help texts: a one-line summary (Short) and a longer description (Long) per
// command.
const (
	RootShort = "Copy acquisition data to ceph with rsync, verified with SHA-256"
	RootLong  = `synctoceph copies data from this acquisition computer to the lab's ceph
storage (or another mounted network drive), using rsync. Run it once, or let it
run in the background on a schedule.

The source holds one folder per animal. Each animal folder is copied into the
animal's folder on ceph, inside this machine's subfolder:
  <source>/<animal>/...  ->  <destination>/<animal>/<subfolder>/...
so other acquisition machines can fill other subfolders of the same animal.

synctoceph only adds files: it never deletes or moves anything, on ceph or on
this computer. A file is reported as copied only after the SHA-256 of the
source and of the ceph copy match.

Start with: synctoceph init, then synctoceph doctor, then synctoceph run --dry-run.

To copy several folders (each with its own source, destination or subfolder),
give each its own profile: synctoceph --profile NAME init. Run them all with
synctoceph run --all-profiles.`

	InitShort = "Create the configuration for this computer"
	InitLong  = `Asks for the source folder (one folder per animal), the destination folder on
ceph, this machine's subfolder name, the mount point that must be present,
and the schedule. Each answer is checked, then the configuration is written
to ~/.config/synctoceph/<profile>.toml.

Every question can also be answered with a flag; with --yes, questions not
answered by a flag use their default. The destination folder must already
exist: synctoceph never creates it.

For a second job (another source, destination or subfolder), create another
profile: synctoceph --profile NAME init.`

	DoctorShort = "Check the setup and explain how to fix problems"
	DoctorLong  = `Checks the configuration, rsync, the source folder (and lists its animal
folders), the ceph mount (and its filesystem type), the destination folder,
the state folder, the time zone and the service, and notes when another
profile copies into the same subfolder. Each problem is printed with how to
fix it. Nothing is changed.
--all-profiles checks every profile.

Exit code 0 means no problems were found.`

	RunShort = "Copy new files to ceph and verify them"
	RunLong  = `Runs one sync: scans the source, copies files that are not on ceph yet (each
animal folder into <destination>/<animal>/<subfolder>/), and verifies every
copy with SHA-256. Files directly in the source, outside any animal folder,
are not copied and are listed.

Files already on ceph are left alone unless the config has
existing = "replace" or --existing replace is given; then the old version is
kept under <destination>/.syncToCeph/<subfolder>/history/<run-id>/.
Files modified within settle_time, or that change during the run, are
deferred to a later run.

Each file is listed as it is copied and verified (setting: verbose). For one
run, --verbose=false shows only the steps and the result, -v lists every file
and -q prints only the result.

--all-profiles runs every profile one after another and ends with a summary;
the exit code is that of the worst result.

Exit codes: 0 OK, 1 FAILED, 2 usage error, 3 PARTIAL (files deferred),
130 INTERRUPTED.`

	ScheduleShort = "Run syncs on the configured schedule (used by the service)"
	ScheduleLong  = `Stays running and starts a sync on the schedule set by interval or at in the
config file. Runs never overlap, and missed runs are not replayed. Stop it
with Ctrl-C or synctoceph stop.

Normally started by the service (synctoceph service install).`

	StatusShort = "Show what synctoceph is doing and the last result"
	StatusLong  = `Shows whether a run or the scheduler is active, the result of the last run,
the next scheduled run, and files that are deferred or differ from their
copy on ceph. --all-profiles shows one line per profile instead. Reads only; it
never starts anything.`

	LogsShort = "Show recent log lines"
	LogsLong  = `Prints the last lines of the log (kept in the state folder, rotated at 5 MB,
with 5 older files kept). --run shows only the lines of one run.`

	StopShort = "Stop a running sync or scheduler gracefully"
	StopLong  = `Asks the running synctoceph for this profile to stop. rsync is asked to stop
(SIGINT) and keeps its partial file so the next run can resume; if it has not
stopped after --timeout seconds it is killed. An interrupted run is never
reported as verified.`

	VerifyShort = "Re-check copied files with SHA-256"
	VerifyLong  = `Reads each source file under PATH (default: the whole source) and its copy on
ceph, compares their SHA-256, and records the ones that match as verified.
Use it once for data copied before synctoceph was used, or to check the
copies at any time. Exit code 0 means every file under PATH was verified.`

	CheckCopiedShort = "Check whether files are safely on ceph before deleting them"
	CheckCopiedLong  = `For each source file under PATH, reports whether it is verified on ceph: the
copy has the same size and time, and its SHA-256 was checked against the
source. Exit code 0 means every file under PATH is verified, so it is safe
to delete PATH from this computer.

This uses the verified-file record and is fast. To re-read the files first,
run synctoceph verify PATH.`

	HistoryShort     = "List and restore previous versions of replaced files"
	HistoryListShort = "List runs that kept previous versions, or the files of one run"
	HistoryRestShort = "Copy a previous version out of the history"
	HistoryRestLong  = `Copies the previous version of PATH (a file or folder, given as its path in
the source, such as LUMS0014/session1) kept by run RUN_ID into DIR, as
DIR/PATH. DIR must be outside the destination, and existing files are never
overwritten. Each copy is checked with SHA-256.`

	FleetShort = "Show the latest sync of every machine copying to the destination"
	FleetLong  = `Reads the run summaries that every machine writes to
<destination>/.syncToCeph/<subfolder>/ and shows, per subfolder, the last run,
the computer it ran on, the last successful sync, deferred files and
problems. Reads only. Uses the destination from this profile's config unless
--destination is given.`

	ServiceShort       = "Set up, remove or check automatic runs"
	ServiceInstallLong = `Sets up automatic runs on the schedule from the config file:
  - with systemd (Linux, or WSL with systemd): a user service running
    "synctoceph schedule";
  - on WSL without systemd: a Windows Task Scheduler task running
    "wsl.exe -d <distro> -- synctoceph run".
--print shows what would be set up without changing anything.
--all-profiles sets up every profile that has a schedule (one service or
task each).`
	ServiceUninstallShort = "Remove automatic runs"
	ServiceStatusShort    = "Show whether automatic runs are set up"
	ServiceInstallShort   = "Set up automatic runs"

	CompletionShort = "Print a shell completion script (bash or zsh)"
	CompletionLong  = `Prints a script that lets the shell complete synctoceph commands and flags.
install.sh installs it for you. To load it by hand in bash:
  source <(synctoceph completion bash)`

	VersionShort = "Show the version"
)
