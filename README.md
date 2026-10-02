# synctoceph

Copies data from lab acquisition computers to the lab's ceph storage, with
rsync, and checks every copy with SHA-256.

What it does:

- **Copies new data** from an acquisition computer to ceph (or any other
  mounted network drive), using rsync. Files already on ceph are skipped.
- **Runs once or in the background.** Run it by hand, or let it run on a
  schedule (for example every 4 hours) so transfers happen automatically.
- **Organises data by animal.** The source holds one folder per animal. Each
  animal folder is copied into that animal's folder on ceph, inside a
  subfolder named for this kind of data:

  ```
  /mnt/d/luminoseData/LUMS0014/...  ->  /mnt/ceph/<project>/LUMS0014/behaviour/...
  ```

  Other acquisition computers (for example an ephys rig or a histology
  scope) fill other subfolders of the same animal folder
  (`LUMS0014/ephys/`, `LUMS0014/histology/`).
- **Checks every copy.** A file counts as copied only once the SHA-256 of the
  source and of the copy on ceph match.
- **Leaves files that are still being written** for a later run, with a clear
  message.
- **Only adds files.** synctoceph itself never deletes or moves anything, on
  ceph or on the acquisition computer; a replaced file is kept in a history
  folder. (Anyone with write access to ceph can still delete files there.)

Runs on Linux and on Windows through WSL2.

## Install

```
git clone https://github.com/SainsburyWellcomeCentre/syncToCeph.git
cd syncToCeph
./install.sh
```

Needs rsync 3.2.4 or newer, and ceph mounted on the computer (see
[Mounting ceph](docs/mounting.md)). Go is downloaded for the build if needed.

## Quickstart

```
synctoceph init                # source, destination on ceph, subfolder, schedule
synctoceph doctor              # check the setup
synctoceph run --dry-run       # see what would be copied
synctoceph run                 # copy and verify (each file is listed)
synctoceph service install     # run automatically on the schedule
```

Several kinds of data on one computer (another source, or another
subfolder)? Give each its own profile: `synctoceph --profile NAME init`, then
`synctoceph run --all-profiles` and `synctoceph status --all-profiles`. See
[Configuration](docs/configuration.md#several-jobs-on-one-computer-profiles).

Before deleting data from an acquisition computer:
`synctoceph check-copied PATH`.

## Update an existing installation

When a new version is published, update from the folder you cloned:

```
cd syncToCeph            # the folder you ran ./install.sh from
./update.sh              # downloads the new version, rebuilds, restarts services
synctoceph version       # shows the version now installed
```

Your configuration, logs, automatic runs and the data on ceph are kept. A
copy in progress is stopped gracefully and resumes on the next run.

- **Lost the cloned folder?** Clone it again (see Install) and run
  `./install.sh`; your settings are kept.
- **Changed the code yourself?** Run `./install.sh` again to rebuild from your
  folder as it is.
- **Remove it:** `./uninstall.sh` keeps your settings and logs;
  `./uninstall.sh --purge` deletes them too (it lists them and asks first).
  Neither touches ceph or the source.

Details: [Installation](docs/installation.md#update-to-a-new-version).

## Documentation

**[All documentation](docs/README.md)**:
[Installation](docs/installation.md) ·
[Configuration](docs/configuration.md) ·
[Mounting ceph](docs/mounting.md) ·
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
