# synctoceph

Copies data from lab acquisition computers into the lab's archive, and checks
every copy with SHA-256.

Each computer writes into its own folder on an already-mounted archive share,
`<archive>/<machine_name>/`. Copying is done by rsync, on demand or on a
schedule. Runs on Linux and on Windows through WSL2.

- **Nothing is ever deleted**, from the archive or from the source. Replaced
  files are kept in a history folder.
- **"Archived" means verified**: a file counts only once the SHA-256 of the
  source and of the archive copy match.
- **Files still being written are left for a later run**, with a clear message,
  and never copied half-finished.

## Install

```
git clone https://github.com/SainsburyWellcomeCentre/syncToCeph.git
cd syncToCeph
./install.sh
```

Needs rsync 3.2.4 or newer. Go is downloaded for the build if needed.

## Quickstart

```
synctoceph init                # machine name, source, archive, schedule
synctoceph doctor              # check the setup
synctoceph run --dry-run       # see what would be copied
synctoceph run                 # copy and verify (each file is listed)
synctoceph service install     # run automatically on the schedule
```

Several folders to copy (another source, or another archive)? Give each its
own profile: `synctoceph --profile NAME init`, then
`synctoceph run --all-profiles` and `synctoceph status --all-profiles`. See
[Configuration](docs/configuration.md#several-jobs-on-one-computer-profiles).

Before deleting data from an acquisition computer:
`synctoceph check-archived PATH`.

## Update an existing installation

When a new version is published, update from the folder you cloned:

```
cd syncToCeph            # the folder you ran ./install.sh from
./update.sh              # downloads the new version, rebuilds, restarts services
synctoceph version       # shows the version now installed
```

Your configuration, logs, automatic runs and the archive are kept. A copy in
progress is stopped gracefully and resumes on the next run.

- **Lost the cloned folder?** Clone it again (see Install) and run
  `./install.sh`; your settings are kept.
- **Changed the code yourself?** Run `./install.sh` again to rebuild from your
  folder as it is.
- **Remove it:** `./uninstall.sh` keeps your settings and logs;
  `./uninstall.sh --purge` deletes them too (it lists them and asks first).
  Neither touches the archive or the source.

Details: [Installation](docs/installation.md#update-to-a-new-version).

## Documentation

**[All documentation](docs/README.md)**:
[Installation](docs/installation.md) ·
[Configuration](docs/configuration.md) ·
[Mounting the share](docs/mounting.md) ·
[Scheduling](docs/scheduling.md) ·
[Operations](docs/operations.md) ·
[Troubleshooting](docs/troubleshooting.md) ·
[Safety model](docs/safety-model.md) ·
[Design](docs/design.md) ·
[Command reference](docs/cli/synctoceph.md) ·
[Changelog](CHANGELOG.md)

## Changing the tool

- Read [AGENTS.md](AGENTS.md), the developer guide for people and AI agents.
- Work in the code directly or through an AI coding agent; no Go knowledge is
  assumed.
- `./scripts/check.sh` must pass.
- The safety invariants can never be weakened.
- Review any `SAFETY:` lines in a change before running `./install.sh`.
