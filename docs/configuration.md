# Configuration

Each profile has one config file:

```
${XDG_CONFIG_HOME:-~/.config}/synctoceph/<profile>.toml
```

The default profile is `default`; choose another with `--profile NAME` (letters,
digits, `-` and `_`). Use several profiles for several jobs on one computer, for
example two source drives. Give each its own `machine_name` if they write to the
same archive.

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

Durations are written like `30s`, `10m`, `4h`, `1h30m` or `1d`.

Unknown keys are errors, so a misspelt key is never silently ignored. There is
no way to pass options to rsync; its options are fixed (see
[design.md](design.md#how-a-run-works)).

## Command-line flags

Flags take precedence over the config file for one run:

```
synctoceph run --dry-run
synctoceph run --existing replace
synctoceph run --verify all
```

Global flags accepted by every command: `--profile NAME`, `-v`/`--verbose`,
`-q`/`--quiet`, `--no-color`. The full list is in [cli/](cli/synctoceph.md).

## Environment variables

| Variable | Effect |
|---|---|
| `XDG_CONFIG_HOME`, `XDG_STATE_HOME` | Where config and state are kept (defaults `~/.config`, `~/.local/state`). The state folder must be on a local Linux disk. |
| `TZ` | Time zone for `at`, e.g. `Europe/London`. Unset: the system time zone. |
| `NO_COLOR` | Any value turns colour off. |
