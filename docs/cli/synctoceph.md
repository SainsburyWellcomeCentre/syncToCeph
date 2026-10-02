[Home](../../README.md) · [All documentation](../README.md)

## synctoceph

Copy acquisition data to ceph with rsync, verified with SHA-256

### Synopsis

synctoceph copies data from this acquisition computer to the lab's ceph
storage (or another mounted network drive), using rsync. Run it once, or let it
run in the background on a schedule.

The source holds one folder per animal. Each animal folder is copied into the
animal's folder on ceph, inside this machine's subfolder:
  <source>/<animal>/...  ->  <destination>/<animal>/<subfolder>/...
so other acquisition machines can fill other subfolders of the same animal.

synctoceph only adds files: it never deletes or moves anything, on ceph or on
this computer. A file is reported as copied only after the SHA-256 of the
source and of the ceph copy match.

Start with: synctoceph init, then synctoceph doctor, then synctoceph run --dry-run.

To copy several folders (each with its own source, destination or subfolder),
give each its own profile: synctoceph --profile NAME init. Run them all with
synctoceph run --all-profiles.

### Options

```
  -h, --help           help for synctoceph
      --no-color       never use colour (NO_COLOR is also honoured)
      --profile NAME   use the profile (job) NAME; its settings are in ~/.config/synctoceph/NAME.toml (default "default")
  -q, --quiet          print only the result and errors
  -v, --verbose        list every file copied and verified; --verbose=false turns it off (default: the config's verbose setting, true unless changed)
```

### SEE ALSO

* [synctoceph check-copied](synctoceph_check-copied.md)	 - Check whether files are safely on ceph before deleting them
* [synctoceph completion](synctoceph_completion.md)	 - Print a shell completion script (bash or zsh)
* [synctoceph doctor](synctoceph_doctor.md)	 - Check the setup and explain how to fix problems
* [synctoceph fleet](synctoceph_fleet.md)	 - Show the latest sync of every machine copying to the destination
* [synctoceph history](synctoceph_history.md)	 - List and restore previous versions of replaced files
* [synctoceph init](synctoceph_init.md)	 - Create the configuration for this computer
* [synctoceph logs](synctoceph_logs.md)	 - Show recent log lines
* [synctoceph run](synctoceph_run.md)	 - Copy new files to ceph and verify them
* [synctoceph schedule](synctoceph_schedule.md)	 - Run syncs on the configured schedule (used by the service)
* [synctoceph service](synctoceph_service.md)	 - Set up, remove or check automatic runs
* [synctoceph status](synctoceph_status.md)	 - Show what synctoceph is doing and the last result
* [synctoceph stop](synctoceph_stop.md)	 - Stop a running sync or scheduler gracefully
* [synctoceph verify](synctoceph_verify.md)	 - Re-check copied files with SHA-256
* [synctoceph version](synctoceph_version.md)	 - Show the version

