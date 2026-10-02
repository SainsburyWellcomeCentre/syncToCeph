[Home](../README.md) · [All documentation](README.md)

# Mounting ceph

synctoceph copies to a folder on ceph (or another network drive) that is
already mounted on the acquisition computer. It never mounts anything itself
and never uses `sudo`. This page sets up the mount once, inside Linux or WSL.

The lab's ceph storage is reached as an SMB (Windows file sharing) share,
here `//ceph-gw02.hpc.swc.ucl.ac.uk/<share>`, where `<share>` is your lab's
share name (for example `harris`). It is usually mounted at `/mnt/ceph`.

## 1. Install the SMB tools and create the mount point

```
sudo apt install cifs-utils
sudo mkdir -p /mnt/ceph
```

`cifs-utils` provides `mount -t cifs`. The folder `/mnt/ceph` must exist
before anything can be mounted there; it stays empty while ceph is not
mounted.

## 2. Mount it by hand (to try it, or for a one-off copy)

```
sudo mount -t cifs //ceph-gw02.hpc.swc.ucl.ac.uk/<share> /mnt/ceph \
  -o username=<your SWC user name>,domain=ad.swc.ucl.ac.uk,uid=$(id -u),gid=$(id -g),vers=3.0
```

It asks for your password. Check with `ls /mnt/ceph`. This mount lasts until
the computer (or WSL) restarts; `sudo umount /mnt/ceph` removes it.

## 3. Mount it at every start (`/etc/fstab`)

For scheduled runs, ceph should be mounted automatically. That needs a
credentials file (so no one has to type the password) and one line in
`/etc/fstab`.

### The credentials file

Create a file in your home folder, for example `~/.swc_credentials`, with
three lines:

```
username=<your SWC user name>
password=<your password>
domain=ad.swc.ucl.ac.uk
```

`domain` is `ad.swc.ucl.ac.uk` for SWC accounts. Make the file readable only
by you, because it holds your password:

```
chmod 600 ~/.swc_credentials
```

### The fstab line

Open `/etc/fstab` with `sudo nano /etc/fstab` and add this as one line, with
`<share>` and `<you>` filled in:

```
//ceph-gw02.hpc.swc.ucl.ac.uk/<share>  /mnt/ceph  cifs  credentials=/home/<you>/.swc_credentials,uid=1000,gid=1000,_netdev,vers=3.0,nofail,x-systemd.automount,x-systemd.idle-timeout=60,x-systemd.device-timeout=10s  0  0
```

The six fields, separated by spaces:

| Field | Value | Meaning |
|---|---|---|
| 1. What | `//ceph-gw02.hpc.swc.ucl.ac.uk/<share>` | The server and the lab's share on it. |
| 2. Where | `/mnt/ceph` | The mount point created in step 1. |
| 3. Type | `cifs` | An SMB share (needs `cifs-utils`). |
| 4. Options | see the next table | How to mount it, separated by commas with no spaces. |
| 5. Dump | `0` | Not backed up by the old `dump` tool. |
| 6. Check | `0` | Never checked with `fsck` at start-up (only for local disks). |

The options:

| Option | Meaning |
|---|---|
| `credentials=/home/<you>/.swc_credentials` | Read user name, password and domain from the credentials file. Use the full path; `~` does not work here. |
| `uid=1000,gid=1000` | Files on the share appear as owned by your Linux user and group, so synctoceph (running as you) can write them. Use the numbers that `id -u` and `id -g` print. |
| `_netdev` | It is a network drive: wait for the network before mounting. |
| `vers=3.0` | Use version 3.0 of the SMB protocol. |
| `nofail` | Start up normally even if ceph cannot be mounted (for example with no network). |
| `x-systemd.automount` | Mount on first use: the share is mounted the moment something opens `/mnt/ceph`, instead of during start-up. |
| `x-systemd.idle-timeout=60` | Unmount after 60 seconds without use; the next use mounts it again. |
| `x-systemd.device-timeout=10s` | Give up after 10 seconds if the server does not answer. |

Then load the new line and mount it now:

```
sudo systemctl daemon-reload
sudo mount -a
ls /mnt/ceph
```

On WSL, the `x-systemd.*` options take effect only when systemd is enabled
(`systemd=true` under `[boot]` in `/etc/wsl.conf`). Without systemd, WSL
mounts the `/etc/fstab` entries each time it starts, and these options are
ignored.

## 4. Tell synctoceph about the mount

`synctoceph init` suggests `require_mount = "/mnt/ceph"` when the destination
is on ceph. Keep it: with it, a run stops instead of writing into the empty
`/mnt/ceph` folder when ceph is not mounted.

```toml
destination   = "/mnt/ceph/<project>"
require_mount = "/mnt/ceph"
```

`synctoceph doctor` checks the mount and shows its filesystem type, read from
`/proc/self/mountinfo` (`cifs` once mounted; `autofs` is the automount
placeholder of `x-systemd.automount`). If the mount is missing, it prints the
commands above.

## Other network drives

Any network drive mounted inside Linux or WSL works the same way (CephFS,
cifs, NFS): mount it, and set `destination` and `require_mount` to match.
Ask your IT team for the server, share and options.

### A Windows mapped drive (e.g. Z:)

If the share is only mapped as a drive letter in Windows, it is not visible
in WSL until it is mounted with drvfs:

```
sudo mkdir -p /mnt/z
sudo mount -t drvfs Z: /mnt/z
```

To mount it at every start, add to `/etc/fstab`:

```
Z: /mnt/z drvfs defaults 0 0
```

Notes for drvfs:

- Modification times keep only whole seconds; synctoceph allows for this.
- See [troubleshooting.md](troubleshooting.md#windows-drives-drvfs) for what
  was tested on Windows drives.
- Whether a mapped drive is visible to runs started by Windows Task Scheduler
  is not yet verified; see [scheduling.md](scheduling.md). Mounting the share
  inside WSL with cifs (above) avoids the question.

## The source on a Windows drive

Data drives of Windows acquisition PCs appear in WSL as `/mnt/<letter>`, for
example `/mnt/d/luminoseData`. They need no setup. synctoceph only reads them.

## The state folder

synctoceph's own state (status, log, lock) must stay on the Linux disk
(`~/.local/state/synctoceph/`). It refuses to use a Windows drive or a network
share for it.

---

Previous: [Configuration](configuration.md) · Next: [Scheduling](scheduling.md) · [All documentation](README.md) · [Home](../README.md)
