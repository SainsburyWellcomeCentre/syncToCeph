[Home](../../README.md) · [All documentation](../README.md)

## synctoceph

Copy acquisition data to the lab archive, verified with SHA-256

### Synopsis

synctoceph copies data from this computer into its own folder on the lab's
archive share (<archive>/<machine_name>/), using rsync. It never deletes
anything from the archive or the source. A file is reported as archived only
after the SHA-256 of the source and of the archive copy match.

Start with: synctoceph init, then synctoceph doctor, then synctoceph run --dry-run.

To copy several folders (each with its own source and archive folder), give
each its own profile: synctoceph --profile NAME init. Run them all with
synctoceph run --all-profiles.

### Options

```
  -h, --help             help for synctoceph
      --no-color         never use colour (NO_COLOR is also honoured)
      --profile string   configuration profile, for several jobs on one computer (default "default")
  -q, --quiet            print only the result and errors
  -v, --verbose          list every file copied and verified (default from config: verbose = true); --verbose=false turns it off
```

### SEE ALSO

* [synctoceph check-archived](synctoceph_check-archived.md)	 - Check whether files are safely archived before deleting them
* [synctoceph completion](synctoceph_completion.md)	 - Print a shell completion script (bash or zsh)
* [synctoceph doctor](synctoceph_doctor.md)	 - Check the setup and explain how to fix problems
* [synctoceph fleet](synctoceph_fleet.md)	 - Show the latest sync of every machine in the archive
* [synctoceph history](synctoceph_history.md)	 - List and restore previous versions of replaced files
* [synctoceph init](synctoceph_init.md)	 - Create the configuration for this computer
* [synctoceph logs](synctoceph_logs.md)	 - Show recent log lines
* [synctoceph run](synctoceph_run.md)	 - Copy new files to the archive and verify them
* [synctoceph schedule](synctoceph_schedule.md)	 - Run syncs on the configured schedule (used by the service)
* [synctoceph service](synctoceph_service.md)	 - Set up, remove or check automatic runs
* [synctoceph status](synctoceph_status.md)	 - Show what synctoceph is doing and the last result
* [synctoceph stop](synctoceph_stop.md)	 - Stop a running sync or scheduler gracefully
* [synctoceph verify](synctoceph_verify.md)	 - Re-check archived files with SHA-256
* [synctoceph version](synctoceph_version.md)	 - Show the version

