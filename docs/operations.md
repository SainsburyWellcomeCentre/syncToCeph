[Home](../README.md) · [All documentation](README.md)

# Operations

Day-to-day use: checking on runs, reading logs, stopping, recovering, and the
machine-readable outputs.

## Checking

```
synctoceph status                # what is running, last result, next run
synctoceph status --deferred     # files waiting for a later run
synctoceph status --differing    # files on ceph that differ and were not replaced
synctoceph logs -n 100           # recent log lines
synctoceph logs --run RUN_ID     # the lines of one run
synctoceph fleet                 # every machine (subfolder) that copies to the destination
```

Use `--profile NAME` for a profile other than `default`, and
`status --all-profiles` for one line per profile.

## What a run shows

```
synctoceph run (profile default)
  From      /mnt/d/luminoseData/<animal>
  To        /mnt/ceph/project/<animal>/behaviour
  Options   existing=skip  verify=new  settle_time=10m

==> Checking the destination, the source and rsync
==> Scanning the source
    Found 1,204 files (38.2 GB)
==> Comparing with ceph
    3 files to copy (1.2 GB); 1,201 already copied and verified
==> Copying 3 files (1.2 GB)
    [1/3] copied    LUMS0014/2026-10-01/stack_0001.tif  400.0 MB
    ...
==> Verifying copies (SHA-256)
    [1/3] verified  LUMS0014/2026-10-01/stack_0001.tif
    ...

OK        Copied and verified 3 files (1.2 GB) in 2m 10s
RESULT    OK: everything that needed copying is on ceph and verified.
```

File paths are shown as in the source (animal folder first); on ceph,
`LUMS0014/2026-10-01/stack_0001.tif` is
`/mnt/ceph/project/LUMS0014/behaviour/2026-10-01/stack_0001.tif`.

Each step starts with `==>`. The `copied` and `verified` lines appear only
when `verbose = true` (the default; see [configuration.md](configuration.md));
`--verbose=false` hides them for one run and `-q` prints only the RESULT line.
On a terminal the verdict is coloured: green for OK, yellow for PARTIAL or
INTERRUPTED, red for FAILED. rsync's own output is in the log
(`synctoceph logs`).

With `run --all-profiles`, each profile's report is followed by a summary:

```
==> Summary of 2 profiles
    default  OK           copied 3 files (1.2 GB)
    video    PARTIAL      nothing new to copy; 1 file left for a later run
RESULT    PARTIAL: 1 of 2 profiles left files for a later run. Do not delete those from the source.
```

## Before deleting data from an acquisition PC

```
synctoceph check-copied /mnt/d/luminoseData/LUMS0014
```

It lists every file under the path that is not verified on ceph, and exits
with 0 only if all of them are. It uses the verified-file record and is
fast. To re-read the files with SHA-256 first:

```
synctoceph verify /mnt/d/luminoseData/LUMS0014
```

Excluded and skipped entries (symlinks, special files, files outside an
animal folder) are never copied, so a folder containing them is never
reported as safe to delete.

## State and logs

The state folder is `${XDG_STATE_HOME:-~/.local/state}/synctoceph/<profile>/`:

| File | Contents |
|---|---|
| `status.json` | current state, last run summary, recent runs, bookkeeping between runs |
| `sync.log` | the log; rotated at 5 MB into `sync.log.1` to `sync.log.5` |
| `lock` | held while a run or the scheduler is active |
| `control.sock` | socket for `stop` and live status; exists only while running |

Each log line has a UTC time, a level, and the run ID in brackets. Line breaks
in messages (for example in file names) are written as `\n`.

## Stopping

```
synctoceph stop                # rsync gets 30 s after SIGINT, then is killed
synctoceph stop --timeout 60
```

Ctrl-C in the terminal and SIGTERM (for example from systemd) do the same. The
run ends INTERRUPTED. An interrupted copy is kept in `.syncToCeph-partial/` in
that animal's subfolder on the destination and resumed by the next run; files
copied before the stop are verified by the next run.

## Recovery

| Situation | What to do |
|---|---|
| A run failed | Read the ERROR lines (`synctoceph status`, `synctoceph logs --run RUN_ID`), fix the cause, run again. Copies that completed are kept and verified. |
| The computer crashed during a run | Nothing to clean up: the lock is released by the operating system, partial copies wait in `.syncToCeph-partial/`, and only verified files are in the record. Run again. |
| Files keep being deferred | Something is still writing them, or the clock is wrong. After 24 hours they are shown as a WARNING. |
| A file differs on ceph | `skip` mode leaves it. To keep both versions: `synctoceph run --existing replace` (the old one goes to history). |
| An old version is needed | `synctoceph history list`, then `synctoceph history restore RUN_ID PATH --to DIR`, with PATH as in the source (for example `LUMS0014/2026-10-01`). |
| A file is reported "not inside an animal folder" | It sits directly in the source folder. Move it into the right animal folder, or leave it; it is never copied where it is. |
| Data was copied before synctoceph was used | Run `synctoceph verify` once, so those files are recorded as verified. |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | OK |
| 1 | FAILED (details in `synctoceph status --json`; rsync's own code is in `rsync_exit_code`). For `verify` and `check-copied`: not every file is verified. `doctor`: problems found. |
| 2 | Command-line usage error |
| 3 | PARTIAL: some files were deferred |
| 130 | INTERRUPTED |

## Machine-readable output

Every read command accepts `--json`: `status`, `logs`, `doctor`, `verify`,
`check-copied`, `history list`, `fleet`, `service status` and `version`.
Each output has `"schema_version": 2`. With `--all-profiles`, `status --json`
prints `{"schema_version": 2, "profiles": [...]}` with one `status` object per
profile, and `doctor --json` prints `{"schema_version": 2, "problems": N,
"profiles": [{"profile", "problems", "checks"}, ...]}`. `check-copied --json`
gives each file a `status` of `verified`, `not-copied`, `unverified`,
`differs`, `excluded` or `skipped`; `verify --json` lists `differing`,
`not_copied`, `changed` and `errors`; `fleet --json` prints
`{"destination", "subfolders": [{"subfolder", "latest"}]}`.

The run summary (in `status --json` under `status.last_run`, and on the
destination under `.syncToCeph/<subfolder>/runs/`) has `schema_version` 2 and
these fields:

| Field | Meaning |
|---|---|
| `run_id`, `subfolder`, `hostname`, `profile`, `synctoceph_version` | identity of the run |
| `source`, `destination`, `started_at`, `finished_at`, `dry_run`, `existing`, `verify` | settings and times |
| `result`, `exit_code` | OK, PARTIAL, FAILED or INTERRUPTED, and the exit code |
| `scanned_files`, `scanned_bytes`, `excluded_files` | what the scan found |
| `planned_files`, `planned_bytes`, `replaced_files` | what was to be copied |
| `copied_files`, `copied_bytes`, `verified_files`, `verified_bytes` | what was copied and verified |
| `up_to_date_files`, `unverified_files` | files already on the destination, with and without a verification record |
| `deferred_count`, `deferred` | deferred files (`path`, `reason`, `modified_at`, `since`) |
| `differing_count`, `differing` | files on the destination that differ and were not replaced |
| `skipped_count`, `skipped` | symlinks, special files, files outside an animal folder and reserved names (`path`, `reason`) |
| `error_count`, `errors` | problems, each with its explanation |
| `would_copy` | dry runs only: files that would be copied |
| `rsync_exit_code`, `history_dir`, `last_success_at` | rsync's exit code, where replaced files went, last OK or PARTIAL run |

On the destination, lists are cut to 100 entries; the `*_count` fields are always
complete. Changing the meaning of a field requires a new `schema_version`.

---

Previous: [Scheduling](scheduling.md) · Next: [Troubleshooting](troubleshooting.md) · [All documentation](README.md) · [Home](../README.md)
