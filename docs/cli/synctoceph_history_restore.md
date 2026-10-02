[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph history restore

Copy a previous version out of the history

### Synopsis

Copies the previous version of PATH (a file or folder, relative to the
machine folder) kept by run RUN_ID into DIR, as DIR/PATH. DIR must be outside
the archive, and existing files are never overwritten. Each copy is checked
with SHA-256.

```
synctoceph history restore RUN_ID PATH --to DIR [flags]
```

### Options

```
  -h, --help     help for restore
      --to DIR   restore into folder DIR (must be outside the archive)
```

### Options inherited from parent commands

```
      --no-color       never use colour (NO_COLOR is also honoured)
      --profile NAME   use the profile (job) NAME; its settings are in ~/.config/synctoceph/NAME.toml (default "default")
  -q, --quiet          print only the result and errors
  -v, --verbose        list every file copied and verified; --verbose=false turns it off (default: the config's verbose setting, true unless changed)
```

### SEE ALSO

* [synctoceph history](synctoceph_history.md)	 - List and restore previous versions of replaced files

