# AGENTS.md: synctoceph developer guide

This guide is for everyone who develops `synctoceph`: people working directly in the
code and AI coding agents (Claude Code, Codex, Cursor, and others) working on their
behalf. The same decisions, design, safety invariants and conventions apply to both.
It is named `AGENTS.md` so that agents load it automatically; `CLAUDE.md` imports it,
and the README links to it for human developers.

## 1. What this project is

`synctoceph` copies data from lab acquisition machines to the lab's ceph storage (or
another mounted network drive) with rsync, once or on a schedule. It never deletes
anything itself. Every copied file is checked with SHA-256 before it is reported as
copied. Data is organised by animal: the source holds one folder per animal, and each
animal folder goes to `<destination>/<animal>/<subfolder>/`, where each acquisition
machine (profile) has its own `subfolder` (for example `behaviour`, `ephys`,
`histology`). ceph is already mounted on each machine (cifs through `/etc/fstab`).

**Wording:** say "ceph" (or "mounted network drive") and "destination", never
"archive", in messages, help and docs.

**Status:** the Go implementation of phases 1 to 5 is complete (section 10). The
earlier Python implementation has been removed; it remains in the git history, and
the lessons from it are in section 11. What is left before `v1.0.0`: the owner
confirms the decisions listed in [docs/design.md](docs/design.md#decisions-taken-while-building-for-the-owner-to-confirm),
and the real-WSL checks in [docs/scheduling.md](docs/scheduling.md) are done.

**Getting started as a developer:** you need Go of the version in `go.mod`. The
simplest way is to run `./install.sh` once (with a temporary `HOME` if you like, see
section 2); it keeps that Go in `~/.cache/synctoceph/go<version>/`. Put its `bin/`
folder on your `PATH`, then:

```
go build ./...              # compile everything
go test ./...               # unit and scenario tests (real rsync, temporary folders)
./scripts/check.sh          # everything CI checks; must pass before a change is done
./scripts/gen-docs.sh       # after changing commands, flags or help text
```

## 2. Working on this project

### Who develops it

The people who maintain and extend `synctoceph` are lab members, typically systems
neuroscientists. **They are not expected to know Go, so most development will happen
through AI coding agents.** Some developers will also work in the code directly.
Either way:

- **Explain changes in plain language.** Every change comes with a summary saying
  what changed, why, how it was tested, and any risk. It can be an agent's reply or a
  human's change description. Avoid Go jargon, or explain it briefly when it matters.
- **Keep the code approachable** (section 7), so the next developer, human or agent,
  can follow it.
- **Safety invariants are never weakened** (section 4). This applies to everyone.

### Version control

- **Human developers** manage git themselves. The project owner handles the main
  repository.
- **AI agents** don't run git commands as part of routine work. If a task genuinely
  needs git, the agent **asks the person it is working with for permission first and
  says why.** Examples are working in a separate worktree, or reading history to track
  down a regression.

### Keeping development sandboxed

This tool moves irreplaceable research data. Development and testing never touch the
real setup:

- **Tests use temporary directories only.** Use `t.TempDir()` or `testscript` work
  directories, and set `HOME`, `XDG_CONFIG_HOME` and `XDG_STATE_HOME` to temporary
  paths. Go's own build and module caches are fine.
- **Leave real data and real configuration alone during development.** Don't:
  - read or write real acquisition folders or ceph (e.g. `/mnt/z`, `/mnt/ceph`,
    `/mnt/d`);
  - start real transfers;
  - install or enable services;
  - install system packages;
  - run `install.sh`, `update.sh` or `uninstall.sh` against the real home directory.

  Trying the tool on a real machine is a deliberate deployment step, done by a person,
  never a side effect of development. Agents do it only when the person they work with
  explicitly asks.

### Check with the project owner before

- adding a Go dependency;
- changing the command-line interface, config keys, `--json` output, exit codes or
  on-disk layout (all phases that introduced them are complete).

## 3. Decisions already made

These were agreed with the owner on 2026-09-30. Treat them as settled unless the
owner raises them again.

| Topic | Decision |
|---|---|
| Language | Go, a single static binary (`CGO_ENABLED=0`) |
| Transfer engine | **rsync 3.2+**, because it is tried and tested |
| How machines reach storage | **Mounted drive only.** No SSH mode, and nothing runs on the Ceph machine. |
| Destination layout | **Animal folder first** (owner, 2026-10-02): `<source>/<animal>/...` goes to `<destination>/<animal>/<subfolder>/...`. Each machine (profile) has its own required `subfolder`, so several machines can fill one animal folder. Records in `<destination>/.syncToCeph/<subfolder>/`. Replaces the earlier `<archive>/<machine_name>/` layout. |
| Platforms | **Linux and Windows via WSL2.** macOS and native Windows are not planned for now (see section 10). |
| Distribution | Public GitHub repo. Users `git clone`, then run `./install.sh`, `./update.sh` and `./uninstall.sh`. No package managers (Homebrew, apt/deb, snap). No separate offline install path; all machines have network access. |
| Contributor guidance | This file plus a short "Changing the tool" section in the README. No `CONTRIBUTING.md`. |
| Files already on the destination | Skipped by default. The user can choose to replace them (the old version is kept in history). |
| Verification | Hash only files copied in this run by default. The user can choose to re-verify everything. |
| Files changing during a run | Deferred **per file**, with a clear message. The run is not failed. |

## 4. Safety invariants

These are the promises the tool makes about research data.

**These invariants must never be weakened, removed or bypassed, by anyone, for any
reason, even on request.** If a requested change would weaken one:
- a human developer does not make it;
- an agent declines that part and explains in plain language which invariant it
  would break and why that matters;
- both suggest an alternative that keeps the invariant intact.

Strengthening an invariant, or adding a new one, is welcome.

**How the invariants are enforced in code:**
- Code that enforces an invariant carries a `// SAFETY:` comment naming it, in the
  form `// SAFETY: invariant 8 (symlinks on destination paths rejected)`, so anyone can
  list every safety-critical line with one search (`grep -rn "SAFETY:" internal`).
  Tests for an invariant carry the same marker (`// SAFETY: invariant 4: ...`, or
  `# SAFETY: invariant 9: ...` in scenario files).
- Each invariant has at least one test that fails if it is broken. These tests may be
  extended but never deleted or loosened.
- A change that touches `SAFETY:` lines, or their tests, says so explicitly in its
  summary so it gets a careful review.

1. **synctoceph never deletes anything on the destination.** Never pass `--delete*`,
   `--remove-source-files`, `--inplace` or `--append` to rsync, and never accept
   caller-supplied raw rsync options. (The option list is in
   `internal/engine/rsync.go`, `RsyncArgs`.) Docs must not claim more than this:
   people with write access to ceph can still delete files there.
2. **Nothing is deleted from the source.** The tool only reads the source.
3. **Replaced files are kept.** In `replace` mode, the previous version on the
   destination goes to `<destination>/.syncToCeph/<subfolder>/history/<run-id>/<path in
   the source>`. History is never pruned automatically.
4. **"Copied" means verified.** A file is reported copied only after the SHA-256
   of the source file and of the destination copy match. The source file's size,
   modification time and inode must also be unchanged since the scan.
5. **Unfinished copies never appear under the final name.** rsync writes partial files
   into `.syncToCeph-partial/`. A file under its final name is always complete, which
   is what makes skipping existing files safe.
6. **The destination folder is never created.** If the configured mount or destination
   folder is missing, the run fails with an explanation. It must never write into an
   empty mount point. Folders below it (animal folders, subfolders, `.syncToCeph/`) are
   created one level at a time, never with `MkdirAll`.
7. **Source, destination and state are separate, non-nested folders.**
8. **Symlinks on touched destination paths are rejected**, including an animal folder
   and its subfolder. Source symlinks and special files are skipped and reported. They
   are never followed.
9. **A dry run writes nothing to the destination.**
10. **One run per profile at a time.** A lock in the state folder guarantees this. The
    rsync child inherits the lock descriptor, so the lock stays held while rsync is
    still running. A second lock in `.syncToCeph/<subfolder>/` stops two profiles on
    one computer writing the same subfolder.
11. **Stopping is graceful.** `stop` or SIGINT/SIGTERM sends SIGINT to rsync's process
    group, and SIGKILL after 30 seconds. An interrupted run is never reported as
    verified.

## 5. Behaviour (specification)

The full specification is in [docs/design.md](docs/design.md): deployment, files and
locations, how a run works step by step, results and exit codes, the records kept
on the destination, scheduling and process control. The safety model is in
[docs/safety-model.md](docs/safety-model.md). In short:

- **A run:** preflight (mount, destination, symlinks, separate paths, rsync
  3.2.4+) -> scan the source (skip symlinks, special files and files outside an
  animal folder, apply excludes, defer files modified within `settle_time`) -> plan
  (new, matching, differing, conflicts, each compared with
  `<destination>/<animal>/<subfolder>/...`) -> rsync once per animal folder, with a
  fixed option list and an explicit NUL-separated file list -> SHA-256 verification
  of every copy -> run summary in the local state folder and in
  `<destination>/.syncToCeph/<subfolder>/runs/`.
- **Results:** OK (exit 0), FAILED (1), usage error (2), PARTIAL (3, files
  deferred), INTERRUPTED (130).
- **Locations:** config `~/.config/synctoceph/<profile>.toml`, state
  `~/.local/state/synctoceph/<profile>/` (must be on a local Linux disk), records on
  the destination `<destination>/.syncToCeph/<subfolder>/`. Paths in the records
  are paths in the source (animal folder first).
- **Config keys:** `source`, `destination`, `subfolder` (all required),
  `require_mount`, `existing`, `verify`, `settle_time`, `exclude`, `exclude_hidden`,
  `interval`, `at`, `dry_run`, `verbose`.
- **Leaving things out:** `exclude` patterns are matched by synctoceph during the
  scan (`excluded` in `internal/engine/scan.go`), never passed to rsync. A name
  matches anywhere, a pattern with `/` is a path from the source, a trailing `/`
  means folders only. `exclude_hidden = true` adds `.*` (in `config.Resolve`).
  `.syncToCeph` and `.syncToCeph-partial` are always skipped, whatever the
  settings. Excluded entries are never reported as safe by `check-copied`.
- **Commands:** `init`, `doctor`, `run`, `schedule`, `status`, `logs`, `stop`,
  `verify`, `check-copied`, `history list|restore`, `fleet`,
  `service install|uninstall|status`, `completion`, `version`. The reference in
  [docs/cli/](docs/cli/synctoceph.md) is generated from the code.
- **Profiles** (one config file per job) are chosen with `--profile NAME`;
  `run`, `status`, `doctor` and `service install|uninstall` accept
  `--all-profiles`. `run --all-profiles` runs them one after another and exits
  with the worst result.
- **Messages** use plain markers (`OK`, `NOTE`, `DEFERRED`, `WARNING`, `ERROR`,
  `RESULT`) and every problem says what happened, why it matters and what to do.
- **Progress** while a run works: a header, one `==>` line per step, and (with
  the `verbose` setting, on by default) one line per file copied, verified or
  deferred. rsync's raw lines go to the log only.
- **Uninstall:** `./uninstall.sh --purge` lists and confirms before deleting
  config, state, logs and the build cache; it never touches the destination.

When behaviour changes, update `docs/design.md` (and the other docs it affects) in
the same change.

## 6. Code layout

```
cmd/synctoceph/main.go         entry point only; calls internal/cli
internal/cli/                  one file per command (cobra); flag parsing and output only;
                               runview.go shows a run's progress; common.go has the
                               --all-profiles helpers; script_test.go runs the
                               scenario tests
internal/config/               profiles, XDG paths, TOML load/validate/save
internal/engine/               preflight, scan, plan, rsync runner, verify, deferral,
                               check-copied and verify (audit.go), progress events
                               (event.go)
internal/destination/          everything on the destination: the animal-first layout
                               (layout.go), manifest, history, run summaries, fleet,
                               safe destination paths; the RunSummary type
internal/scheduler/            interval/daily timing; the controller (lock, log, control
                               socket, status) used by run, schedule and verify
internal/state/                state lock, status.json, rotating logs, control socket
internal/service/              systemd user unit, Windows Task Scheduler (WSL)
internal/platform/             mount table (/proc/self/mountinfo), WSL detection,
                               atomic file writes
internal/ui/                   all user-facing text (messages*.go, help.go), formatting,
                               colour, the Problem error type
internal/tools/gendocs/        generates docs/cli/ and completions/ (used by scripts)
internal/tools/mdlinks/        Markdown link check (used by check.sh)
testdata/script/*.txtar        scenario tests (section 8)
completions/                   generated bash and zsh completion scripts
scripts/check.sh               the one command that checks everything
scripts/gen-docs.sh            regenerates docs/cli/ and completions/ from the command definitions
.github/workflows/check.yml    CI: runs scripts/check.sh on Ubuntu
install.sh  update.sh  uninstall.sh
docs/                          see section 9; docs/README.md is the index
```

Dependency direction (a package only imports packages to its left): `platform` <-
`destination` <- `state` <- `ui` <- `config` <- `engine` <- `scheduler` <- `cli`;
`service` uses only `config` and `platform`.

**Dependencies** (anything beyond these needs the owner's approval):
- `github.com/spf13/cobra` for commands, completions and generated docs.
- `github.com/BurntSushi/toml` for the config file.
- `golang.org/x/sys/unix` for flock and signals.
- `github.com/rogpeppe/go-internal/testscript`, in tests only.
- Pin the latest stable Go release at project start in `go.mod`, and pin the same
  version in `install.sh`. (Now Go 1.27.1. To change it: update `go.mod`, and
  `GO_VERSION` and both `GO_SHA256_*` values in `install.sh`, from
  https://go.dev/dl/?mode=json.)
- Tools run by `scripts/check.sh` (staticcheck, goimports, shellcheck) are pinned
  there and are not module dependencies.

## 7. Conventions for readable Go

The next person to read this code is likely a scientist working with an agent, not a
Go developer. Write for them.

- **Explain every file in plain English.** Each `.go` file starts with a comment for
  someone who doesn't know Go: what the file is for and how it fits in.
- **Document every exported identifier.** Comments explain *why* and what is
  guaranteed, not what the syntax does.
- **Keep it small and simple.** Files stay under about 300 lines and functions do
  one thing.
  - Prefer plain structs and functions.
  - Avoid generics, interfaces with a single implementation, and clever abstractions.
  - Use goroutines only where needed: streaming rsync output and handling signals.
- **Handle errors where they happen.** Wrap each error with context
  (`fmt.Errorf("reading config %s: %w", path, err)`).
- **Give the user a fix.** User-facing errors are `ui.Problem` values (what
  happened, why it matters, what to do), made by functions in `internal/ui`.
- **Keep wording in `internal/ui`.** Messages are grouped by topic in
  `messages.go` (setup problems), `messages_destination.go`, `messages_run.go` (the run
  report), `messages_progress.go` (what a run shows while it works),
  `messages_profiles.go` (several profiles), `messages_status.go`,
  `messages_setup.go`, `messages_fleet.go`, and command help in `help.go`.
  Terminal styling (markers, `==>` steps, colour) is in `output.go`.
- **Name the magic numbers.** Use constants such as the 30 s SIGKILL grace, the log
  rotation size, the settle time default and the number of recent runs kept, each
  with a comment.
- **Machine-readable output is a contract.** Types written to JSON (status, run
  summary, manifest line) include a `schema_version`, and changing them requires
  updating the docs.
- **Stick to portable APIs where it costs nothing.** For example, use a lock plus a
  socket for process control, not `/proc` parsing. This keeps a future macOS port
  cheap. Linux-specific code (such as reading `/proc/self/mountinfo`) goes in
  clearly named files.
- **Format and lint.** Run `gofmt` and `goimports`; `staticcheck` and `go vet` must be
  clean.

## 8. Testing

- **Scenario tests** (`testdata/script/*.txtar`, run by `testscript`) are the main
  behavioural spec. They read like terminal sessions, so non-Go users can follow and
  request them:
  ```
  # Files that already exist on ceph are skipped by default
  exec synctoceph run
  stdout 'Copied and verified 2 files'
  cp new-version.txt src/LUMS0001/a.txt
  exec synctoceph run
  stdout '1 file already on ceph differs from the source and was NOT replaced'
  cmp ceph/LUMS0001/behaviour/a.txt original-a.txt
  ```
- **Unit tests** are table-driven, for parsing, planning, scheduling (including
  daylight-saving cases) and formatting.
- **Real rsync is used in tests.** A fake rsync on `PATH` is allowed only for signal
  and timeout tests (`stop_signal.txtar`, `lock_inherited.txtar`). Service tests
  use stand-in `systemctl`/`loginctl` programs so no real service is touched, and
  the scenario `PATH` excludes Windows programs such as `schtasks.exe`.
- **Install scripts** (`install.sh`, `update.sh`, `uninstall.sh`) have no
  automated tests. Try them by hand with `HOME` set to a temporary folder,
  `SYNCTOCEPH_VERSION`/`SYNCTOCEPH_COMMIT` set (so no git command runs), and a
  `PATH` of only Go's `bin/`, `/usr/bin` and `/bin`, so Windows programs such as
  `schtasks.exe` cannot change real scheduled tasks.
- **Scenario helpers.** Besides testscript's own commands, scenarios can use
  `config key=value...`, `age DURATION PATH...`, `waitfor PATH`,
  `waitlog [N] REGEXP`, `snapshot DIR FILE`, `killpid FILE` and `mkfifo PATH`
  (documented in `internal/cli/script_test.go`). `config` defaults to
  `source = $WORK/src`, `destination = $WORK/ceph` and `subfolder = "behaviour"`, so
  put source files in an animal folder: `src/LUMS0001/a.txt` is copied to
  `ceph/LUMS0001/behaviour/a.txt`. File names in a txtar archive are expanded like
  arguments, so create names containing `$` with `cp` and single quotes.
- **Every SAFETY invariant and every behaviour in section 5 has a test.** Port the
  Python tests' intent (section 11).
- **Tests clean up every process they start.** Stand-in rsync programs write
  their process ID to `$WORK/rsync-pid`; the test setup kills that process group
  when a scenario ends, even if it failed.
- **`./scripts/check.sh` runs** (it prints `RESULT OK` or the list of failed steps):
  - `gofmt` / `goimports` check
  - `go vet`
  - `staticcheck`
  - `go test -race ./...`
  - `shellcheck` on all scripts
  - a check that `docs/cli/` and the completions are up to date
  - a Markdown link check

  It must pass before a change is considered done. GitHub Actions runs the same
  script on Ubuntu.

## 9. Documentation

- **`README.md`** stays short (about two screens):
  - what the tool does, in a few plain bullets: copies with rsync from the
    acquisition computer to ceph, once or on a schedule, organised by animal, every
    copy checked with SHA-256, only adds files. No overstated guarantees (people
    with write access to ceph can delete files there);
  - install (`git clone` then `./install.sh`);
  - a quickstart (`init`, then `doctor`, then `run --dry-run`, then `run`, then
    `service install`), and a pointer to profiles for several jobs;
  - "Update an existing installation": `./update.sh`, what is kept, what to do
    without the cloned folder or after changing the code, and `uninstall.sh`
    with and without `--purge`;
  - links to the docs, starting with the index `docs/README.md`;
  - a short "Changing the tool" section. It says:
    - read `AGENTS.md`, the developer guide for people and agents;
    - work directly or through an AI agent;
    - `./scripts/check.sh` must pass;
    - safety invariants can never be weakened;
    - review any `SAFETY:` lines in the change before running `./install.sh`.
- **`docs/` contains:**
  - `README.md`, the index: every page with one line on what it covers, in
    reading order
  - `design.md` (from section 5)
  - `installation.md` (install, update to a new version, reinstall, uninstall,
    purge)
  - `configuration.md` (including how data is organised by animal)
  - `cli/` (generated; never edit by hand)
  - `scheduling.md` (systemd, WSL, Task Scheduler)
  - `mounting.md` (ceph with cifs: `cifs-utils`, the mount point, a manual mount, the
    credentials file with username, password and domain, and every `/etc/fstab`
    option explained; other network drives and Windows mapped drives)
  - `operations.md` (state, logs, recovery, exit codes)
  - `safety-model.md` (what is and is not guaranteed)
  - `troubleshooting.md` (keyed by exact error message)
  - `CHANGELOG.md` at the root (Keep a Changelog format)
- **Navigation:** every page in `docs/` starts with
  `[Home](../README.md) · [All documentation](README.md)` and ends with a `---`
  line followed by Previous / Next links (in the order of `docs/README.md`), All
  documentation and Home. A new page goes into the index and into that chain.
  The `docs/cli/` pages get their navigation line from
  `internal/tools/gendocs`.
- **Style:**
  - Concise, factual, no promotional language, no emoji.
  - Plain Markdown headings, lists, tables and code blocks.
  - Never claim unconditional zero data loss: a live, changing source limits what any
    tool can guarantee.
  - Check every documented command against `--help`.

**A change is done when:**
1. `./scripts/check.sh` passes.
2. New behaviour has a scenario test.
3. Docs and generated docs are updated.
4. `CHANGELOG.md` has an entry.
5. The change summary (an agent's reply, or a human's change description) explains,
   in plain language:
   - what changed;
   - how it was tested;
   - any `SAFETY:` lines or safety tests it touched.

## 10. Roadmap

| Phase | Work | Status |
|---|---|---|
| 1: Foundation | `go.mod`, the layout from section 6, `scripts/check.sh`, CI, `install.sh` / `update.sh` / `uninstall.sh`. Write `docs/design.md` from section 5 and confirm the *(proposed)* items with the owner. | Done; owner confirmation pending (see below) |
| 2: Core | config, `init`, `doctor`, `run`, `status`, `logs`, `stop`; scan, plan, rsync, verify, deferral, messages; state lock and control socket; scenario tests ported from the Python suite | Done |
| 3: Records on the destination | verified-file record, `verify`, `check-copied` (was `check-archived`), `history list/restore`, run summaries, `fleet` | Done |
| 4: Scheduling | `schedule`, `service install/uninstall/status` for systemd and Task Scheduler on WSL; the real-WSL checks (drvfs behaviour, WSL scheduling) | Done; drvfs checks recorded in `docs/troubleshooting.md`; the two real-WSL scheduling checks are still open (`docs/scheduling.md`) |
| 5: Finish | Complete the docs. With the owner's approval, remove the Python code and its docs. The owner tags `v1.0.0`. | Done except the `v1.0.0` tag |

**Open questions for the owner.** Each was implemented with a default that can be
changed; see [docs/design.md](docs/design.md#decisions-taken-while-building-for-the-owner-to-confirm):
- Exit code numbers (implemented as proposed: 0, 1, 2, 3, 130).
- Whether history pruning should ever exist, even as a manual command (none now).
- The default `settle_time` (10 minutes).
- rsync minimum 3.2.4 rather than "3.2+" (`--fsync` needs 3.2.4).

**Deferred: macOS support.** This was considered on 2026-09-30 and postponed because
it added complexity without being needed yet. If it is revisited, the known issues are:
- **rsync.** macOS ships an unsuitable rsync (2.6.9 or openrsync), so rsync 3.2+
  would have to be built from source without Homebrew.
- **Mounts.** SMB shares appear under `/Volumes` and disappear on sleep or logout.
- **Scheduling.** macOS uses launchd instead of systemd.
- **Privacy permissions.** Background access to network volumes and user folders
  needs permission in System Settings.
- **Sleep.** Schedules must use wall-clock time, because a timer can pause while the
  Mac sleeps.

## 11. Lessons from the legacy Python implementation

The Python version (`src/synctoceph/`, `tests/test_synctoceph.py`) was removed in
Phase 5; it is in the git history. Its intent was ported. Keep these lists in mind
when changing the code.

**Behaviour kept:**
- SHA-256 verification with checks that the source file is unchanged.
- The per-run history folder.
- Rejecting symlinks on destination paths, and file/directory type conflicts.
- Separate, non-nested paths.
- The mount guard.
- Atomic `status.json`.
- Rotating logs.
- Forwarding SIGINT to rsync's process group, then SIGKILL after 30 s.
- rsync inheriting the lock descriptors.
- Handling daylight-saving changes for daily schedules.
- Filenames with spaces, newlines and shell characters handled literally.

**Problems not reproduced (keep it that way):**
- Config and state depended on the current working directory.
- Built-in lab data paths as defaults.
- A state folder on drvfs failing with an unclear error.
- `--immediate` using up that day's `--at` run.
- One changing file failing the whole run.
- Re-reading the entire dataset twice on every run (`--checksum` plus full
  re-verification).
- No exclude patterns.
- Raw byte counts in `status`.
