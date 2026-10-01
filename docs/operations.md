# Operations

Day-to-day use: checking on runs, reading logs, stopping, recovering, and the
machine-readable outputs.

## Checking

```
synctoceph status                # what is running, last result, next run
synctoceph status --deferred     # files waiting for a later run
synctoceph status --differing    # archive files that differ and were not replaced
synctoceph logs -n 100           # recent log lines
synctoceph logs --run RUN_ID     # the lines of one run
synctoceph fleet                 # every machine that writes to the archive
```

Use `--profile NAME` for a profile other than `default`.

## Before deleting data from an acquisition PC

```
synctoceph check-archived /mnt/d/acquisition/session_42
```

It lists every file under the path that is not verified in the archive, and
exits with 0 only if all of them are. It uses the verified-file record and is
fast. To re-read the files with SHA-256 first:

```
synctoceph verify /mnt/d/acquisition/session_42
```

Excluded and skipped entries (symlinks, special files) are never archived, so a
folder containing them is never reported as safe to delete.

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
the archive and resumed by the next run; files copied before the stop are
verified by the next run.

## Recovery

| Situation | What to do |
|---|---|
| A run failed | Read the ERROR lines (`synctoceph status`, `synctoceph logs --run RUN_ID`), fix the cause, run again. Copies that completed are kept and verified. |
| The computer crashed during a run | Nothing to clean up: the lock is released by the operating system, partial copies wait in `.syncToCeph-partial/`, and only verified files are in the record. Run again. |
| Files keep being deferred | Something is still writing them, or the clock is wrong. After 24 hours they are shown as a WARNING. |
| A file differs in the archive | `skip` mode leaves it. To keep both versions: `synctoceph run --existing replace` (the old one goes to history). |
| An old version is needed | `synctoceph history list`, then `synctoceph history restore RUN_ID PATH --to DIR`. |
| Data was copied before synctoceph was used | Run `synctoceph verify` once, so those files are recorded as verified. |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | OK |
| 1 | FAILED (details in `synctoceph status --json`; rsync's own code is in `rsync_exit_code`). For `verify` and `check-archived`: not every file is verified. `doctor`: problems found. |
| 2 | Command-line usage error |
| 3 | PARTIAL: some files were deferred |
| 130 | INTERRUPTED |

## Machine-readable output

Every read command accepts `--json`: `status`, `logs`, `doctor`, `verify`,
`check-archived`, `history list`, `fleet`, `service status` and `version`.
Each output has `"schema_version": 1`. The run summary (in `status --json`
under `status.last_run`, and in the archive under `.syncToCeph/runs/`) has
these fields:

| Field | Meaning |
|---|---|
| `run_id`, `machine_name`, `hostname`, `profile`, `synctoceph_version` | identity of the run |
| `source`, `archive`, `started_at`, `finished_at`, `dry_run`, `existing`, `verify` | settings and times |
| `result`, `exit_code` | OK, PARTIAL, FAILED or INTERRUPTED, and the exit code |
| `scanned_files`, `scanned_bytes`, `excluded_files` | what the scan found |
| `planned_files`, `planned_bytes`, `replaced_files` | what was to be copied |
| `copied_files`, `copied_bytes`, `verified_files`, `verified_bytes` | what was copied and verified |
| `up_to_date_files`, `unverified_files` | files already archived, with and without a verification record |
| `deferred_count`, `deferred` | deferred files (`path`, `reason`, `modified_at`, `since`) |
| `differing_count`, `differing` | archive files that differ and were not replaced |
| `skipped_count`, `skipped` | symlinks, special files and reserved names (`path`, `reason`) |
| `error_count`, `errors` | problems, each with its explanation |
| `would_copy` | dry runs only: files that would be copied |
| `rsync_exit_code`, `history_dir`, `last_success_at` | rsync's exit code, where replaced files went, last OK or PARTIAL run |

In the archive, lists are cut to 100 entries; the `*_count` fields are always
complete. Changing the meaning of a field requires a new `schema_version`.
