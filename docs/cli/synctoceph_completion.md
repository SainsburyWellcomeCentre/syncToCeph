[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph completion

Print a shell completion script (bash or zsh)

### Synopsis

Prints a script that lets the shell complete synctoceph commands and flags.
install.sh installs it for you. To load it by hand in bash:
  source <(synctoceph completion bash)

```
synctoceph completion bash|zsh [flags]
```

### Options

```
  -h, --help   help for completion
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

