[Home](../README.md) · [All documentation](README.md)

# Configuration

Each profile has one config file:

```
${XDG_CONFIG_HOME:-~/.config}/synctoceph/<profile>.toml
```

The default profile is `default`; choose another with `--profile NAME` (letters,
digits, `-` and `_`). See [Several jobs on one computer](#several-jobs-on-one-computer-profiles).

`synctoceph init` writes the file with an explanation above every key. You can
edit it by hand; `synctoceph doctor` checks it.

## Example

```toml
machine_name  = "scope-2p-01"
source        = "/mnt/d/acquisition"
archive       = "/mnt/z/lab-archive"
require_mount = "/mnt/z"
existing      = "skip"
verify        = "new"
settle_time   = "10m"
exclude       = ["Thumbs.db", "desktop.ini", "~$*"]
interval      = "4h"
# at          = "02:00"
dry_run       = false
verbose       = true
```

## Keys

| Key | Default | Meaning |
|---|---|---|
| `machine_name` | host name | Name of this machine's folder in the archive: data goes to `<archive>/<machine_name>/`. 1 to 63 letters, digits, `.`, `-` or `_`, starting with a letter or digit. |
| `source` | (required) | Full path of the folder to copy from. It is only read. |
| `archive` | (required) | Full path of the archive root on the lab share. It must already exist; synctoceph never creates it. |
| `require_mount` | `""` (no check) | A mount point that must be mounted and must contain `archive`, for example `/mnt/z`. Strongly recommended for network shares and Windows drives: without it, an unmounted share could look like an empty local folder. `init` suggests the right value. |
| `existing` | `"skip"` | What to do with files already in the archive that differ from the source. `skip`: leave them and list them (`status --differing`). `replace`: copy the source version and keep the old one in `.syncToCeph/history/<run-id>/`. |
| `verify` | `"new"` | Which files to check with SHA-256. `new`: the files copied in this run. `all`: every file (reads all data on both sides; slow). |
| `settle_time` | `"10m"` | Files modified more recently than this are left for a later run. |
| `exclude` | `[]` (`init` writes the example above) | Names to leave out. `*` matches any characters, `?` one character, `[abc]` one of a set. A pattern without `/` matches a file or folder name anywhere; a pattern with `/` matches a path from the source folder (e.g. `raw/scratch`). Excluded folders are not entered. |
| `interval` | none | Schedule: run this long after the previous run ended, e.g. `"30m"`, `"4h"`, `"1d"`. |
| `at` | none | Schedule: run daily at this local time, `"HH:MM"` (24-hour). Set `interval` or `at`, not both. |
| `dry_run` | `false` | `true`: only report what would be copied; never write to the archive. |
| `verbose` | `true` | `true`: `run` and `schedule` list every file as it is copied and verified. `false`: they show only each step and the result. Override once with `-v` or `--verbose=false`; `-q` prints only the result. |

Durations are written like `30s`, `10m`, `4h`, `1h30m` or `1d`.

Unknown keys are errors, so a misspelt key is never silently ignored. There is
no way to pass options to rsync; its options are fixed (see
[design.md](design.md#how-a-run-works)).

## Several jobs on one computer (profiles)

Each profile is one config file with its own source, archive folder,
schedule and settings. Use one profile per job, for example a second data
drive, or the same data copied to a second archive:

```
synctoceph init                                  # the "default" profile
synctoceph --profile video init                  # a second job, in video.toml
synctoceph --profile video run                   # run one profile
synctoceph run --all-profiles                    # run every profile, one after another
synctoceph status --all-profiles                 # one line per profile
synctoceph doctor --all-profiles                 # check every profile
synctoceph service install --all-profiles        # automatic runs for every profile with a schedule
```

`--profile` can go before or after the command name. Each profile has its own
state folder, log and lock, so profiles never block each other; two profiles
writing the same machine folder take turns (a lock in the archive).

`run --all-profiles` prints a summary at the end. Its exit code is that of the
worst result (FAILED before PARTIAL before OK). A profile that cannot start,
for example because of a config mistake, is reported and the others still
run. Ctrl-C (or `synctoceph stop`) stops the current profile and skips the
rest.

Give each profile its own `machine_name` when they write to the same archive
and their sources are unrelated (for example `scope-01-ephys` and
`scope-01-video`). If two profiles copy into the same folder, `init` and
`doctor` point it out: files with the same path in both sources would meet
in one place.

## Command-line flags

Flags take precedence over the config file for one run:

```
synctoceph run --dry-run
synctoceph run --existing replace
synctoceph run --verify all
```

Global flags accepted by every command: `--profile NAME`, `-v`/`--verbose`
(or `--verbose=false`), `-q`/`--quiet`, `--no-color`. The full list is in
[cli/](cli/synctoceph.md).

## Environment variables

| Variable | Effect |
|---|---|
| `XDG_CONFIG_HOME`, `XDG_STATE_HOME` | Where config and state are kept (defaults `~/.config`, `~/.local/state`). The state folder must be on a local Linux disk. |
| `TZ` | Time zone for `at`, e.g. `Europe/London`. Unset: the system time zone. |
| `NO_COLOR` | Any value turns colour off. |

---

Previous: [Installation](installation.md) · Next: [Mounting the share](mounting.md) · [All documentation](README.md) · [Home](../README.md)
