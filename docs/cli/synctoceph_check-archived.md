[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph check-archived

Check whether files are safely archived before deleting them

### Synopsis

For each source file under PATH, reports whether it is verified in the
archive: the archive copy has the same size and time, and its SHA-256 was
checked against the source. Exit code 0 means every file under PATH is
verified, so it is safe to delete PATH from this computer.

This uses the verified-file record and is fast. To re-read the files first,
run synctoceph verify PATH.

```
synctoceph check-archived PATH [flags]
```

### Options

```
  -h, --help   help for check-archived
      --json   print machine-readable JSON
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

