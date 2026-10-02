[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph history list

List runs that kept previous versions, or the files of one run

```
synctoceph history list [RUN_ID] [flags]
```

### Options

```
  -h, --help   help for list
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

* [synctoceph history](synctoceph_history.md)	 - List and restore previous versions of replaced files

