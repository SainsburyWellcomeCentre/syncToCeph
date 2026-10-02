[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph stop

Stop a running sync or scheduler gracefully

### Synopsis

Asks the running synctoceph for this profile to stop. rsync is asked to stop
(SIGINT) and keeps its partial file so the next run can resume; if it has not
stopped after --timeout seconds it is killed. An interrupted run is never
reported as verified.

```
synctoceph stop [flags]
```

### Options

```
  -h, --help              help for stop
      --timeout SECONDS   give rsync SECONDS seconds to stop before it is killed (default 30)
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

