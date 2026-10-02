[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph logs

Show recent log lines

### Synopsis

Prints the last lines of the log (kept in the state folder, rotated at 5 MB,
with 5 older files kept). --run shows only the lines of one run.

```
synctoceph logs [flags]
```

### Options

```
  -h, --help         help for logs
      --json         print machine-readable JSON
  -n, --lines N      show the last N lines; 0 shows all (default 50)
      --run RUN_ID   show only the lines of run RUN_ID
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

