[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph init

Create the configuration for this computer

### Synopsis

Asks for the source folder (one folder per animal), the destination folder on
ceph, this machine's subfolder name, the mount point that must be present,
and the schedule. Each answer is checked, then the configuration is written
to ~/.config/synctoceph/<profile>.toml.

Every question can also be answered with a flag; with --yes, questions not
answered by a flag use their default. The destination folder must already
exist: synctoceph never creates it.

For a second job (another source, destination or subfolder), create another
profile: synctoceph --profile NAME init.

```
synctoceph init [flags]
```

### Options

```
      --destination DIR     folder DIR on ceph that holds the animal folders (must already exist)
      --force               replace an existing configuration
  -h, --help                help for init
      --require-mount DIR   only run when DIR is mounted; "none" for no check (default: the network mount holding the destination, if any)
      --schedule WHEN       when to run: WHEN is an interval such as 4h, a daily time such as 02:00, or "none" (default: 4h)
      --source DIR          folder DIR to copy from, holding one folder per animal
      --subfolder NAME      this machine's folder NAME inside every animal folder, e.g. behaviour or ephys
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

* [synctoceph](synctoceph.md)	 - Copy acquisition data to ceph with rsync, verified with SHA-256

