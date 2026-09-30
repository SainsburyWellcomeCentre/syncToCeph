# syncToCeph

Copy files to a locally mounted Ceph archive using rsync. Files removed from the
source remain in the destination. Replaced destination files are retained in
version history. Completed transfers are checked with SHA-256.

## Requirements

| Component | Requirement |
| --- | --- |
| Python | **3.11 or newer** within Python 3; no third-party Python runtime packages |
| rsync | **3.2 or newer**, available on `PATH` |
| Operating system | Linux; on Windows, run inside WSL2 |
| Process control | Linux kernel 5.3+ and Python's `os.pidfd_open` / `signal.pidfd_send_signal` for `stop` |
| Storage | Existing source and writable destination directories; locally mounted Ceph |

Python 3.10 and earlier are unsupported: configuration uses
[`tomllib`, added in Python 3.11](https://docs.python.org/3.11/library/tomllib.html).
Python alone is insufficient; rsync is a separate executable. Native Windows and
macOS are unsupported by the current Linux process-management implementation.

Check your installed versions:

```bash
python3 --version
rsync --version
```

## Run without installation

From the project directory:

```bash
./syncToCeph --help
./syncToCeph run --dry-run
./syncToCeph run
./syncToCeph status
```

The launcher uses `python3` from `PATH`. You can also run
`python3 -m synctoceph` from the project directory, or select an installed
interpreter explicitly, for example `python3.11 ./syncToCeph --help`.
Neither pip nor a virtual environment is required for direct execution.

Defaults:

| Setting | Path |
| --- | --- |
| Source | `/mnt/d/luminoseData` |
| Destination | `/mnt/ceph/LuminoseFM/LuminoseDataCeph` |
| State and logs | `.synctoceph-state` in the current working directory |

Override the data paths when needed:

```bash
./syncToCeph run --source /data/source --dest /data/archive --dry-run
```

Both directories must already exist. For the default destination, `/mnt/ceph`
must be mounted. For a custom destination, add `--require-mount /mount/path`
when a mount check is needed. This option checks that the path is a mount point
containing the destination; it does not check the filesystem type.

### Run from another directory

Use the launcher's absolute path and a fixed state directory. Replace
`/path/to/syncToCeph` with your project directory:

```bash
/path/to/syncToCeph/syncToCeph run --state-dir /path/to/syncToCeph/.synctoceph-state
/path/to/syncToCeph/syncToCeph status --state-dir /path/to/syncToCeph/.synctoceph-state
```

Use the same state directory for `run`, `schedule`, `status`, `logs`, and `stop`.
The executable's location does not determine the default state directory.
An absolute `SYNCTOCEPH_STATE_DIR` environment variable can replace repeated
`--state-dir` options. Uninstalled `python3 -m synctoceph` requires the project
on Python's import path; running from the project directory provides that.

### Optional virtual environment installation

From the project directory:

```bash
python3 -m venv .venv
.venv/bin/python -m pip install --no-cache-dir .
.venv/bin/syncToCeph --help
```

This requires `venv` and pip support in your Python installation. Pip may download
build dependencies. It installs the application into `.venv`; rsync remains a
separate system dependency.

Activation is optional. In Bash or Zsh, it makes the installed command available
without its path:

```bash
source .venv/bin/activate
syncToCeph --help
```

Without activation, use `/path/to/syncToCeph/.venv/bin/syncToCeph` from any
directory. State-directory rules are the same as for direct execution.
After changing the source, repeat the install command to update a regular
installation. For development, use
`.venv/bin/python -m pip install --no-cache-dir --editable .` to run from the
source tree. See Python's [venv documentation](https://docs.python.org/3.11/library/venv.html)
and pip's [local installation reference](https://pip.pypa.io/en/stable/topics/local-project-installs/).

## Commands and scheduling

`--config FILE` and `--state-dir DIR` work before or after every subcommand.

| Command | Options | Purpose |
| --- | --- | --- |
| `run` | `--source`, `--dest`, `--require-mount`, `--dry-run` | Transfer once and verify |
| `schedule` | All `run` options, plus those below | Run periodically |
| `status` | `--json` | Show PID, phase, last run, byte counts, recent exit codes, next run |
| `logs` | `--lines N` / `-n N` (default 100) | Show recent logs |
| `stop` | `--timeout SECONDS` (default 45) | Request shutdown and wait |

Choose one schedule:

```bash
./syncToCeph schedule --every 4h --immediate
TZ=Europe/London ./syncToCeph schedule --at 02:00 --background
```

These are alternative commands for the same job; only one controller can use a
state directory at a time.

- `--interval` and `--every` are aliases. Values are positive integers followed
  by `s`, `m`, `h`, or `d`. Each interval starts after the preceding run finishes.
- `--at HH:MM` uses the system timezone, or the IANA timezone supplied through `TZ`.
- `--immediate` adds a run at startup. Otherwise, the first run waits for its schedule.
- `--background` detaches the scheduler. Without it, scheduling runs in the foreground.

Runs do not overlap. Transfer failures are recorded and retried at the next
scheduled occurrence. Missed runs are not replayed; restarting resets an interval
countdown. Daily times skipped by daylight-saving changes are skipped for that
date; repeated daily times run once, with the last attempt persisted across
restarts. `--immediate` explicitly requests a startup run regardless of that history.

Monitor or stop the job from the same directory, or supply its `--state-dir`:

```bash
./syncToCeph status --json
./syncToCeph logs -n 50
./syncToCeph stop
```

For services, cron, and restart behavior, see [operations](docs/operations.md#services-and-cron).

## Configuration

[examples/syncToCeph.toml](examples/syncToCeph.toml) contains the supported settings:

```bash
./syncToCeph --config examples/syncToCeph.toml run --dry-run
./syncToCeph --config examples/syncToCeph.toml schedule --background
./syncToCeph --config examples/syncToCeph.toml status
```

Configuration is loaded only with `--config`. CLI values override TOML values;
a CLI timing option replaces the configured schedule. Unknown keys and incorrect
types are errors. State-directory precedence is CLI, TOML,
`SYNCTOCEPH_STATE_DIR`, then the default.

All relative paths, including those inside TOML files, resolve from the working
directory. Absolute paths give consistent behavior across terminals and services.
To disable a configured `dry_run = true`, edit the setting; there is no
`--no-dry-run` option.

## Transfer behavior

- Destination-only files remain. Previous versions of replaced files are stored
  under `DEST/.syncToCeph/history/<run-id>/<relative-path>`, without automatic pruning.
- Rsync uses checksum comparison and resumable partial files. A real run succeeds
  only after independent SHA-256 checks of every source file and its destination
  copy, plus a source manifest recheck. Detected source changes fail the run.
- Dry runs preview transfers and write local status/logs, but write nothing to the
  destination and do not verify data.
- Regular files and directories are supported. Symlinks on source or touched
  destination paths, special files, and file/directory type conflicts are rejected.
- Source, destination, and state directories must be separate and non-nested.
  `.syncToCeph` and `.syncToCeph-partial` are reserved source entry names.
- File modification times are preserved. Ownership, ACLs, extended attributes,
  hard-link relationships, and exact permissions are not replicated.

Before removing source files, check that the relevant run has
`last_run.exit_code: 0` and `last_run.verified: true` in `status --json`.
Verification applies to data read during that run. Stable source files or a
filesystem snapshot are needed for consistent results; the tool does not create
snapshots or provide a whole-tree transaction. Failed runs may leave completed
files alongside unfinished transfers.

[Operations](docs/operations.md) covers locking, recovery, exit codes, storage
requirements, and the module architecture.

## Tests

From the project directory, with Python and rsync available:

```bash
python3 -m unittest discover -s tests -v
```

The suite uses fixtures under `.test-tmp/` and does not access the default data
paths. It covers archive retention, verification, scheduling, locking, shutdown,
and real-rsync interruption and resumption.
