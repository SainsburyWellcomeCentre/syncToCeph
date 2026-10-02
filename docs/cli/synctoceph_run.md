[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph run

Copy new files to the archive and verify them

### Synopsis

Runs one sync: scans the source, copies files that are not in the archive yet,
and verifies every copy with SHA-256.

Files already in the archive are left alone unless the config has
existing = "replace" or --existing replace is given; then the old version is
kept under .syncToCeph/history/<run-id>/.
Files modified within settle_time, or that change during the run, are
deferred to a later run.

Each file is listed as it is copied and verified (setting: verbose). For one
run, --verbose=false shows only the steps and the result, -v lists every file
and -q prints only the result.

--all-profiles runs every profile one after another and ends with a summary;
the exit code is that of the worst result.

Exit codes: 0 OK, 1 FAILED, 2 usage error, 3 PARTIAL (files deferred),
130 INTERRUPTED.

```
synctoceph run [flags]
```

### Options

```
      --all-profiles    run every profile, one after another
      --dry-run         show what would be copied; write nothing to the archive
      --existing MODE   MODE for files already in the archive that differ: skip or replace (default: the config's existing setting, skip unless changed)
  -h, --help            help for run
      --verify MODE     MODE for SHA-256 checks: new (only files copied in this run) or all (default: the config's verify setting, new unless changed)
```

### Options inherited from parent commands

```
      --no-color       never use colour (NO_COLOR is also honoured)
      --profile NAME   use the profile (job) NAME; its settings are in ~/.config/synctoceph/NAME.toml (default "default")
  -q, --quiet          print only the result and errors
  -v, --verbose        list every file copied and verified; --verbose=false turns it off (default: the config's verbose setting, true unless changed)
```

### SEE ALSO

* [synctoceph](synctoceph.md)	 - Copy acquisition data to the lab archive, verified with SHA-256

