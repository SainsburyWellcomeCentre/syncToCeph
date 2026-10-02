[Home](../README.md) · [All documentation](README.md)

# Design

This is the specification of `synctoceph`: what it does and why. The safety
promises are listed in [safety-model.md](safety-model.md); the command
reference is in [cli/](cli/synctoceph.md).

## Deployment

```
acquisition PC (Linux, or Windows + WSL2), subfolder = "behaviour"
  source:       /mnt/d/luminoseData/          a Windows data drive seen from WSL, or a Linux folder
                  LUMS0014/...                one folder per animal
                  LUMS0015/...
  destination:  /mnt/ceph/<project>/          ceph (or another network drive), already mounted
                  LUMS0014/
                    behaviour/...             this machine only ever writes here
                    ephys/...                 written by the ephys rig
                    histology/...             written by the histology scope
                  LUMS0015/
                    behaviour/...
                  .syncToCeph/
                    behaviour/                this machine's records (see below)
                    ephys/
```

- The destination is organised by animal. Every folder directly in the
  source is one animal; its contents go to
  `<destination>/<animal>/<subfolder>/`, keeping their paths. Files directly
  in the source, outside any animal folder, are skipped and reported.
- Each acquisition machine (each profile) has its own `subfolder`, named for
  the kind of data (for example `behaviour`, `ephys`, `histology`), and
  writes only inside that subfolder of each animal folder. Machines may
  share animal folders but never a subfolder, so they never collide.
  synctoceph creates `<destination>/<animal>/` and its subfolder when they
  are missing, one level at a time; it never creates the destination itself.
- ceph is reached through a mount that already exists (cifs through
  `/etc/fstab`; see [mounting.md](mounting.md)). There is no SSH mode and
  nothing runs on the storage side. Other network drives (CephFS, NFS, or
  drvfs for a Windows drive) work the same way.
- Locks are local only. The state lock stops two runs of one profile; the
  destination lock (`.syncToCeph/<subfolder>/lock`) stops two profiles on the
  same computer from writing the same subfolder. Neither coordinates between
  computers; separate subfolders do that.

## Files and locations

Nothing depends on the current working directory.

| What | Where |
|---|---|
| Config | `${XDG_CONFIG_HOME:-~/.config}/synctoceph/<profile>.toml` |
| State (status, log, lock, control socket) | `${XDG_STATE_HOME:-~/.local/state}/synctoceph/<profile>/` |
| Go toolchain used by `install.sh` | `${XDG_CACHE_HOME:-~/.cache}/synctoceph/go<version>/` |
| Program | `~/.local/bin/synctoceph` (or `<prefix>/bin`) |
| Install record | `${XDG_DATA_HOME:-~/.local/share}/synctoceph/installed-files.txt` |

The state folder must be on a local Linux disk. If it is on a Windows drive
(`/mnt/<letter>/...`, drvfs) or a network share, `run`, `schedule`, `verify` and
`doctor` refuse with an explanation: locks and sockets are not reliable there.

## Configuration

See [configuration.md](configuration.md). The file is TOML and strict: unknown
keys are errors. Precedence is command-line flag, then config file, then
built-in default. There are no built-in data folders; `synctoceph init` must
be run first.

## Several profiles

A profile is one config file (`<profile>.toml`) with its own state folder,
log and lock: one job, with one source, one destination and one subfolder.
One computer can have any number of profiles, for example one per kind of
data.

- `run --all-profiles` runs every profile in name order, one after another
  (never in parallel), each with its own report, and ends with a summary.
  Its result is the worst one: INTERRUPTED, then FAILED, then PARTIAL, then
  OK; the exit code follows the table below. A profile that cannot start (a
  config error, or it is already running) counts as FAILED and the next one
  still runs. A stop request ends the current profile and skips the rest.
- `status`, `doctor` and `service install` accept `--all-profiles` too;
  `service uninstall --all-profiles` (or `--all`) removes every profile's
  automatic runs. `--profile` and `--all-profiles` cannot be combined.
- Two profiles may write the same subfolder of the same destination (the
  destination lock makes them take turns). `init` and `doctor` print a NOTE
  when they do, because files with the same path in both sources would meet
  in one place.

## How a run works

1. **Preflight** (nothing is written if any check fails):
   - if `require_mount` is set: it is a mount point in `/proc/self/mountinfo`,
     it contains the destination, and the destination is on that filesystem;
   - no part of the destination path (up to and including
     `.syncToCeph/<subfolder>`) is a symlink;
   - the destination exists (it is never created);
   - the source exists;
   - source, destination and state folders are separate (none inside
     another, after following symlinks);
   - rsync is 3.2.4 or newer.

   The state lock is taken before preflight. In a real run (not a dry run)
   `.syncToCeph/<subfolder>/` is then created if missing, and the destination
   lock is taken.
2. **Scan the source** without following symlinks. For every regular file,
   record size, modification time, inode and device. Symlinks, special files
   (pipes, sockets, devices), files directly in the source (outside an animal
   folder) and the reserved names `.syncToCeph` and `.syncToCeph-partial` are
   skipped and reported. Exclude patterns are applied here (by synctoceph,
   never passed to rsync): a name, a path from the source, or with a trailing
   `/` folders only; `exclude_hidden` adds `.*`. Excluded folders are not
   entered. Files modified within `settle_time` are deferred.
3. **Plan.** Each file is compared with its copy at
   `<destination>/<animal>/<subfolder>/<rest of the path>` (size, and
   modification time within one second) and with the verified-file record:
   - not on the destination: copy;
   - on the destination and matching: nothing to copy. If it has no
     verification record, it counts as *unverified*; the run suggests
     `synctoceph verify` the first time the number of such files changes;
   - on the destination and different: `skip` mode lists it as *differing*;
     `replace` mode copies it and rsync moves the old version to history;
   - a symlink or a file/folder type conflict anywhere on the destination
     path (including the animal folder and the subfolder): reported as an
     error for that file, nothing written there.
4. **Check existing files** (only with `verify = all`, and for files an earlier
   interrupted run copied without verifying): hash source and destination
   copy. A mismatch becomes *differing* (skip mode) or is copied again
   (replace mode).
5. **Transfer** with rsync, once per animal folder that has files to copy.
   `<destination>/<animal>/` and `<destination>/<animal>/<subfolder>/` are
   created first if missing (one level at a time, each checked not to be a
   symlink). The option list is fixed:

   ```
   rsync --files-from=- --from0 --times --omit-dir-times
         --partial-dir=.syncToCeph-partial --fsync --itemize-changes
         [skip:    --ignore-existing]
         [replace: --backup --backup-dir=<destination>/.syncToCeph/<subfolder>/history/<run-id>/<animal> --ignore-times]
         -- <source>/<animal>/ <destination>/<animal>/<subfolder>/
   ```

   The file list (paths inside the animal folder) is sent NUL-separated on
   standard input, so any file name is passed literally. `--files-from` implies `--relative` (paths are kept) and
   `--dirs`. rsync's environment has `RSYNC_*` variables removed, `LC_ALL=C`,
   and `HOME` set to the state folder (so `~/.popt` aliases cannot add
   options). rsync runs in its own process group and inherits the lock files.
   Output is streamed line by line into the log; itemized lines (`>f...`)
   count the copied files. A stop request ends the current rsync and starts
   no further one.
6. **Verify** every file on the copy list: SHA-256 of source and destination
   copy must match, and the source must still have the size, modification time,
   inode and device from the scan (checked before and after hashing). A
   changed or vanished source file is **deferred, not failed**. Verified files
   are appended to the verified-file record.
7. **Record the result**: `status.json` in the state folder (atomic, with a
   schema version), and, except for dry runs, the run summary in
   `<destination>/.syncToCeph/<subfolder>/runs/<run-id>.json` and
   `runs/latest.json`.

A **dry run** does steps 1 to 3 only and writes nothing to the destination;
it does not start rsync.

When rsync exits with an error for one animal folder, the other animal
folders are still copied and verification still runs, so completed copies
are recorded and each missing file is reported. rsync's exit code 24 ("some
files vanished") is not an error by itself: the vanished files are deferred.

## Results and exit codes

| Result | Meaning | Exit code |
|---|---|---|
| OK | everything planned was copied and verified | 0 |
| FAILED | a preflight, rsync or verification error | 1 |
| (usage) | a mistake in the command line | 2 |
| PARTIAL | some files were deferred | 3 |
| INTERRUPTED | stopped by `synctoceph stop`, Ctrl-C or SIGTERM | 130 |

Files that differ in skip mode, unverified files and skipped symlinks are
reported as NOTE lines and do not change the result. A file deferred for more
than 24 hours is shown as a WARNING in `status` and `fleet`.

Output lines start with a plain marker (`OK`, `NOTE`, `DEFERRED`, `WARNING`,
`ERROR`, `RESULT`). Colour is used only on a terminal and never when
`NO_COLOR` is set or `--no-color` is given; the verdict on the RESULT line is
green (OK, SAFE), yellow (PARTIAL, INTERRUPTED) or red (anything else). Every
problem says what happened, why it matters and what to do.

While `run` or `schedule` works, it prints a header (profile, source,
destination as `<destination>/<animal>/<subfolder>`, settings), then one `==>` line per step with an indented summary
after the scan and the plan. With `verbose = true` (the default; `-v` and
`--verbose=false` override it for one command) every file is listed as it is
copied (`[n/total] copied PATH SIZE`), verified and deferred; file names with
control characters are shown quoted. rsync's raw output goes only to the
log, except its warnings and errors, which are also shown when verbose. `-q`
prints only RESULT and ERROR lines. Example output is in
[operations.md](operations.md#what-a-run-shows).

## Records on the destination

Each subfolder (usually each acquisition machine) keeps its records in one
place, next to the animal folders rather than inside them:

```
<destination>/.syncToCeph/<subfolder>/
  lock                  same-computer guard (see Deployment)
  manifest.jsonl        verified-file record, one JSON line per verification
  history/<run-id>/     previous versions of replaced files (never pruned)
  runs/<run-id>.json    run summaries; runs/latest.json is read by fleet
```

Paths in these records are paths in the source, starting with the animal
folder (`LUMS0014/2026-10-01/events.csv`); the copy is at
`<destination>/LUMS0014/<subfolder>/2026-10-01/events.csv`. History keeps the
same form: `history/<run-id>/LUMS0014/2026-10-01/events.csv`.

- **manifest.jsonl** lines: `schema_version`, `path`, `size`, `mtime`,
  `sha256`, `verified_at`, `run_id`. Lines are only appended. When the file has
  at least 10,000 lines and more than two lines per file it describes, it is
  rewritten with the latest line per file (atomically). A line cut short by a
  crash is ignored.
- **Run summaries** hold the counts in full and at most 100 entries of each
  file list.
- `.syncToCeph/<subfolder>/runs/` is readable by others (mode 755 under the
  umask), so `fleet` works from any machine that can read the destination.
  `fleet` shows one line per subfolder, with the host name of the computer
  that ran last.

## Scheduling

- `interval`: the next run starts `interval` after the previous run ended.
  If the previous run ended longer ago than that (for example after a reboot),
  one run starts at once; missed runs are never replayed.
- `at`: daily at a local time, using `TZ` if set (an unknown `TZ` is an error)
  or the system time zone. A time skipped by a daylight-saving change does not
  run that day; a time that happens twice runs once. A daily time that was
  missed (the computer was off) is not made up.
- `schedule --immediate` also runs once at start-up, without using up that
  day's daily run.
- Runs never overlap: the scheduler holds the state lock for as long as it
  runs, so `synctoceph run` refuses while it is active.
- Waiting is done in steps of at most 10 seconds against the wall clock, so a
  computer that slept still starts the run at the right time.

See [scheduling.md](scheduling.md) for systemd and Task Scheduler.

## Process control

- The state lock (`flock` on `<state>/lock`) shows whether a run or scheduler
  is alive. rsync inherits it, so the lock stays held while rsync runs, even if
  synctoceph was killed.
- A Unix socket, `<state>/control.sock`, carries `stop` and live status. Paths
  longer than the 107-character socket limit are reached through
  `/proc/self/fd`.
- Stop (`synctoceph stop`, SIGINT or SIGTERM) sends SIGINT to rsync's process
  group, then SIGKILL after 30 seconds (`stop --timeout S` changes this). The
  run ends INTERRUPTED; files it copied are verified by the next run.

## Decisions taken while building (for the owner to confirm)

The animal-first layout (2026-10-02) was requested by the owner; the choices
made in implementing it are listed here too.

These were marked *proposed* or open in the original brief. They were
implemented as follows and can be changed:

- **Exit codes**: as proposed (0, 1, 2, 3, 130).
- **Default `settle_time`**: 10 minutes.
- **History pruning**: none, not even a manual command. History is only ever
  added to.
- **rsync minimum**: 3.2.4, not 3.2.0, because `--fsync` appeared in 3.2.4.
- **Unverified files**: counted every run (shown in `status`); the suggestion
  to run `synctoceph verify` is printed when the count changes.
- **Empty folders** in the source are not copied (only files and the folders
  that contain them).
- **Modification times** are compared with a one-second tolerance, because
  Windows drives keep only whole seconds through rsync (see
  [troubleshooting.md](troubleshooting.md#windows-drives-drvfs)).
- **Animal folders**: every folder directly in the source counts as an animal;
  there is no name pattern. Unwanted top-level folders are left out with
  `exclude` (for example `"/LUMS0099"`), and hidden ones with
  `exclude_hidden`.
- **`exclude_hidden`** is `false` when missing from the config, so nothing is
  left out unless asked, but `init` writes `true`, so new configs skip
  `.git`, `.Trash-1000` and similar.
- **Files outside animal folders** are skipped and reported on every run, not
  copied anywhere.
- **`subfolder` is required**, with no default, so two machines never fall
  into the same subfolder by accident.
- **Records live in `<destination>/.syncToCeph/<subfolder>/`**, one place per
  subfolder, so animal folders hold only data.
- **No migration** from the earlier `<archive>/<machine_name>/` layout: data
  already copied there stays where it is, and the new layout is filled by
  new runs (see the changelog).

---

Previous: [Safety model](safety-model.md) · Next: [Command reference](cli/synctoceph.md) · [All documentation](README.md) · [Home](../README.md)
