[Home](../README.md) · [All documentation](README.md)

# Safety model

What synctoceph guarantees about research data, how, and what it does not
guarantee.

## Guarantees

Each of these is enforced by code marked with a `SAFETY:` comment (list them
with `grep -rn "SAFETY:" internal`) and checked by at least one test.

| # | Guarantee | How |
|---|---|---|
| 1 | Nothing is ever deleted from the archive. | The rsync options are fixed in code and never include `--delete*`, `--remove-source-files`, `--inplace` or `--append`. The config file rejects unknown keys, so no option can be passed through. |
| 2 | Nothing is deleted from the source. | Source files are only opened read-only; rsync never gets `--remove-source-files`. |
| 3 | Replaced files are kept. | In `replace` mode rsync moves the previous version to `<machine folder>/.syncToCeph/history/<run-id>/<path>` before writing the new one. History is never pruned. |
| 4 | "Archived" means verified. | A file is recorded as archived only after the SHA-256 of the source and the archive copy match and the source still has the size, modification time and inode seen by the scan (checked before and after reading). |
| 5 | Unfinished copies never appear under the final name. | rsync writes to a temporary name, flushes it to disk (`--fsync`) and renames it; an interrupted copy goes to `.syncToCeph-partial/`. So a file under its final name is complete, which makes skipping existing files safe. |
| 6 | The archive root is never created. | A missing archive folder is an error. With `require_mount`, the mount must be present, contain the archive, and be the filesystem holding it. |
| 7 | Source, archive and state are separate. | None may be inside another (after following symlinks); checked before the state folder is created. |
| 8 | Symlinks are never followed. | Any symlink on an archive path the tool touches is refused; source symlinks and special files are skipped and reported. |
| 9 | A dry run writes nothing to the archive. | A dry run does not create folders, start rsync, or write summaries. |
| 10 | One run per profile at a time. | A lock in the state folder, inherited by rsync so it stays held while rsync runs. A second lock in `.syncToCeph/` stops two profiles on one computer writing the same machine folder. |
| 11 | Stopping is graceful. | Stop, Ctrl-C and SIGTERM send SIGINT to rsync's process group, then SIGKILL after 30 seconds. An interrupted run is never reported as verified. |

Uninstalling keeps these promises too: `./uninstall.sh --purge` deletes only
synctoceph's own files on this computer (config, state, logs, build cache).
The archive, including its `.syncToCeph/` folder with the verified-file
record and the history of replaced files, and the source are never touched.

## What is not guaranteed

- **A live, changing source limits any guarantee.** A file still being written
  is deferred, not copied. A file that changes in a way that keeps its size,
  modification time and inode cannot be told apart from an unchanged one.
- **Files already in the archive are compared by size and time** (within one
  second) unless `verify = all` or `synctoceph verify` is used. A change that
  keeps both is not noticed by a normal run.
- **Damage to the archive after verification** (for example by other programs
  or storage faults) is found only by `synctoceph verify` or `--verify all`.
- **Deleting a file from the source** is the user's decision. Use
  `synctoceph check-archived` first; it relies on the verified-file record.
- **Locks are local.** They do not coordinate between computers; separate
  machine folders do. Two computers configured with the same `machine_name`
  and archive would write the same folder.
- **Empty source folders are not copied.**
- **Windows drives and network filesystems** are only as reliable as their
  own `rename` and `fsync`; see [troubleshooting.md](troubleshooting.md#windows-drives-drvfs).

## Changing safety-related code

The invariants above must never be weakened, removed or bypassed. A change
that touches `SAFETY:` lines or their tests must say so in its summary; see
[AGENTS.md](../AGENTS.md).

---

Previous: [Troubleshooting](troubleshooting.md) · Next: [Design](design.md) · [All documentation](README.md) · [Home](../README.md)
