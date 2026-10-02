[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph service uninstall

Remove automatic runs

```
synctoceph service uninstall [flags]
```

### Options

```
      --all            the same as --all-profiles
      --all-profiles   remove automatic runs of every profile
  -h, --help           help for uninstall
```

### Options inherited from parent commands

```
      --no-color       never use colour (NO_COLOR is also honoured)
      --profile NAME   use the profile (job) NAME; its settings are in ~/.config/synctoceph/NAME.toml (default "default")
  -q, --quiet          print only the result and errors
  -v, --verbose        list every file copied and verified; --verbose=false turns it off (default: the config's verbose setting, true unless changed)
```

### SEE ALSO

* [synctoceph service](synctoceph_service.md)	 - Set up, remove or check automatic runs

