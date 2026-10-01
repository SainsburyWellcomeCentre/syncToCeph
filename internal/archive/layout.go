// Package archive manages everything synctoceph keeps inside a machine's
// folder on the archive: the verified-file record (manifest), the history of
// replaced files, and run summaries. This file defines where each of those
// lives and creates the folders safely.
//
// Layout of one machine folder:
//
//	<archive>/<machine_name>/
//	  <copied data...>
//	  .syncToCeph/
//	    archive.lock        same-machine guard
//	    manifest.jsonl      verified-file record
//	    history/<run-id>/   previous versions of replaced files
//	    runs/<run-id>.json  run summaries (runs/latest.json for fleet)
package archive

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Reserved names. A source entry with either name is skipped and reported.
const (
	MetaName    = ".syncToCeph"
	PartialName = ".syncToCeph-partial"
)

// MetaDir returns the metadata folder of a machine folder.
func MetaDir(machineDir string) string { return filepath.Join(machineDir, MetaName) }

// HistoryRoot returns the folder holding one subfolder per run that replaced files.
func HistoryRoot(machineDir string) string { return filepath.Join(MetaDir(machineDir), "history") }

// HistoryDir returns where a run keeps the previous versions of files it replaced.
func HistoryDir(machineDir, runID string) string {
	return filepath.Join(HistoryRoot(machineDir), runID)
}

// RunsDir returns the folder of run summaries.
func RunsDir(machineDir string) string { return filepath.Join(MetaDir(machineDir), "runs") }

// ManifestPath returns the path of the verified-file record.
func ManifestPath(machineDir string) string {
	return filepath.Join(MetaDir(machineDir), "manifest.jsonl")
}

// LockPath returns the path of the same-machine archive lock.
func LockPath(machineDir string) string { return filepath.Join(MetaDir(machineDir), "archive.lock") }

// NewRunID returns a new run ID. It starts with the UTC time so IDs sort in
// time order, and ends with random letters so two IDs never collide.
func NewRunID(now time.Time) string {
	random := make([]byte, 3)
	rand.Read(random)
	return now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(random)
}

// metaFolderMode is the permission for folders synctoceph creates in the
// archive (further limited by the user's umask). Others in the lab can read
// run summaries, which `fleet` relies on.
const metaFolderMode = 0o755

// EnsureMetaDirs creates the machine folder and its .syncToCeph, runs and
// history folders if they are missing. The archive root itself must already
// exist: it is never created.
func EnsureMetaDirs(archiveRoot, machineDir string) error {
	// SAFETY: invariant 6 (archive root never created): only single folders below an
	// existing archive root are made, one level at a time, never MkdirAll.
	if info, err := os.Lstat(archiveRoot); err != nil || !info.IsDir() {
		return fmt.Errorf("archive root %s is missing or not a folder", archiveRoot)
	}
	for _, dir := range []string{machineDir, MetaDir(machineDir), RunsDir(machineDir), HistoryRoot(machineDir)} {
		if err := mkdirOne(dir); err != nil {
			return err
		}
	}
	return nil
}

// mkdirOne creates one folder whose parent exists, and checks that the result
// is a real folder and not a symlink.
func mkdirOne(dir string) error {
	err := os.Mkdir(dir, metaFolderMode)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	// SAFETY: invariant 8 (symlinks on archive paths rejected).
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("checking %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a folder (it may be a symlink or a file)", dir)
	}
	return nil
}

// MakeHistoryDir creates the history folder for one run.
func MakeHistoryDir(machineDir, runID string) (string, error) {
	dir := HistoryDir(machineDir, runID)
	return dir, mkdirOne(dir)
}
