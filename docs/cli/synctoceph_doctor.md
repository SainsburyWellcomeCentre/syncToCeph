## synctoceph doctor

Check the setup and explain how to fix problems

### Synopsis

Checks the configuration, rsync, the source folder, the archive mount (and its
filesystem type), the archive folder, the state folder, the time zone and the
service. Each problem is printed with how to fix it. Nothing is changed.

Exit code 0 means no problems were found.

```
synctoceph doctor [flags]
```

### Options

```
  -h, --help   help for doctor
      --json   print machine-readable JSON
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

