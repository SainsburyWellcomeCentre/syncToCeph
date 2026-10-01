## synctoceph stop

Stop a running sync or scheduler gracefully

### Synopsis

Asks the running synctoceph for this profile to stop. rsync is asked to stop
(SIGINT) and keeps its partial file so the next run can resume; if it has not
stopped after --timeout seconds it is killed. An interrupted run is never
reported as verified.

```
synctoceph stop [flags]
```

### Options

```
  -h, --help          help for stop
      --timeout int   seconds rsync gets to stop before it is killed (default 30)
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

