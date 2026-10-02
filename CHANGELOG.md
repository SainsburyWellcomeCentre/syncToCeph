# Changelog

All notable changes to this project are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added: leaving out hidden files and folders-only patterns (2026-10-02)

- `exclude_hidden` setting: `true` leaves out every file and folder whose
  name starts with `.` (such as `.git`, `.DS_Store`, `.Trash-1000`), as if
  `".*"` were in `exclude`. It is `false` when missing; `init` writes `true`.
- `exclude` patterns ending in `/` match folders only (`"scratch/"`).
- `docs/configuration.md` has a section "Leaving files and folders out" with
  examples, including excluding a whole animal (`"/LUMS0099"`).

### Changed: animal-first layout and new names (2026-10-02)

This changes the config file, the folders on the destination and the
`--json` output. Existing config files are refused with an explanation; run
`synctoceph init --force` on each acquisition computer.

- **Data is organised by animal.** The source holds one folder per animal,
  and each animal folder is copied into this machine's subfolder of the
  animal's folder on the destination:
  `<source>/<animal>/...` goes to `<destination>/<animal>/<subfolder>/...`.
  Several acquisition machines (for example behaviour, ephys and histology)
  can now fill the same animal folder, each in its own subfolder. Before,
  each machine wrote to its own top-level folder,
  `<archive>/<machine_name>/`, which kept one animal's data apart.
- Files directly in the source, outside any animal folder, are no longer
  copied; each run lists them as skipped ("not inside an animal folder").
- rsync runs once per animal folder that has something to copy.
- synctoceph's records moved from `<archive>/<machine_name>/.syncToCeph/` to
  `<destination>/.syncToCeph/<subfolder>/` (the lock file there is now
  called `lock`). History keeps paths as in the source, animal folder first.
- "Archive" is no longer used: the config key `archive` is now
  `destination`; `machine_name` is replaced by `subfolder`, which is
  required (no default); `check-archived` is now `check-copied`, and its
  `not-archived` status is `not-copied`; `fleet --archive` is now
  `fleet --destination`. Messages and documentation say "ceph" or
  "destination".
- `--json` outputs have `schema_version` 2, and so does the run summary:
  `machine_name` became `subfolder`, `archive` became `destination`;
  `verify --json` has `not_copied` instead of `not_archived`;
  `fleet --json` lists `subfolders` (each with `subfolder`) under
  `destination`. `status.json` has schema version 2.
- `init` asks for the source, the destination, the subfolder, the mount and
  the schedule (flags `--source`, `--destination`, `--subfolder`), and lists
  the animal folders it found in the source; `doctor` lists them too.
- `doctor` prints ceph mount commands (cifs, credentials file, `/etc/fstab`)
  when the mount is missing. An `x-systemd.automount` placeholder (`autofs`)
  counts as a network mount, so `init` suggests `require_mount` for it.
- The README describes what the tool does instead of overstating its
  guarantees: synctoceph itself never deletes data, but people with write
  access to ceph can.
- `docs/mounting.md` is rewritten for ceph: installing `cifs-utils`, creating
  the mount point, mounting by hand, the credentials file, and every option
  of the `/etc/fstab` line.

Upgrading: data already copied under `<archive>/<machine_name>/` is left
where it is; synctoceph does not move it. After the change, runs fill the
new layout, copying files again where they are not yet in
`<destination>/<animal>/<subfolder>/`. Move or remove the old folders by hand
once you have checked the new copies (`synctoceph check-copied`).

### Added

- `verbose` config setting, on by default: `run` and `schedule` list every
  file as it is copied (`[n/total] copied PATH SIZE`), verified and deferred.
  `--verbose=false` hides the list for one run; `-v` shows it.
- Clearer terminal output while a run works: a header with the profile,
  source and destination; one `==>` line per step with a summary after the
  scan and the plan; the RESULT verdict coloured by outcome.
- `--all-profiles` for `run`, `status`, `doctor` and `service install`, to
  work with several config files (profiles) at once. `run --all-profiles`
  ends with a summary and exits with the worst result.
  `service uninstall --all-profiles` is the new name for `--all` (which
  still works).
- `init` and `doctor` note when two profiles copy into the same subfolder.
- `docs/README.md`, an index of the documentation; every page now links to
  the README, the index, and the previous and next page.
- README section "Update an existing installation".

- Rewrite in Go as a single program, `synctoceph`, installed with
  `install.sh`, updated with `update.sh` and removed with `uninstall.sh`.
- Commands: `init`, `doctor`, `run`, `schedule`, `status`, `logs`, `stop`,
  `verify`, `check-copied`, `history list`, `history restore`, `fleet`,
  `service install|uninstall|status`, `completion`, `version`.
- Records on the destination in `.syncToCeph/<subfolder>/`: verified-file
  record, history of replaced files, and run summaries read by `fleet`.
- Files already on the destination are skipped by default; `--existing replace`
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

- Clearer flag help: value placeholders such as `--profile NAME`,
  `--source DIR` and `--timeout SECONDS` instead of `string` and `int`;
  `--profile` says where the profile's settings are, so `(default "default")`
  reads as the profile named "default"; `init` shows the defaults for
  `--schedule` and `--require-mount`; `run` help mentions the `existing`
  config setting as well as `--existing replace`.
- `-v` no longer prints rsync's raw `>f+++++++++` lines; they are in the log
  (`synctoceph logs`). rsync warnings and errors are still shown.
- `./uninstall.sh --purge` lists everything it will delete and asks for
  confirmation (`--yes` skips the question), refuses while a run is still
  active, and also removes `~/.local/share/synctoceph/`.

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
