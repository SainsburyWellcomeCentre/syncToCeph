[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph service install

Set up automatic runs

### Synopsis

Sets up automatic runs on the schedule from the config file:
  - with systemd (Linux, or WSL with systemd): a user service running
    "synctoceph schedule";
  - on WSL without systemd: a Windows Task Scheduler task running
    "wsl.exe -d <distro> -- synctoceph run".
--print shows what would be set up without changing anything.
--all-profiles sets up every profile that has a schedule (one service or
task each).

```
synctoceph service install [flags]
```

### Options

```
      --all-profiles   set up automatic runs for every profile that has a schedule
  -h, --help           help for install
      --manager NAME   use the service manager NAME: auto, systemd or task-scheduler (default "auto")
      --print          only show what would be set up
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

