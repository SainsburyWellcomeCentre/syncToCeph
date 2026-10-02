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
      --no-color       never use colour (NO_COLOR is also honoured)
      --profile NAME   use the profile (job) NAME; its settings are in ~/.config/synctoceph/NAME.toml (default "default")
  -q, --quiet          print only the result and errors
  -v, --verbose        list every file copied and verified; --verbose=false turns it off (default: the config's verbose setting, true unless changed)
```

### SEE ALSO

* [synctoceph](synctoceph.md)	 - Copy acquisition data to ceph with rsync, verified with SHA-256

