# Operations reference

For requirements, installation, and common commands, see the [README](../README.md).

## Services and cron

`schedule --background` detaches from the terminal. It does not configure startup
at boot or restart after a crash. System shutdown, WSL shutdown, and service-manager
policies can stop it. Its startup message confirms that the controller is ready;
transfer results appear in `status` and `logs`.

The [systemd template](../examples/syncToCeph.service) runs a foreground scheduler
and restarts the controller on failure. Edit its user, working directory, executable,
and configuration paths before deploying it. For a virtual environment installation,
set `ExecStart` to `/path/to/syncToCeph/.venv/bin/syncToCeph` followed by the existing
arguments. Activation is unnecessary. Direct execution uses `python3` on the
service's `PATH`; an absolute Python path can be used before the repository launcher.

Alternatively, have cron invoke `run`. For example, run daily at 02:00 in cron's
configured timezone, replacing `/path/to/syncToCeph` with the project directory:

```cron
0 2 * * * /path/to/syncToCeph/syncToCeph --config /path/to/syncToCeph/examples/syncToCeph.toml --state-dir /path/to/syncToCeph/.synctoceph-state run
```

Cron needs a `PATH` containing Python and rsync. Data and mount paths in the
configuration should be absolute. A cron job or service timer invokes `run`;
a continuously running service invokes `schedule` without `--background`.
Installation does not install or enable a service.

## Storage and locking

The destination filesystem must support advisory `flock` locks, atomic file
replacement, and `fsync`. One controller holds a lock in its state directory;
each real transfer also holds `DEST/.syncToCeph/archive.lock`. Concurrent clients
sharing an archive need working cross-client locks and compatible permissions.
Other writers must honor that lock to participate in coordination.

State directories must be owned by the process user and not writable by group or
others. The state directory and archive metadata root are created with mode
`0700`; new lock files use `0600` (subject to the process umask).
The same service UID across machines can use these defaults.
Permissions and locking behavior need validation on the deployed Ceph mount.

Source and destination paths must be protected from uncoordinated changes.
Path validation is not protection against hostile changes during filesystem access.
The mount check confirms a mount boundary, not Ceph cluster identity or health.

Rsync uses `--checksum`, `--fsync`, `--backup`, a partial directory, and
`--no-whole-file`. It does not use destination deletion, source removal, or
in-place writes. Completed files replace their destination paths individually.
Rsync manages its temporary/partial files; history is retained without automatic
cleanup. See the [rsync manual](https://download.samba.org/pub/rsync/rsync.1) for
these option semantics.

When machines transfer different content to the same relative path, the live
destination contains the most recently transferred version and history retains
the replaced version. Distinct source namespaces keep both directly visible.

Checksum comparison and SHA-256 verification read file contents even when no
transfer is needed. The in-memory source manifest grows with the number of entries.
History and partial files consume additional destination space. Verification
does not cover later changes or storage failure; durability depends on the
underlying filesystem and storage configuration.

## State and logs

```text
STATE_DIR/
  job.lock
  status.json
  sync.log
  sync.log.1 ... sync.log.5
  startup.log

DEST/
  .syncToCeph/
    archive.lock
    history/<run-id>/<previous files>
  <relative-directory>/.syncToCeph-partial/<unfinished files>
```

Status is atomically replaced and fsynced at phase boundaries. It stores up to
20 recent results, the last verified success, and cumulative logical transferred
bytes. Changing the source/destination pair resets history and counters in that
state directory. `sync.log` rotates at 5 MiB with five backups; `startup.log`
contains the most recent background launch diagnostics. Logs include file paths.

| Field | Meaning |
| --- | --- |
| `running` | The recorded controller process is alive with the same identity |
| `job_lock_held` | A process currently holds the state directory's lock |
| `last_run.transferred_bytes` | Rsync's logical size of transferred files, not network traffic |
| `last_run.verified_bytes` | Source content checked with SHA-256 during a successful verification |
| `last_run.verified` | Transfer and all verification checks completed successfully |
| `last_success_at` | Last verified completion; may precede the latest failed or dry run |

Interrupted rsync runs can lack final statistics, leaving transferred bytes at
zero despite partial work. Dry-run byte counts are estimates and do not contribute
to cumulative totals. Failed verification leaves verified counters at zero.
Dry runs do not take the destination lock, so previews may reflect concurrent
archive activity. Monitoring commands do not create a missing state directory.

## Shutdown and recovery

`stop`, `SIGINT`, and `SIGTERM` request controller shutdown. During a transfer,
the controller sends `SIGINT` to rsync's process group to allow cleanup and partial
file retention. After 30 seconds it sends `SIGKILL` if the group has not finished.
Verification checks cancellation between read chunks. Interrupted runs are unverified.
A blocked filesystem operation can delay shutdown; `stop --timeout` limits how
long the caller waits, not how long the kernel takes to release blocked I/O.

`stop` checks PID, process start time, and boot identity through a pidfd before
signalling. This requires [Linux pidfd support](https://docs.python.org/3.11/library/os.html#os.pidfd_open)
and Python's [signal API](https://docs.python.org/3.11/library/signal.html#signal.pidfd_send_signal).
Rsync inherits lock descriptors so locks can remain held after a controller crash.

| Status | Action |
| --- | --- |
| Failed run | Read `last_run.error` and logs; correct the cause, then rerun or wait for the next scheduled attempt |
| `stale` | The recorded controller is gone and the job lock is free; a new controller can start |
| `orphaned` | The controller is gone but a process still holds the lock; inspect remaining processes and let active I/O finish |
| Missing mount | Restore the configured mount; each subsequent run checks it again |

Lock files remain after shutdown; their existence does not mean the lock is held.
Deleting an active lock file breaks coordination. Retained partial files are
available to rsync on the next attempt.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Verified real run, successful dry run/control command, or requested scheduler shutdown |
| `1` | Configuration, validation, verification, or other operational failure |
| `2` | CLI syntax error; rsync can also return this code for a protocol error |
| `130` | Interrupted active run |
| Other positive rsync codes | Propagated for failed one-off transfers and saved in scheduled results |

`last_run.rsync_exit_code` records rsync's original result when available, including
codes `23` (partial transfer due to error) and `24` (vanished source files). Both
leave the run unverified. Scheduled controllers continue after transfer errors; process
liveness alone does not establish a successful transfer.

## Architecture

| Module | Responsibility |
| --- | --- |
| `cli.py` | Arguments, configuration precedence, detachment, status, logs, stop |
| `config.py` | TOML validation, defaults, path separation, schedule parsing |
| `engine.py` | Source manifest, rsync subprocess, cancellation, SHA-256 verification |
| `scheduler.py` | Controller lifecycle, signals, interval and daily schedules |
| `state.py` | Locks, atomic JSON, rotating logs, Linux process identity |
