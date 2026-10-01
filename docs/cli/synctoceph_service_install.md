## synctoceph service install

Set up automatic runs

### Synopsis

Sets up automatic runs on the schedule from the config file:
  - with systemd (Linux, or WSL with systemd): a user service running
    "synctoceph schedule";
  - on WSL without systemd: a Windows Task Scheduler task running
    "wsl.exe -d <distro> -- synctoceph run".
--print shows what would be set up without changing anything.

```
synctoceph service install [flags]
```

### Options

```
  -h, --help             help for install
      --manager string   auto, systemd or task-scheduler (default "auto")
      --print            only show what would be set up
```

### Options inherited from parent commands

```
      --no-color         never use colour (NO_COLOR is also honoured)
      --profile string   configuration profile, for several jobs on one computer (default "default")
  -q, --quiet            print only the result and errors
  -v, --verbose          show more detail, including every rsync line
```

### SEE ALSO

* [synctoceph service](synctoceph_service.md)	 - Set up, remove or check automatic runs

