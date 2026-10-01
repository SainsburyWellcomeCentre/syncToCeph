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

Running it again reinstalls.

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

## Update

```
cd syncToCeph
./update.sh
```

`update.sh`:

1. stops if the folder has local changes, and explains the choices (stash,
   commit or discard); nothing is changed;
2. runs `git pull --ff-only`;
3. stops running synctoceph systemd services (a copy in progress is stopped
   gracefully and resumes on the next run);
4. runs `./install.sh` with the same prefix as before;
5. starts the stopped services again.

Windows Task Scheduler tasks start the new program at their next run.

## Uninstall

```
./uninstall.sh            # program, completions and automatic runs
./uninstall.sh --purge    # also configuration, state (logs) and the Go build cache
```

`uninstall.sh` removes the automatic runs of every profile and exactly the
files listed in the install record. It never touches the archive or the
source, and it prints what it kept and where. Delete the cloned folder
yourself afterwards if you no longer need it.
