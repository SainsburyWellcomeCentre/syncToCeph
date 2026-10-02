[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph fleet

Show the latest sync of every machine copying to the destination

### Synopsis

Reads the run summaries that every machine writes to
<destination>/.syncToCeph/<subfolder>/ and shows, per subfolder, the last run,
the computer it ran on, the last successful sync, deferred files and
problems. Reads only. Uses the destination from this profile's config unless
--destination is given.

```
synctoceph fleet [flags]
```

### Options

```
      --destination DIR   read the destination DIR instead of the one in the config
  -h, --help              help for fleet
      --json              print machine-readable JSON
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

