# Agent guide: syncToCeph

## Scope and invariants

This is a Python 3.11+ standard-library CLI for Linux/WSL2. Rsync 3.2+ transfers
regular files and directories into an additive, locally mounted Ceph archive.

- Do not introduce destination deletion, source removal, in-place writes, automatic
  history pruning, or caller-supplied arbitrary rsync flags.
- Destination-only files survive all syncs. Changed same-path files keep previous
  contents in `DEST/.syncToCeph/history/<run-id>/`.
- Exit zero for a real transfer requires successful rsync, SHA-256 verification of
  all source files, and a stable source manifest. Dry runs are always unverified.
- A changing live source cannot provide snapshot guarantees. Preserve clear
  documentation of this boundary; never claim unconditional zero data loss.
- Validate the configured mount requirement each run; do not create a missing
  destination root. Custom destinations have no mount guard unless configured.
- Require separate source, destination, and state trees. Reject symlinks on touched
  paths, source special files, reserved names, and destination type conflicts.
- Coordinate through lifetime state locks and shared destination locks. Keep
  locks inherited by rsync. Do not delete lock files as a recovery mechanism.
- Use argv subprocess execution, bounded output streaming, rotating logs, atomic
  status replacement, and process identities/pidfds for safe shutdown.

## Workspace rules

The project owner explicitly forbids **all git commands** and **all writes outside
this workspace** during development. Reading outside it is allowed. Do not install
system packages, create external environments, enable services, or transfer real
data as part of a development check. Do not launch a default-source transfer.
Temporary test files belong in `.test-tmp/` inside this repository. Version control
and production deployment are operator-managed.

## Layout

```text
syncToCeph                 executable repository launcher
pyproject.toml             package metadata and installed console entry point
synctoceph/
  __main__.py              python -m synctoceph entry point
  cli.py                   arguments, detachment, monitoring, stop
  config.py                defaults, strict TOML, path and schedule validation
  engine.py                snapshot, rsync, partial recovery, SHA-256, cancellation
  scheduler.py             controller lifecycle, interval/daily scheduling, signals
  state.py                 locks, status persistence, rotating logs, PID identity
tests/test_synctoceph.py    standard-library unit/integration suite
examples/                  configuration and uninstalled systemd template
README.md                  user and deployment reference
docs/operations.md         services, state, recovery, exit codes, architecture
agent.md                   this file
claude.md                  symlink to agent.md
```

## Execution patterns

From the repository, use `./syncToCeph` or `python3 -m synctoceph`; installation
is optional. Run tests with `python3 -m unittest discover -s tests -v`.

Direct execution needs Python 3.11+ and rsync 3.2+; pip and a virtual environment
are optional. The launcher uses `python3` on PATH. From other directories, use
its absolute path and an explicit absolute state directory. Installed commands
have the same working-directory and state rules. Regular installations require
reinstallation after source changes; editable installations use the source tree.

One-off execution is `run --source DIR --dest DIR --state-dir DIR`. Scheduling
is `schedule --every 4h` or `schedule --at 02:00`, with optional `--immediate`,
`--background`, and `--dry-run`. Daily times honor `TZ` or the system timezone;
intervals wait from completion. Scheduled errors retry at the next normal run.
Monitoring commands must use the same state directory as the controller.

Default source: `/mnt/d/luminoseData`. Default destination:
`/mnt/ceph/LuminoseFM/LuminoseDataCeph`, guarded by the `/mnt/ceph` mount point.
State defaults to the working directory's `.synctoceph-state`; CLI, explicit TOML,
and `SYNCTOCEPH_STATE_DIR` can override it. Config paths resolve from the working
directory. A state directory owns one controller; a destination owns one writer.

`status.json` stores process identity, controller phase, next execution, last
verified success, up to 20 recent results, and cumulative logical transferred bytes.
`sync.log` rotates at 5 MiB with five backups. A foreground or detached controller
handles SIGINT/SIGTERM by forwarding SIGINT to rsync and recording interrupted
runs as unverified. An orphaned rsync can retain its locks after controller death.

## Changes and validation

Use meaningful real-rsync tests when modifying transfer safety, collision handling,
partial recovery, signals, or locks. Tests must use workspace fixtures and clean up
their spawned processes. Keep dry-run destination writes at zero. Ensure rsync
failures (including vanished files, exit 24) do not become verified successes.
Keep runtime dependencies at zero unless a concrete requirement justifies one.
Read README.md before changing public CLI semantics or operational guarantees.

## Documentation and comment standards

- Use concise, factual language suitable for both new and experienced users.
  Avoid promotional claims, lectures, decorative banners, emoji, and repeated warnings.
- Keep setup and common commands in README.md; keep operational detail in
  docs/operations.md. Link to details instead of duplicating explanations.
- Distinguish required runtime dependencies from optional installation/build tools.
  State exact minimum versions and supported platforms; do not equate a declared
  minimum with a tested version or make unsupported claims about future releases.
- Specify the working directory, interpreter selection, state location, and path
  placeholders where they affect an example. Explain activation only where relevant.
- Check documented flags and behavior against the implementation and --help.
  Verify external version/API claims against official documentation when updating
  them. Update related examples and this guide when behavior or usage changes.
- Use ordinary Markdown headings, lists, tables, and fenced code blocks. Use
  comments to explain intent, invariants, or non-obvious constraints, not to
  narrate obvious statements. Keep docstrings accurate and free of absolute claims
  that exceed what the function establishes.
- For documentation/comment-only changes, check links, command syntax, and changed
  examples with workspace fixtures. Add tests only for new behavior or a concrete
  regression risk; do not run real-data transfers to validate documentation.
- Keep claude.md as a symlink to agent.md so both readers use the same instructions.
