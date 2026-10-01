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
      --no-color         never use colour (NO_COLOR is also honoured)
      --profile string   configuration profile, for several jobs on one computer (default "default")
  -q, --quiet            print only the result and errors
  -v, --verbose          show more detail, including every rsync line
```

### SEE ALSO

* [synctoceph](synctoceph.md)	 - Copy acquisition data to the lab archive, verified with SHA-256

