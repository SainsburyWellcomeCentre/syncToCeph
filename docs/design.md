# Design

This is the specification of `synctoceph`: what it does and why. The safety
promises are listed in [safety-model.md](safety-model.md); the command
reference is in [cli/](cli/synctoceph.md).

## Deployment

```
acquisition PC (Linux, or Windows + WSL2)
  source:  /mnt/d/<data>                   a Windows data drive seen from WSL, or a Linux folder
  archive: /mnt/z/<lab-archive>/           lab share, already mounted
             <machine_name>/               this machine only ever writes here
               <copied data...>
               .syncToCeph/                per-machine metadata (see below)
```

- Each acquisition machine writes only into its own folder,
  `<archive>/<machine_name>/`. Machines never share a folder, so they never
  collide.
- The lab share is reached through a mount that already exists. There is no SSH
  mode and nothing runs on the storage side.
- The share is usually mounted inside WSL (CephFS, cifs, NFS, or drvfs for a
  Windows drive). If it is only a Windows mapped drive, it must be mounted in
  WSL first; see [mounting.md](mounting.md).
- Locks are local only. The state lock stops two runs of one profile; the
  archive lock (`.syncToCeph/archive.lock`) stops two profiles on the same
  computer from writing the same machine folder. Neither coordinates between
  computers; separate machine folders do that.

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

## How a run works

1. **Preflight** (nothing is written if any check fails):
   - if `require_mount` is set: it is a mount point in `/proc/self/mountinfo`,
     it contains the archive, and the archive is on that filesystem;
   - no part of the archive path (up to and including the machine folder) is a
     symlink;
   - the archive root exists (it is never created);
   - the source exists;
   - source, archive and state folders are separate (none inside another,
     after following symlinks);
   - rsync is 3.2.4 or newer.

   The state lock is taken before preflight. In a real run (not a dry run) the
   machine folder and `.syncToCeph/` are then created if missing, and the
   archive lock is taken.
2. **Scan the source** without following symlinks. For every regular file,
   record size, modification time, inode and device. Symlinks, special files
   (pipes, sockets, devices) and the reserved names `.syncToCeph` and
   `.syncToCeph-partial` are skipped and reported. Exclude patterns are applied
   here. Files modified within `settle_time` are deferred.
3. **Plan.** Each file is compared with its archive copy (size, and
   modification time within one second) and with the verified-file record:
   - not in the archive: copy;
   - in the archive and matching: nothing to copy. If it has no verification
     record, it counts as *unverified*; the run suggests `synctoceph verify`
     the first time the number of such files changes;
   - in the archive and different: `skip` mode lists it as *differing*;
     `replace` mode copies it and rsync moves the old version to history;
   - a symlink or a file/folder type conflict on the archive path: reported as
     an error for that file, nothing written there.
4. **Check existing files** (only with `verify = all`, and for files an earlier
   interrupted run copied without verifying): hash source and archive copy. A
   mismatch becomes *differing* (skip mode) or is copied again (replace mode).
5. **Transfer** with rsync and a fixed option list:

   ```
   rsync --files-from=- --from0 --times --omit-dir-times
         --partial-dir=.syncToCeph-partial --fsync --itemize-changes
         [skip:    --ignore-existing]
         [replace: --backup --backup-dir=<machine folder>/.syncToCeph/history/<run-id> --ignore-times]
         -- <source>/ <machine folder>/
   ```

   The file list is sent NUL-separated on standard input, so any file name is
   passed literally. `--files-from` implies `--relative` (paths are kept) and
   `--dirs`. rsync's environment has `RSYNC_*` variables removed, `LC_ALL=C`,
   and `HOME` set to the state folder (so `~/.popt` aliases cannot add
   options). rsync runs in its own process group and inherits the lock files.
   Output is streamed line by line into the log; itemized lines (`>f...`)
   count the copied files.
6. **Verify** every file on the copy list: SHA-256 of source and archive copy
   must match, and the source must still have the size, modification time,
   inode and device from the scan (checked before and after hashing). A
   changed or vanished source file is **deferred, not failed**. Verified files
   are appended to the verified-file record.
7. **Record the result**: `status.json` in the state folder (atomic, with a
   schema version), and, except for dry runs, the run summary in
   `<machine folder>/.syncToCeph/runs/<run-id>.json` and `runs/latest.json`.

A **dry run** does steps 1 to 3 only and writes nothing to the archive; it
does not start rsync.

When rsync exits with an error, verification still runs, so completed copies
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
`NO_COLOR` is set or `--no-color` is given. Every problem says what happened,
why it matters and what to do.

## Per-machine archive metadata

```
<archive>/<machine_name>/.syncToCeph/
  archive.lock          same-computer guard (see Deployment)
  manifest.jsonl        verified-file record, one JSON line per verification
  history/<run-id>/     previous versions of replaced files (never pruned)
  runs/<run-id>.json    run summaries; runs/latest.json is read by fleet
```

- **manifest.jsonl** lines: `schema_version`, `path`, `size`, `mtime`,
  `sha256`, `verified_at`, `run_id`. Lines are only appended. When the file has
  at least 10,000 lines and more than two lines per file it describes, it is
  rewritten with the latest line per file (atomically). A line cut short by a
  crash is ignored.
- **Run summaries** hold the counts in full and at most 100 entries of each
  file list.
- `.syncToCeph/runs/` is readable by others (mode 755 under the umask), so
  `fleet` works from any machine that can read the archive.

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
