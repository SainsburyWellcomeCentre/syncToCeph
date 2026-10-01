# Changelog

All notable changes to this project are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Rewrite in Go as a single program, `synctoceph`, installed with
  `install.sh`, updated with `update.sh` and removed with `uninstall.sh`.
- Commands: `init`, `doctor`, `run`, `schedule`, `status`, `logs`, `stop`,
  `verify`, `check-archived`, `history list`, `history restore`, `fleet`,
  `service install|uninstall|status`, `completion`, `version`.
- One folder per machine in the archive (`<archive>/<machine_name>/`), with
  per-machine metadata in `.syncToCeph/`: verified-file record, history of
  replaced files, and run summaries read by `fleet`.
- Files already in the archive are skipped by default; `--existing replace`
  replaces them and keeps the old version in history.
- Only files copied in the run are verified by default; `--verify all` and
  `synctoceph verify` re-check everything.
- Files that are still changing are deferred per file with an explanation
  instead of failing the run; deferrals older than 24 hours are escalated.
- Exclude patterns, profiles (`--profile`), `--json` output with a schema
  version on every read command, and plain-text markers with optional colour.
- Automatic runs with a systemd user service, or Windows Task Scheduler on WSL
  without systemd.
- Scenario tests (`testdata/script/`), `scripts/check.sh`, and GitHub Actions.
- Documentation in `docs/`; the command reference in `docs/cli/` is generated.

### Changed

- Config and state follow XDG locations and no longer depend on the current
  folder. There are no built-in data folders; `synctoceph init` is required.
- The state folder must be on a local Linux disk, with a clear error otherwise.
- rsync 3.2.4 or newer is required (for `--fsync`).
- `--immediate` no longer uses up that day's daily run.
- Existing files are compared by size and time instead of re-reading all data
  with `--checksum` on every run.

### Removed

- The Python implementation (`src/`, `tests/`, the `syncToCeph` launcher,
  `pyproject.toml`, `examples/`).
