// This file holds the help text of every command (what `synctoceph <command>
// --help` prints). docs/cli/ is generated from it with scripts/gen-docs.sh,
// so run that script after changing anything here.
package ui

// Help texts: a one-line summary (Short) and a longer description (Long) per
// command.
const (
	RootShort = "Copy acquisition data to the lab archive, verified with SHA-256"
	RootLong  = `synctoceph copies data from this computer into its own folder on the lab's
archive share (<archive>/<machine_name>/), using rsync. It never deletes
anything from the archive or the source. A file is reported as archived only
after the SHA-256 of the source and of the archive copy match.

Start with: synctoceph init, then synctoceph doctor, then synctoceph run --dry-run.

To copy several folders (each with its own source and archive folder), give
each its own profile: synctoceph --profile NAME init. Run them all with
synctoceph run --all-profiles.`

	InitShort = "Create the configuration for this computer"
	InitLong  = `Asks for the machine name, the source folder, the archive folder, the mount
point that must be present, and the schedule. Each answer is checked, then the
configuration is written to ~/.config/synctoceph/<profile>.toml.

Every question can also be answered with a flag; with --yes, questions not
answered by a flag use their default. The archive folder must already exist:
synctoceph never creates it.

For a second job (another source or archive folder), create another profile:
synctoceph --profile NAME init.`

	DoctorShort = "Check the setup and explain how to fix problems"
	DoctorLong  = `Checks the configuration, rsync, the source folder, the archive mount (and its
filesystem type), the archive folder, the state folder, the time zone and the
service, and notes when another profile copies into the same archive
folder. Each problem is printed with how to fix it. Nothing is changed.
--all-profiles checks every profile.

Exit code 0 means no problems were found.`

	RunShort = "Copy new files to the archive and verify them"
	RunLong  = `Runs one sync: scans the source, copies files that are not in the archive yet,
and verifies every copy with SHA-256.

Files already in the archive are left alone unless the config has
existing = "replace" or --existing replace is given; then the old version is
kept under .syncToCeph/history/<run-id>/.
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
the next scheduled run, and files that are deferred or differ from the
archive. --all-profiles shows one line per profile instead. Reads only; it
never starts anything.`

	LogsShort = "Show recent log lines"
	LogsLong  = `Prints the last lines of the log (kept in the state folder, rotated at 5 MB,
with 5 older files kept). --run shows only the lines of one run.`

	StopShort = "Stop a running sync or scheduler gracefully"
	StopLong  = `Asks the running synctoceph for this profile to stop. rsync is asked to stop
(SIGINT) and keeps its partial file so the next run can resume; if it has not
stopped after --timeout seconds it is killed. An interrupted run is never
reported as verified.`

	VerifyShort = "Re-check archived files with SHA-256"
	VerifyLong  = `Reads each source file under PATH (default: the whole source) and its archive
copy, compares their SHA-256, and records the ones that match as verified.
Use it once for data copied before synctoceph was used, or to check the
archive at any time. Exit code 0 means every file under PATH was verified.`

	CheckArchivedShort = "Check whether files are safely archived before deleting them"
	CheckArchivedLong  = `For each source file under PATH, reports whether it is verified in the
archive: the archive copy has the same size and time, and its SHA-256 was
checked against the source. Exit code 0 means every file under PATH is
verified, so it is safe to delete PATH from this computer.

This uses the verified-file record and is fast. To re-read the files first,
run synctoceph verify PATH.`

	HistoryShort     = "List and restore previous versions of replaced files"
	HistoryListShort = "List runs that kept previous versions, or the files of one run"
	HistoryRestShort = "Copy a previous version out of the history"
	HistoryRestLong  = `Copies the previous version of PATH (a file or folder, relative to the
machine folder) kept by run RUN_ID into DIR, as DIR/PATH. DIR must be outside
the archive, and existing files are never overwritten. Each copy is checked
with SHA-256.`

	FleetShort = "Show the latest sync of every machine in the archive"
	FleetLong  = `Reads the run summary that every machine writes to its folder in the archive
and shows each machine's last run, last successful sync, deferred files and
problems. Reads only. Uses the archive from this profile's config unless
--archive is given.`

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
