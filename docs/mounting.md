[Home](../README.md) · [All documentation](README.md)

# Mounting the lab share

synctoceph writes to a share that is already mounted. It never mounts anything
itself and never uses `sudo`. Set `require_mount` in the config to the mount
point, so a run stops instead of writing into an empty folder when the share
is not mounted:

```toml
archive       = "/mnt/z/lab-archive"
require_mount = "/mnt/z"
```

`synctoceph doctor` checks the mount and shows its filesystem type, read from
`/proc/self/mountinfo`. If the mount is missing, it prints the commands below.

## Inside WSL or Linux (CephFS, cifs/SMB, NFS)

Ask your IT team for the server, share name and mount options. A typical SMB
mount:

```
sudo mkdir -p /mnt/z
sudo mount -t cifs //SERVER/SHARE /mnt/z -o credentials=/root/.smbcred,uid=$(id -u),gid=$(id -g)
```

`/root/.smbcred` holds `username=...` and `password=...` lines and should be
readable only by root. To mount at every start, add a line to `/etc/fstab`:

```
//SERVER/SHARE /mnt/z cifs credentials=/root/.smbcred,uid=YOUR_UID,gid=YOUR_GID,nofail 0 0
```

`uid` and `gid` (from `id -u` and `id -g`) make the files yours, so
synctoceph can write them.

## A Windows mapped drive (e.g. Z:)

If the share is mapped as a drive letter in Windows, it is not visible in WSL
until it is mounted with drvfs:

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
  is not yet verified; see [scheduling.md](scheduling.md).

## The source on a Windows drive

Data drives of Windows acquisition PCs appear in WSL as `/mnt/<letter>`, for
example `/mnt/d/acquisition`. They need no setup. synctoceph only reads them.

## The state folder

synctoceph's own state (status, log, lock) must stay on the Linux disk
(`~/.local/state/synctoceph/`). It refuses to use a Windows drive or a network
share for it.

---

Previous: [Configuration](configuration.md) · Next: [Scheduling](scheduling.md) · [All documentation](README.md) · [Home](../README.md)
