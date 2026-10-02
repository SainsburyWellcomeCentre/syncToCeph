[Home](../README.md) · [All documentation](README.md)

# Troubleshooting

Find the first line of the message (after `ERROR`) below. Every message also
says what to do; this page adds background. `synctoceph doctor` checks most of
these at once.

## Setup and configuration

**`no configuration found at ...`**
Run `synctoceph init`. synctoceph has no built-in data folders. With
`--profile NAME`, the file is `~/.config/synctoceph/NAME.toml`.

**`unknown setting(s) in ...: ...`**
A key in the config file is misspelt or does not exist. Valid keys are in
[configuration.md](configuration.md). There is no key for rsync options, by
design. If the keys are `archive` and `machine_name`, the file is from an
earlier version: `archive` is now `destination`, and `machine_name` is
replaced by `subfolder`. Run `synctoceph init --force`.

**`the setting KEY = "VALUE" is not valid`**, **`the setting KEY is missing`**,
**`both interval and at are set`**, **`cannot read the configuration ...`**
Correct the config file (the message names the key and the rule), or answer
the questions again with `synctoceph init --force`.

**`a configuration already exists at ...`**
`init` never overwrites by accident. Use `synctoceph init --force`; the current
values are offered as defaults. To copy another folder as a separate job,
create another profile instead: `synctoceph --profile NAME init`.

**`no profiles found in ...`**
`--all-profiles` works on every config file in `~/.config/synctoceph/`, and
there is none. Create one with `synctoceph init`.

**`--profile and --all-profiles cannot be used together`**
Use one or the other: `--profile NAME` for one profile, `--all-profiles` for
every profile.

**`Profiles A, B all copy into ...`** (a NOTE from `init` or `doctor`)
Two profiles write the same subfolder of the same destination. That works,
but files with the same path in both sources meet in one place. If the
sources hold different kinds of data, give each profile its own `subfolder`.
See
[configuration.md](configuration.md#several-jobs-on-one-computer-profiles).

**`the time zone TZ="..." is not known`**
Set `TZ` to a name such as `Europe/London`, or unset it.

## The destination and the mount

**`... is not mounted`**
`require_mount` is set and nothing is mounted there. Mount ceph
([mounting.md](mounting.md)); `synctoceph doctor` prints the commands. With
`x-systemd.automount` in `/etc/fstab`, check that the credentials file and
the network are fine: `ls /mnt/ceph` shows the error.

**`require_mount ... does not contain the destination ...`**
`destination` must be inside `require_mount`, for example
`/mnt/ceph/project` inside `/mnt/ceph`.

**`the destination ... is not on the filesystem mounted at ...`**
Another filesystem is mounted between the mount point and the destination
folder. `findmnt -T <destination>` shows which mount holds it; use that as
`require_mount`.

**`the destination folder ... does not exist or is not reachable`**
synctoceph never creates the destination folder. Mount ceph, or create the
folder yourself if it really is new, or correct `destination`.

**`the destination path ... goes through a symlink`**
Use the real path: `realpath <path>` prints it.

**`... was not copied: ... the destination has a symlink at this path`**, **`... the destination has a file at ... where the source has a folder`**, **`... destination folder ... is a symlink`**
Something on the destination (the animal folder, this machine's subfolder,
or a folder below it) has the same name as a source file or folder but is of
another kind, or is a symlink. synctoceph never deletes or follows it.
Rename or move it by hand; the next run copies the file.

**`... not inside an animal folder ...`** (in the list of skipped files)
The file is directly in the source folder. Only folders directly in the
source are copied, each as one animal. Move the file into an animal folder,
or check that `source` points at the folder that holds the animal folders.

**`another synctoceph run on this computer is copying into the subfolder ...`**
Two profiles on this computer use the same destination and `subfolder`, and
one is running. Wait, or give each profile its own `subfolder`.

**`cannot prepare the folder ...`**, **`cannot prepare the records folder ...`**
synctoceph could not create an animal folder, its subfolder, or
`.syncToCeph/<subfolder>/` on the destination. Check that you can write to
the destination (`touch <destination>/test && rm <destination>/test`).

**`you cannot write to ...`** (from `doctor`)
Check the share's permissions and mount options; for cifs, `uid=` and `gid=`
make the files yours (see [mounting.md](mounting.md)).

## The source

**`the source folder ... does not exist or cannot be opened`**
On WSL, check that the data drive is visible (`ls /mnt/d`). Correct `source`
if it moved.

**`cannot read ... in the source (...); it was not copied`**
A folder or file could not be read, usually because of permissions. Fix them
and run again.

**`the source (...) and the destination (...) overlap`** (or the state folder)
One folder is inside another. Choose separate folders.

**DEFERRED: files changed recently or during the sync**
Normal while acquisition is running. Files are copied once unchanged for
`settle_time`. If the same files stay deferred for more than 24 hours, a
WARNING appears: a program may still have them open, or the computer clock is
wrong (a file "modified in the future" waits until that time).

## rsync

**`rsync is not installed (or not on PATH)`**, **`rsync ... is too old`**, **`cannot tell which rsync version ... is`**
Install rsync 3.2.4 or newer, e.g. `sudo apt install rsync`. Older rsync lacks
`--fsync`. macOS's built-in rsync or openrsync are not suitable.

**`rsync reported an error (exit code N: ...)`**
The rsync lines in `synctoceph logs --run RUN_ID` show the cause (often a full
or disconnected share, or permissions). Completed copies are verified and kept;
run again after fixing the cause.

## Verification

**`... is NOT copied and verified: the copy on the destination differs from the source (SHA-256 mismatch)`**
The copy on ceph does not match the source, although the source did not
change. The next run checks it again. If it repeats, compare both copies by
hand; the storage may be faulty. `synctoceph run --existing replace` copies it
again and keeps the damaged copy in history.

**`... is NOT copied and verified: the copy on the destination is missing`**
rsync did not produce the file (see the rsync lines in the log). Run again.

**`cannot write the verified-file record ...`**
ceph is full or mounted read-only.

## Running, stopping and the state folder

**`synctoceph is already running for profile ...`**
A run or the scheduler (service) is active. `synctoceph status` shows it;
`synctoceph stop` stops it. If a synctoceph process was killed while rsync was
copying, the lock stays held until that rsync exits.

**`synctoceph is running for profile ... but does not answer on its control socket`**
Wait a moment and try again. Otherwise stop it with `kill -TERM <process>`
(the number is shown in the message).

**`the state folder ... is on a Windows drive`** (or a network share)
Locks and sockets are not reliable there. Unset `XDG_STATE_HOME`, or point it
to a folder on the Linux disk.

**`cannot use the state folder ...`**
It must be a folder you own that others cannot write to:
`chmod 700 ~/.local/state/synctoceph/<profile>`.

## Installing and uninstalling

**`--purge asks for confirmation; run it in a terminal, or add --yes`**
`./uninstall.sh --purge` deletes your configuration and logs, so it asks
first. Run it in a terminal and type `yes`, or add `--yes` in a script.

**`a synctoceph run is still active`** (from `./uninstall.sh --purge`)
A run started by hand is still using the state folder. Stop it with
`synctoceph --profile NAME stop` (the table above the message shows which
profile), then run `./uninstall.sh --purge` again. Only the automatic runs
were removed.

## Automatic runs

**`no schedule is set for profile ...`**
Add `interval = "4h"` or `at = "02:00"` to the config file.

**`no service manager was found`**
Neither systemd nor Windows Task Scheduler is available. Enable systemd in WSL
(see [scheduling.md](scheduling.md)) or run `synctoceph schedule` in a
terminal.

**`setting up automatic runs failed`**
The detail line shows the failing command. `synctoceph service install --print`
shows what would be set up, so you can do it by hand.

## Windows drives (drvfs)

The design brief asked for drvfs behaviour to be checked before relying on it.
Checked on 2026-09-30 on WSL2 (kernel 5.15.167.4-microsoft-standard-WSL2,
rsync 3.2.7), with a Windows NTFS drive mounted by WSL as `9p` with
`aname=drvfs`:

| Question | Result |
|---|---|
| Does `rsync --times` work? | Yes, to whole seconds only: a source time of `03:04:05.123456789` became `03:04:05.000000000`. synctoceph compares times with a one-second tolerance, so repeat runs correctly see such files as unchanged. |
| Is rename into place atomic? | `rename()` over an existing file succeeds and the target has the new content; no partial state was seen. Windows (NTFS) implements it as a replace operation; a power cut at that instant was not tested. |
| Does `fsync` work? | Yes, for files and for folders (no error). Whether Windows flushes to disk when asked was not measured. |
| Does `flock` work? | Yes, between processes in the same WSL instance (a second lock was refused). |
| Does `--partial-dir` work after an interruption? | Yes: the interrupted file was kept in `.syncToCeph-partial/`, nothing appeared under the final name. |
| Full runs | Destination (then called the archive) on drvfs: copy, repeat run ("nothing new"), `--verify all` and `check-archived` (now `check-copied`) all behaved correctly; `init` suggested `require_mount = "/mnt/c"`. Source on drvfs: same; inode numbers stayed stable between runs. |
| Is a mapped Windows drive visible to a scheduled task? | Not yet verified; see [scheduling.md](scheduling.md). |

These checks used a local NTFS drive, not a network drive mapped through
Windows; results for a mapped network share should be confirmed the same way
before relying on it.

---

Previous: [Operations](operations.md) · Next: [Safety model](safety-model.md) · [All documentation](README.md) · [Home](../README.md)
