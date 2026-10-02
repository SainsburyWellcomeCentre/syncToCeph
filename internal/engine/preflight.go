// Package engine performs one sync: check the setup (preflight), scan the
// source, plan what to copy, run rsync, and verify every copy with SHA-256.
//
// This file is the preflight: the checks that run before anything is
// written. If any check fails, the run stops with an explanation and the
// destination is not touched.
package engine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/platform"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// MinRsync is the oldest rsync that has every option synctoceph relies on
// (--fsync arrived in rsync 3.2.4).
var MinRsync = [3]int{3, 2, 4}

// rsyncVersionPattern finds the version in `rsync --version` output.
var rsyncVersionPattern = regexp.MustCompile(`rsync\s+version\s+v?([0-9]+)\.([0-9]+)\.([0-9]+)`)

// RsyncVersion runs `rsync --version` and returns the executable's path, its
// version text, and whether it is new enough.
func RsyncVersion() (path, version string, ok bool, err error) {
	path, err = exec.LookPath("rsync")
	if err != nil {
		return "", "", false, ui.RsyncMissing()
	}
	out, err := exec.Command(path, "--version").Output()
	m := rsyncVersionPattern.FindSubmatch(out)
	if err != nil || m == nil {
		return path, "unknown", false, ui.RsyncUnknown(path)
	}
	var v [3]int
	for i := range v {
		v[i], _ = strconv.Atoi(string(m[i+1]))
	}
	version = fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2])
	ok = v[0] > MinRsync[0] || v[0] == MinRsync[0] && (v[1] > MinRsync[1] || v[1] == MinRsync[1] && v[2] >= MinRsync[2])
	return path, version, ok, nil
}

// Preflight checks the setup and returns the path of rsync. It never writes.
func Preflight(s config.Settings) (string, error) {
	if err := CheckMount(s); err != nil {
		return "", err
	}
	// SAFETY: invariant 8 (symlinks on destination paths rejected): checked from
	// the filesystem root down to the records folder; animal folders are
	// checked file by file when planning.
	if err := destination.NoSymlinks(s.MetaDir); err != nil {
		return "", ui.DestinationSymlink(s.Destination, err)
	}
	// SAFETY: invariant 6 (destination never created): a missing destination is an
	// error, never something to create.
	info, err := os.Lstat(s.Destination)
	if err != nil || !info.IsDir() {
		return "", ui.DestinationMissing(s.Destination, err)
	}
	for _, dir := range []string{destination.MetaRoot(s.Destination), s.MetaDir} {
		if info, err := os.Lstat(dir); err == nil && !info.IsDir() {
			return "", ui.MetaDirNotFolder(dir)
		}
	}
	if info, err := os.Stat(s.Source); err != nil || !info.IsDir() {
		return "", ui.SourceMissing(s.Source, err)
	}
	if err := CheckSeparate(s); err != nil {
		return "", err
	}
	path, version, ok, err := RsyncVersion()
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ui.RsyncTooOld(path, version)
	}
	return path, nil
}

// CheckMount checks require_mount: it must be a mount point, the destination
// must be inside it, and on the same filesystem. It reads only the mount
// table until the mount is known to be present.
func CheckMount(s config.Settings) error {
	if s.RequireMount == "" {
		return nil
	}
	// SAFETY: invariant 6 (never write into an empty mount point).
	if !platform.Within(s.Destination, s.RequireMount) {
		return ui.MountNotContainingDestination(s.RequireMount, s.Destination)
	}
	mounts, err := platform.ReadMounts()
	if err != nil {
		return ui.MountTableUnreadable(err)
	}
	if !platform.IsMountPoint(mounts, s.RequireMount) {
		return ui.NotMounted(s.RequireMount, s.Destination)
	}
	if err := destination.NoSymlinks(s.RequireMount); err != nil {
		return ui.DestinationSymlink(s.RequireMount, err)
	}
	mountInfo, err1 := os.Stat(s.RequireMount)
	destInfo, err2 := os.Stat(s.Destination)
	if err1 != nil || err2 != nil {
		return ui.DestinationMissing(s.Destination, errors.Join(err1, err2))
	}
	if deviceOf(mountInfo) != deviceOf(destInfo) {
		return ui.DestinationOnOtherFilesystem(s.RequireMount, s.Destination)
	}
	return nil
}

// deviceOf returns the filesystem ID of a stat result.
func deviceOf(info os.FileInfo) uint64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Dev)
	}
	return 0
}

// CheckSeparate checks that source, destination and state folders are
// separate: none may be inside another (after following symlinks), so the
// tool can never copy the destination into itself or its own state into the
// destination.
func CheckSeparate(s config.Settings) error {
	// SAFETY: invariant 7 (source, destination and state are separate, non-nested folders).
	named := []struct{ name, path string }{
		{"source", s.Source}, {"destination", s.Destination}, {"state folder", s.StateDir},
	}
	for i := range named {
		named[i].path = resolve(named[i].path)
	}
	for i := 0; i < len(named); i++ {
		for j := i + 1; j < len(named); j++ {
			a, b := named[i], named[j]
			if platform.Within(a.path, b.path) || platform.Within(b.path, a.path) {
				return ui.PathsOverlap(a.name, a.path, b.name, b.path)
			}
		}
	}
	return nil
}

// resolve follows symlinks in the longest existing part of path.
func resolve(path string) string {
	probe, rest := path, ""
	for {
		if real, err := filepath.EvalSymlinks(probe); err == nil {
			return filepath.Join(real, rest)
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return path
		}
		rest = filepath.Join(filepath.Base(probe), rest)
		probe = parent
	}
}
