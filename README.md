# syncToCeph

Copy files to a locally mounted Ceph archive using rsync. Files removed from the
source remain in the destination. Replaced destination files are retained in
version history. Completed transfers are checked with SHA-256.

Start with [viewing and changing directory defaults](#view-and-change-directory-defaults)
to choose your source, archive, and state/log locations.

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
./syncToCeph config show
./syncToCeph run --dry-run
./syncToCeph run
./syncToCeph status
```

The launcher uses `python3` from `PATH`. You can also run
`PYTHONPATH=src python3 -m synctoceph` from the project directory, or select an installed
interpreter explicitly, for example `python3.11 ./syncToCeph --help`.
Neither pip nor a virtual environment is required for direct execution.
The package lives in `src/synctoceph` to avoid a name collision with the
`syncToCeph` launcher on case-insensitive filesystems.

## View and change directory defaults

From the project directory, inspect the settings that the next command will use:

```bash
./syncToCeph config show
```

This shows the loaded config file, absolute paths, whether each setting comes
from a flag, the config file, an environment variable, or a built-in default,
and derived log/history locations. It does not start a transfer, create data or
state directories, or require the source and destination to be mounted.
Use `config show --json` for machine-readable output.

Without a config file or overrides, the defaults are:

| Setting | Default | How to change it |
| --- | --- | --- |
| `source` | `/mnt/d/luminoseData` | `config set source /your/source` |
| `dest` | `/mnt/ceph/LuminoseFM/LuminoseDataCeph` | `config set dest /your/archive` |
| `state_dir` | `.synctoceph-state` in the current working directory | `config set state_dir /your/state` |
| `require_mount` | `/mnt/ceph` for the built-in destination; none for custom destinations | `config set require_mount /your/mount` |

Create your local defaults file once, then change the paths. Replace the example
paths below with your own; quote paths containing spaces:

```bash
./syncToCeph config init
./syncToCeph config set source "/data/my source"
./syncToCeph config set dest /data/archive
./syncToCeph config set state_dir /home/your-user/synctoceph-state
./syncToCeph config show
./syncToCeph run --dry-run
```

`config init` creates **`syncToCeph.toml` in your current working directory** and
refuses to overwrite an existing file. Every command automatically loads that file
when present. You can also open it in any text editor. There is no need to edit
Python source. The repository ignores this local settings file.

To set all directories at creation time:

```bash
./syncToCeph config init --source /data/source --dest /data/archive --state-dir /home/your-user/synctoceph-state
```

`config init` and directory changes through `config set` save absolute paths.
`config set` preserves the other settings but rewrites TOML formatting and comments.
These commands save settings without moving files or creating source, destination,
or state directories. Restart a running scheduler for changes to take effect.
If changing `state_dir`, stop the old scheduler using its old state directory first;
existing status and logs stay there.

Other paths follow your selected directories:

| Files | Location | How to change the location |
| --- | --- | --- |
| Status, logs, background startup log | `STATE_DIR/status.json`, `sync.log`, `startup.log` | Change `state_dir` |
| Previous file versions | `DEST/.syncToCeph/history/<run-id>/` | Follows `dest`; no separate setting |
| Unfinished transfers | `.syncToCeph-partial/` inside destination subdirectories | Follows `dest`; no separate setting |

For a one-command override that does not change saved defaults:

```bash
./syncToCeph run --source /data/source --dest /data/archive --dry-run
```

Both directories must already exist. For the default destination, `/mnt/ceph`
must be mounted. For a custom destination, add `--require-mount /mount/path`
when a mount check is needed. This option checks that the path is a mount point
containing the destination; it does not check the filesystem type.
To save a mount check, use `config set require_mount /mount/path`. An empty value
(`config set require_mount ""`) restores automatic behavior: `/mnt/ceph` for the
built-in destination, no guard for other destinations.

### Run from another directory

Use the launcher's absolute path and your config file's absolute path. Replace
`/path/to/syncToCeph` with your project directory:

```bash
/path/to/syncToCeph/syncToCeph --config /path/to/syncToCeph/syncToCeph.toml run
/path/to/syncToCeph/syncToCeph --config /path/to/syncToCeph/syncToCeph.toml status
```

Use the same state directory for `run`, `schedule`, `status`, `logs`, and `stop`.
The executable's location does not determine the default state directory.
Automatic config discovery also uses the working directory, not the executable's
directory. Use `--config` when launching from elsewhere.
An absolute `SYNCTOCEPH_STATE_DIR` environment variable can replace repeated
`--state-dir` options. Uninstalled `python3 -m synctoceph` requires the project
on Python's import path; set `PYTHONPATH` to the project's `src` directory.

## Optional virtual environment installation

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
| `config show` | `--json`, `--source`, `--dest`, `--require-mount` | View effective defaults and derived paths without a transfer |
| `config init` | `--source`, `--dest`, `--require-mount` | Create a defaults file; refuses to overwrite |
| `config set KEY VALUE` | Keys listed in Configuration below | Save a setting in an existing file |

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

Without `--config`, commands load `./syncToCeph.toml` if it exists; otherwise they
use built-in defaults. `--config FILE` selects a different file instead of the local
file; a missing explicit file is an error. To create another profile, use
`./syncToCeph --config /path/to/profile.toml config init` (its parent must exist).
Use that same `--config` with `config show`, `config set`, and job commands.

CLI values override TOML values;
a CLI timing option replaces the configured schedule. Unknown keys and incorrect
types are errors. State-directory precedence is CLI, TOML,
`SYNCTOCEPH_STATE_DIR`, then the default.

Supported keys are `source`, `dest`, `state_dir`, `require_mount`, `interval`,
`at`, and `dry_run`. For example:

```bash
./syncToCeph config set interval 4h
./syncToCeph config set dry_run true
./syncToCeph config show
```

Setting `interval` removes a saved `at`, and setting `at` removes a saved `interval`.
The `dry_run` value accepts `true` or `false`.

All relative paths, including those inside TOML files, resolve from the working
directory. Absolute paths give consistent behavior across terminals and services.
To disable a configured `dry_run = true`, use `config set dry_run false` or edit
the setting; there is no `--no-dry-run` option.

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
