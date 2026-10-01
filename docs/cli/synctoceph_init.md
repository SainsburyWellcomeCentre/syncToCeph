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
      --archive string         root folder of the lab archive (must exist)
      --force                  replace an existing configuration
  -h, --help                   help for init
      --machine-name string    name of this computer's folder in the archive (default: host name)
      --require-mount string   mount point that must be mounted ("none" for no check)
      --schedule string        how often to run: an interval such as 4h, a daily time such as 02:00, or "none"
      --source string          folder to copy from
  -y, --yes                    do not ask; use flags and defaults
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

