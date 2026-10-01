[Home](../README.md) · [All documentation](README.md)

# Scheduling

Set a schedule in the config file, then install the service:

```toml
interval = "4h"      # every 4 hours, counted from the end of the previous run
# at     = "02:00"   # or daily at 02:00 local time
```

```
synctoceph service install
synctoceph service status
synctoceph status
```

How the timing works (overlaps, missed runs, daylight saving) is described in
[design.md](design.md#scheduling).

## Which service manager is used

`synctoceph service install` picks one automatically; override it with
`--manager systemd` or `--manager task-scheduler`. Add `--print` to see what
would be set up without changing anything.

Each profile gets its own service or task. `service install --all-profiles`
sets up every profile that has a schedule and lists the ones it skipped;
`service uninstall --all-profiles` removes them all.

| Computer | Manager | What runs |
|---|---|---|
| Linux with systemd | systemd user service `synctoceph-<profile>.service` | `synctoceph schedule --profile <profile>`, which stays running |
| WSL with systemd enabled | systemd user service (as above) | as above; see the WSL notes below |
| WSL without systemd | Windows Task Scheduler task `synctoceph-<profile>` | `wsl.exe -d <distro> -- synctoceph run --profile <profile> --quiet` at each scheduled time |

synctoceph never uses `sudo`. When something needs administrator rights, it
prints the command for you to run.

## systemd (Linux)

The unit is written to `~/.config/systemd/user/synctoceph-<profile>.service`,
then enabled and started with `systemctl --user enable --now`. It restarts
after failures and stops gracefully: systemd sends SIGTERM, synctoceph asks
rsync to stop, and systemd waits up to 60 seconds.

User services stop when you log out unless *lingering* is on. If it is off,
`service install` says so; ask an administrator to run:

```
sudo loginctl enable-linger <your user name>
```

Logs: `synctoceph logs` (the service's own output is also in
`journalctl --user -u synctoceph-<profile>`).

Remove with `synctoceph service uninstall` (add `--all` for every profile).

## WSL

WSL starts systemd only if `/etc/wsl.conf` contains:

```
[boot]
systemd=true
```

(then run `wsl.exe --shutdown` from Windows and open WSL again).

### Windows Task Scheduler (WSL without systemd)

`service install` runs `schtasks.exe` to create the task. Intervals must be
whole minutes (up to 23 hours), whole hours below 24, or whole days; `at`
becomes a daily task. Daily times follow the Windows time zone, not `TZ` in
WSL.

By default the task runs only while you are logged in to Windows. To run it
when nobody is logged in, open Task Scheduler, find the task
`synctoceph-<profile>`, and choose "Run whether user is logged on or not"
(Windows asks for your password).

Task Scheduler does not start a second copy while one is running, and the
synctoceph lock prevents overlap as well.

### Checks still to be done on a real WSL machine

These questions from the design brief affect how reliable scheduling is on
WSL. They could not be verified during development (it requires closing every
WSL terminal, logging off Windows, and a real lab share). Record the answers
here when checked.

| Question | Status | How to check |
|---|---|---|
| Does WSL keep a systemd service running when no WSL terminal is open? | Not yet verified. WSL is known to shut down an idle distribution after a timeout; if it does, the service stops too. | Install the service, close all WSL terminals, wait an hour, reopen, and run `synctoceph status` and `synctoceph logs`. |
| Are Windows mapped drives visible to scheduled runs? | Not yet verified. A drive mapped by a user session may be missing for a task running without that session. | Mount the share inside WSL (see [mounting.md](mounting.md)); with the task set to "run whether user is logged on or not", log off and check `synctoceph logs` the next day. |

Until these are confirmed, prefer mounting the share inside WSL with an
`/etc/fstab` entry, and check `synctoceph status` or `synctoceph fleet`
regularly.

---

Previous: [Mounting the share](mounting.md) · Next: [Operations](operations.md) · [All documentation](README.md) · [Home](../README.md)
