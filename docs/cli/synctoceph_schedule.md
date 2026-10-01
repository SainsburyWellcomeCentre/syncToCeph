[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph schedule

Run syncs on the configured schedule (used by the service)

### Synopsis

Stays running and starts a sync on the schedule set by interval or at in the
config file. Runs never overlap, and missed runs are not replayed. Stop it
with Ctrl-C or synctoceph stop.

Normally started by the service (synctoceph service install).

```
synctoceph schedule [flags]
```

### Options

```
  -h, --help        help for schedule
      --immediate   also run once right away (does not use up a daily run)
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

