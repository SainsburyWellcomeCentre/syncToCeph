[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph doctor

Check the setup and explain how to fix problems

### Synopsis

Checks the configuration, rsync, the source folder, the archive mount (and its
filesystem type), the archive folder, the state folder, the time zone and the
service, and notes when another profile copies into the same archive
folder. Each problem is printed with how to fix it. Nothing is changed.
--all-profiles checks every profile.

Exit code 0 means no problems were found.

```
synctoceph doctor [flags]
```

### Options

```
      --all-profiles   check every profile
  -h, --help           help for doctor
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

