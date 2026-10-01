## synctoceph status

Show what synctoceph is doing and the last result

### Synopsis

Shows whether a run or the scheduler is active, the result of the last run,
the next scheduled run, and files that are deferred or differ from the
archive. Reads only; it never starts anything.

```
synctoceph status [flags]
```

### Options

```
      --deferred    list files left for a later run
      --differing   list files in the archive that differ from the source and were not replaced
  -h, --help        help for status
      --json        print machine-readable JSON
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

