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
  -n, --lines int    number of lines to show (0 = all) (default 50)
      --run string   show only the lines of this run ID
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

