// This file answers questions about mounts using the mount table: which mount
// holds a folder, whether a folder is a mount point, and whether a mount is a
// Windows drive or a network share. It works on paths as text and never
// opens or lists the mounted drives.
package platform

import (
	"path/filepath"
	"strings"
)

// Mount is one entry of the mount table.
type Mount struct {
	Point   string // where it is mounted, e.g. /mnt/z
	FSType  string // filesystem type, e.g. ext4, 9p, drvfs, cifs, nfs4, ceph
	Source  string // what is mounted, e.g. Z:\ or //server/share
	Options string // filesystem options, e.g. aname=drvfs;path=Z:\
}

// MountFor returns the mount that contains path (the mount point that is the
// longest prefix of it). path must be absolute and should have symlinks
// resolved. ok is false if no mount matches.
func MountFor(mounts []Mount, path string) (m Mount, ok bool) {
	path = filepath.Clean(path)
	best := -1
	for _, candidate := range mounts {
		if !Within(path, candidate.Point) {
			continue
		}
		// ">=" so a later mount on the same point (mounted on top) wins.
		if len(candidate.Point) >= best {
			best, m, ok = len(candidate.Point), candidate, true
		}
	}
	return m, ok
}

// IsMountPoint reports whether path is exactly a mount point in the table.
func IsMountPoint(mounts []Mount, path string) bool {
	m, ok := MountFor(mounts, path)
	return ok && m.Point == filepath.Clean(path)
}

// IsWindowsDrive reports whether m is a Windows drive seen from WSL (drvfs,
// which newer WSL versions show as a 9p mount with aname=drvfs).
func IsWindowsDrive(m Mount) bool {
	return m.FSType == "drvfs" || (m.FSType == "9p" && strings.Contains(m.Options, "aname=drvfs"))
}

// networkTypes are filesystem types that live on another computer. autofs
// is the placeholder systemd puts at a mount point with x-systemd.automount
// (as recommended for ceph in docs/mounting.md) until the share is first used.
var networkTypes = map[string]bool{
	"cifs": true, "smb3": true, "smbfs": true, "nfs": true, "nfs4": true,
	"ceph": true, "fuse.ceph-fuse": true, "fuse.sshfs": true, "9p": true,
	"afs": true, "glusterfs": true, "fuse.glusterfs": true, "lustre": true,
	"gpfs": true, "beegfs": true, "autofs": true,
}

// IsNetwork reports whether m is a network share or a Windows drive rather
// than a local Linux disk.
func IsNetwork(m Mount) bool {
	return networkTypes[m.FSType] || IsWindowsDrive(m)
}

// Within reports whether path is dir or inside it. Both must be clean,
// absolute paths; the comparison is on the text only.
func Within(path, dir string) bool {
	if dir == "/" {
		return strings.HasPrefix(path, "/")
	}
	return path == dir || strings.HasPrefix(path, dir+"/")
}

// DriveLetter returns the Windows drive letter for a WSL path such as
// /mnt/z or /mnt/z/lab-data, or "" if the path is not of that form.
func DriveLetter(path string) string {
	parts := strings.Split(strings.TrimPrefix(filepath.Clean(path), "/"), "/")
	if len(parts) >= 2 && parts[0] == "mnt" && len(parts[1]) == 1 {
		c := parts[1][0]
		if 'a' <= c && c <= 'z' {
			return strings.ToUpper(parts[1])
		}
	}
	return ""
}
