[Home](../README.md)

# synctoceph documentation

New to synctoceph? Read the pages in this order: install it, configure it,
make sure ceph is mounted, then set up automatic runs. Every
page links back here and on to the next one.

| Page | What it covers |
|---|---|
| [Installation](installation.md) | Install, update to a new version, reinstall, uninstall, and remove everything (`--purge`) |
| [Configuration](configuration.md) | How data is organised (animal folder, then one subfolder per machine), the config file and its keys, profiles, flags |
| [Mounting ceph](mounting.md) | Mounting ceph inside WSL or Linux with `/etc/fstab` and a credentials file, every option explained |
| [Scheduling](scheduling.md) | Automatic runs with systemd or Windows Task Scheduler |
| [Operations](operations.md) | Day-to-day use: status, logs, stopping, recovery, exit codes, JSON output |
| [Troubleshooting](troubleshooting.md) | Every error message and what to do about it |
| [Safety model](safety-model.md) | What synctoceph guarantees about your data, and what it does not |
| [Design](design.md) | The full specification: how a run works, step by step |
| [Command reference](cli/synctoceph.md) | Every command and flag (generated from the program) |

For people changing the tool: [AGENTS.md](../AGENTS.md) (the developer
guide) and the [changelog](../CHANGELOG.md).

---

[Home](../README.md) · Start reading: [Installation](installation.md)
