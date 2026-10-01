[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph run

Copy new files to the archive and verify them

### Synopsis

Runs one sync: scans the source, copies files that are not in the archive yet,
and verifies every copy with SHA-256.

Files already in the archive are left alone unless --existing replace is
given; then the old version is kept under .syncToCeph/history/<run-id>/.
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
      --all-profiles      run every profile, one after another
      --dry-run           show what would be copied; write nothing to the archive
      --existing string   files already in the archive that differ: skip or replace (default from config: skip)
  -h, --help              help for run
      --verify string     what to check with SHA-256: new (files copied now) or all (default from config: new)
```

### Options inherited from parent commands

```
      --no-color         never use colour (NO_COLOR is also honoured)
      --profile string   configuration profile, for several jobs on one computer (default "default")
  -q, --quiet            print only the result and errors
  -v, --verbose          list every file copied and verified (default from config: verbose = true); --verbose=false turns it off
```

### SEE ALSO

* [synctoceph](synctoceph.md)	 - Copy acquisition data to the lab archive, verified with SHA-256

