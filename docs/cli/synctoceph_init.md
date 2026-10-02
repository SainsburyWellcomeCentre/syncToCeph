[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph init

Create the configuration for this computer

### Synopsis

Asks for the machine name, the source folder, the archive folder, the mount
point that must be present, and the schedule. Each answer is checked, then the
configuration is written to ~/.config/synctoceph/<profile>.toml.

Every question can also be answered with a flag; with --yes, questions not
answered by a flag use their default. The archive folder must already exist:
synctoceph never creates it.

For a second job (another source or archive folder), create another profile:
synctoceph --profile NAME init.

```
synctoceph init [flags]
```

### Options

```
      --archive DIR         root folder DIR of the lab archive (must already exist)
      --force               replace an existing configuration
  -h, --help                help for init
      --machine-name NAME   folder NAME for this computer in the archive (default: the host name)
      --require-mount DIR   only run when DIR is mounted; "none" for no check (default: the network mount holding the archive, if any)
      --schedule WHEN       when to run: WHEN is an interval such as 4h, a daily time such as 02:00, or "none" (default: 4h)
      --source DIR          folder DIR to copy from
  -y, --yes                 do not ask; use flags and defaults
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

