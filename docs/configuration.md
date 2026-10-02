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

## How data is organised

The source holds one folder per animal. Each profile has a `subfolder` name
for the kind of data it copies, and every animal folder is copied into that
subfolder of the animal's folder on the destination:

```
source        /mnt/d/luminoseData/LUMS0014/2026-10-01/events.csv
destination   /mnt/ceph/project/LUMS0014/behaviour/2026-10-01/events.csv
                                 ^ animal ^ subfolder
```

Other acquisition computers use the same `destination` with their own
`subfolder` (for example `ephys` or `histology`), so the files of one animal
from every machine end up together in `/mnt/ceph/project/LUMS0014/`, each
machine in its own subfolder. Animal folders are created on the destination
as needed; a machine only ever writes inside its own subfolder.

- Choose `source` so that the folders directly in it are the animal folders.
  `synctoceph init` and `synctoceph doctor` list the folders they found.
- Files directly in `source`, outside any animal folder, are not copied; each
  run lists them.
- If the data on the acquisition computer is already inside a folder named
  like the subfolder (`LUMS0014/behaviour/...`), it is copied as
  `LUMS0014/behaviour/behaviour/...`. Point `source` at the folder that holds
  the animal folders, and let `subfolder` add the data type.

## Example

```toml
source        = "/mnt/d/luminoseData"
destination   = "/mnt/ceph/project"
subfolder     = "behaviour"
require_mount = "/mnt/ceph"
existing      = "skip"
verify        = "new"
settle_time   = "10m"
exclude       = ["Thumbs.db", "desktop.ini", "~$*"]
exclude_hidden = true
interval      = "4h"
# at          = "02:00"
dry_run       = false
verbose       = true
```

## Keys

| Key | Default | Meaning |
|---|---|---|
| `source` | (required) | Full path of the folder to copy from, holding one folder per animal. It is only read. |
| `destination` | (required) | Full path of the folder on ceph (or another mounted network drive) that holds the animal folders. It must already exist; synctoceph never creates it. |
| `subfolder` | (required) | This machine's folder inside every animal folder, for example `behaviour`, `ephys` or `histology`: `<source>/<animal>/...` goes to `<destination>/<animal>/<subfolder>/...`. 1 to 63 letters, digits, `.`, `-` or `_`, starting with a letter or digit. |
| `require_mount` | `""` (no check) | A mount point that must be mounted and must contain `destination`, for example `/mnt/ceph`. Strongly recommended for network shares and Windows drives: without it, an unmounted share could look like an empty local folder. `init` suggests the right value. |
| `existing` | `"skip"` | What to do with files already on the destination that differ from the source. `skip`: leave them and list them (`status --differing`). `replace`: copy the source version and keep the old one in `<destination>/.syncToCeph/<subfolder>/history/<run-id>/`. |
| `verify` | `"new"` | Which files to check with SHA-256. `new`: the files copied in this run. `all`: every file (reads all data on both sides; slow). |
| `settle_time` | `"10m"` | Files modified more recently than this are left for a later run. |
| `exclude` | `[]` (`init` writes the example above) | Files and folders to leave out; see [Leaving files and folders out](#leaving-files-and-folders-out). |
| `exclude_hidden` | `false` (`init` writes `true`) | `true`: leave out every file and folder whose name starts with `.`, such as `.git`, `.DS_Store` or `.Trash-1000`, at any depth. The same as adding `".*"` to `exclude`. |
| `interval` | none | Schedule: run this long after the previous run ended, e.g. `"30m"`, `"4h"`, `"1d"`. |
| `at` | none | Schedule: run daily at this local time, `"HH:MM"` (24-hour). Set `interval` or `at`, not both. |
| `dry_run` | `false` | `true`: only report what would be copied; never write to the destination. |
| `verbose` | `true` | `true`: `run` and `schedule` list every file as it is copied and verified. `false`: they show only each step and the result. Override once with `-v` or `--verbose=false`; `-q` prints only the result. |

Durations are written like `30s`, `10m`, `4h`, `1h30m` or `1d`.

Unknown keys are errors, so a misspelt key is never silently ignored. There is
no way to pass options to rsync; its options are fixed (see
[design.md](design.md#how-a-run-works)).

Config files from earlier versions used `archive` and `machine_name`. They
are refused with an explanation: `archive` is now `destination`, and
`machine_name` is replaced by `subfolder`. Run `synctoceph init --force` to
answer the questions again.

## Leaving files and folders out

`exclude` is a list of patterns. `*` matches any characters, `?` one
character, `[abc]` one of a set. Excluded folders are not entered, so
nothing inside them is copied.

| Pattern | Leaves out |
|---|---|
| `"Thumbs.db"`, `"*.tmp"` | Files or folders with that name, in any folder (no `/` in the pattern). |
| `"scratch/"` | Folders named `scratch`, in any folder; a file named `scratch` is still copied (a trailing `/` means folders only). |
| `"LUMS0014/scratch"` | That one path, counted from the source folder (a `/` inside the pattern). |
| `"/LUMS0099"` | A whole animal folder: a leading `/` also counts from the source folder. |
| `".*"` | Everything whose name starts with a dot; `exclude_hidden = true` does the same. |

Some names are always left out, whatever the settings: `.syncToCeph` and
`.syncToCeph-partial`, which synctoceph uses itself. They are listed as
skipped.

Excluded entries are counted in each run ("N excluded" after the scan), and
`synctoceph check-copied` lists them as `EXCLUDED`. A folder that contains
excluded entries is never reported as safe to delete, because those entries
are not on ceph.

Without `exclude_hidden`, hidden files are copied like any other file, and a
hidden folder directly in the source (for example `.Trash-1000` on a Linux
data drive) would be copied as if it were an animal. `init` therefore turns
`exclude_hidden` on; turn it off only if your acquisition software keeps data
in hidden files.

## Several jobs on one computer (profiles)

Each profile is one config file with its own source, destination, subfolder,
schedule and settings. Use one profile per kind of data, for example a second
data drive with video, copied into a `video` subfolder:

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
writing the same subfolder take turns (a lock in
`<destination>/.syncToCeph/<subfolder>/`).

`run --all-profiles` prints a summary at the end. Its exit code is that of the
worst result (FAILED before PARTIAL before OK). A profile that cannot start,
for example because of a config mistake, is reported and the others still
run. Ctrl-C (or `synctoceph stop`) stops the current profile and skips the
rest.

Give each profile its own `subfolder` when they write to the same destination
and hold different kinds of data (for example `behaviour` and `video`). If
two profiles copy into the same subfolder, `init` and `doctor` point it out:
files with the same path in both sources would meet in one place.

Different computers must use different subfolders too. Locks only work
within one computer, so two computers with the same `destination` and
`subfolder` would write the same folders and share one verified-file record.

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

Previous: [Installation](installation.md) · Next: [Mounting ceph](mounting.md) · [All documentation](README.md) · [Home](../README.md)
