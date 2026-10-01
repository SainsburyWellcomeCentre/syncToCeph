[Home](../../README.md) · [All documentation](../README.md) · [All commands](synctoceph.md)

## synctoceph verify

Re-check archived files with SHA-256

### Synopsis

Reads each source file under PATH (default: the whole source) and its archive
copy, compares their SHA-256, and records the ones that match as verified.
Use it once for data copied before synctoceph was used, or to check the
archive at any time. Exit code 0 means every file under PATH was verified.

```
synctoceph verify [PATH] [flags]
```

### Options

```
  -h, --help   help for verify
      --json   print machine-readable JSON
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

