## synctoceph fleet

Show the latest sync of every machine in the archive

### Synopsis

Reads the run summary that every machine writes to its folder in the archive
and shows each machine's last run, last successful sync, deferred files and
problems. Reads only. Uses the archive from this profile's config unless
--archive is given.

```
synctoceph fleet [flags]
```

### Options

```
      --archive string   archive root to read (default: from the config)
  -h, --help             help for fleet
      --json             print machine-readable JSON
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

