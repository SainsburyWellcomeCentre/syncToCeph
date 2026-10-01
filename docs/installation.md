[Home](../README.md) · [All documentation](README.md)

# Installation

synctoceph runs on Linux and on Windows through WSL2. It is installed from a
clone of the GitHub repository with three scripts: `install.sh`, `update.sh`
and `uninstall.sh`. No package manager is used.

## Requirements

- Linux, or Windows 10/11 with WSL2 (Ubuntu or another distribution).
- `git`, `curl` (or `wget`), `tar` and `sha256sum` (present on most systems).
- rsync 3.2.4 or newer. Check with `rsync --version`. On Ubuntu or Debian:
  `sudo apt update && sudo apt install rsync`.
- Network access to github.com and go.dev during installation.

Go does not need to be installed. If Go of the exact version in `go.mod` is not
on `PATH`, `install.sh` downloads the official Go release from go.dev, checks it
against a SHA-256 pinned in the script, and keeps it in
`~/.cache/synctoceph/` for building only.

## Install

```
git clone https://github.com/SainsburyWellcomeCentre/syncToCeph.git
cd syncToCeph
./install.sh
```

`install.sh` announces each step. It:

1. finds or downloads Go (see above);
2. builds synctoceph with the version, commit and build date stamped in
   (`synctoceph version` shows them);
3. installs `~/.local/bin/synctoceph` and shell completions for bash
   (`~/.local/share/bash-completion/completions/synctoceph`) and zsh
   (`~/.local/share/zsh/site-functions/_synctoceph`);
4. records the installed files in `~/.local/share/synctoceph/installed-files.txt`;
5. warns if `~/.local/bin` is not on your `PATH` and shows the line to add;
6. checks rsync and prints the install command if it is missing or too old;
7. runs `synctoceph doctor` and prints the next steps.

Running it again reinstalls (see [Update to a new version](#update-to-a-new-version)).

To install elsewhere, for example for all users:

```
./install.sh --prefix /usr/local
```

`sudo` is used only for copying into a folder you cannot write to.

Then set up the computer:

```
synctoceph init
synctoceph doctor
synctoceph run --dry-run
synctoceph run
synctoceph service install
```

## Update to a new version

synctoceph is installed from the folder you cloned. To add the features of a
newer version to an existing installation, update that folder and rebuild:

```
cd syncToCeph            # the folder you ran ./install.sh from
./update.sh
synctoceph version       # check the version now installed
```

`update.sh`:

1. stops if the folder has local changes, and explains the choices (stash,
   commit or discard); nothing is changed;
2. runs `git pull --ff-only` to download the new version;
3. stops running synctoceph systemd services (a copy in progress is stopped
   gracefully and resumes on the next run);
4. runs `./install.sh` with the same prefix as before;
5. starts the stopped services again.

Windows Task Scheduler tasks start the new program at their next run.

What is kept: the configuration of every profile, the state and logs, the
automatic runs, and of course the archive. If a new version adds a config
setting, it has a default, so existing config files keep working;
`synctoceph doctor` checks them. New settings are listed in the
[changelog](../CHANGELOG.md) and in [configuration.md](configuration.md).

Other situations:

| Situation | What to do |
|---|---|
| You no longer have the cloned folder | Clone it again (see [Install](#install)) and run `./install.sh`. The settings in `~/.config/synctoceph/` are used as they are. |
| You changed the code yourself | Run `./install.sh` again. It rebuilds from the folder as it is, without downloading anything. |
| `update.sh` reports local changes | Follow its advice: `git stash`, then `./update.sh`, then `git stash pop` to keep your changes. |
| You want a clean reinstall | `./uninstall.sh`, then `./install.sh`. Settings and logs are kept. |
| You want to start from scratch | `./uninstall.sh --purge`, then `./install.sh` and `synctoceph init`. |

## Uninstall

```
./uninstall.sh                 # program, completions and automatic runs
./uninstall.sh --purge         # also every config file, the state and logs, and the Go build cache
./uninstall.sh --purge --yes   # the same, without the question (for scripts)
```

`uninstall.sh` removes the automatic runs of every profile and exactly the
files listed in the install record. Without `--purge` it keeps your settings
and logs, so a later `./install.sh` carries on where you left off.

With `--purge` it first lists everything it will delete (the folders
`~/.config/synctoceph/`, `~/.local/state/synctoceph/`,
`~/.cache/synctoceph/` and `~/.local/share/synctoceph/`, and the profile of
each config file) and asks you to type `yes`. Answering anything else changes
nothing. When it is not run in a terminal, it needs `--yes`. It refuses to
delete the state while a run is still active (stop it first with
`synctoceph --profile NAME stop`).

Neither form ever touches the archive, including its `.syncToCeph/` folder
(the verified-file record, run summaries and the history of replaced files),
or the source data. Delete the cloned folder yourself afterwards if you no
longer need it.

---

Next: [Configuration](configuration.md) · [All documentation](README.md) · [Home](../README.md)
