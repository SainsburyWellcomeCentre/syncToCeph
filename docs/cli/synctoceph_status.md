[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph status

Show what synctoceph is doing and the last result

### Synopsis

Shows whether a run or the scheduler is active, the result of the last run,
the next scheduled run, and files that are deferred or differ from the
archive. --all-profiles shows one line per profile instead. Reads only; it
never starts anything.

```
synctoceph status [flags]
```

### Options

```
      --all-profiles   show one line for every profile
      --deferred       list files left for a later run
      --differing      list files in the archive that differ from the source and were not replaced
  -h, --help           help for status
      --json           print machine-readable JSON
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

