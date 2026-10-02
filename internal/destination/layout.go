// Package destination manages everything synctoceph keeps on the destination
// (the mounted ceph share, or another mounted network drive): where each
// copied file goes, the verified-file record (manifest), the history of
// replaced files, and run summaries. This file defines where each of those
// lives and creates the folders safely.
//
// The destination is organised by animal. The source holds one folder per
// animal; each acquisition machine (profile) has its own subfolder name, such
// as "behaviour" or "ephys", and writes only into that subfolder of each
// animal folder:
//
//	<source>/LUMS0014/session1/a.bin
//	  -> <destination>/LUMS0014/behaviour/session1/a.bin
//
// so several machines can fill the same animal folder without colliding.
// synctoceph's own records for one subfolder are kept in one place:
//
//	<destination>/.syncToCeph/<subfolder>/
//	  lock                same-computer guard
//	  manifest.jsonl      verified-file record
//	  history/<run-id>/   previous versions of replaced files
//	  runs/<run-id>.json  run summaries (runs/latest.json for fleet)
//
// Paths in these records are relative to the source (LUMS0014/session1/a.bin).
package destination

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Reserved names. A source entry with either name is skipped and reported.
const (
	MetaName    = ".syncToCeph"
	PartialName = ".syncToCeph-partial"
)

// MetaRoot returns the folder holding the records of every subfolder.
func MetaRoot(dest string) string { return filepath.Join(dest, MetaName) }

// MetaDir returns the records folder of one subfolder.
func MetaDir(dest, subfolder string) string { return filepath.Join(MetaRoot(dest), subfolder) }

// HistoryRoot returns the folder holding one subfolder per run that replaced files.
func HistoryRoot(metaDir string) string { return filepath.Join(metaDir, "history") }

// HistoryDir returns where a run keeps the previous versions of files it replaced.
func HistoryDir(metaDir, runID string) string {
	return filepath.Join(HistoryRoot(metaDir), runID)
}

// RunsDir returns the folder of run summaries.
func RunsDir(metaDir string) string { return filepath.Join(metaDir, "runs") }

// ManifestPath returns the path of the verified-file record.
func ManifestPath(metaDir string) string { return filepath.Join(metaDir, "manifest.jsonl") }

// LockPath returns the path of the same-computer destination lock.
func LockPath(metaDir string) string { return filepath.Join(metaDir, "lock") }

// SplitAnimal splits a source-relative path into the animal folder and the
// rest: "LUMS0014/session1/a.bin" gives "LUMS0014" and "session1/a.bin".
// rest is empty for a path directly in the source folder.
func SplitAnimal(rel string) (animal, rest string) {
	animal, rest, _ = strings.Cut(rel, "/")
	return animal, rest
}

// Rel returns where a source file goes, relative to the destination: the
// subfolder is put just below the animal folder.
func Rel(subfolder, rel string) string {
	animal, rest := SplitAnimal(rel)
	return filepath.Join(animal, subfolder, rest)
}

// NewRunID returns a new run ID. It starts with the UTC time so IDs sort in
// time order, and ends with random letters so two IDs never collide.
func NewRunID(now time.Time) string {
	random := make([]byte, 3)
	rand.Read(random)
	return now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(random)
}

// folderMode is the permission for folders synctoceph creates on the
// destination (further limited by the user's umask). Others in the lab can
// read the copied data and the run summaries, which `fleet` relies on.
const folderMode = 0o755

// EnsureMetaDirs creates the records folder of a subfolder (and its runs and
// history folders) if they are missing. The destination itself must already
// exist: it is never created.
func EnsureMetaDirs(dest, subfolder string) error {
	// SAFETY: invariant 6 (destination never created): only single folders below an
	// existing destination are made, one level at a time, never MkdirAll.
	if info, err := os.Lstat(dest); err != nil || !info.IsDir() {
		return fmt.Errorf("destination %s is missing or not a folder", dest)
	}
	meta := MetaDir(dest, subfolder)
	for _, dir := range []string{MetaRoot(dest), meta, RunsDir(meta), HistoryRoot(meta)} {
		if err := mkdirOne(dir); err != nil {
			return err
		}
	}
	return nil
}

// MakeAnimalDirs creates <dest>/<animal>/<subfolder> if missing and returns
// it. The animal folder may already exist, made by another machine.
func MakeAnimalDirs(dest, animal, subfolder string) (string, error) {
	// SAFETY: invariant 6 (destination never created): one level at a time below
	// the existing destination.
	if info, err := os.Lstat(dest); err != nil || !info.IsDir() {
		return "", fmt.Errorf("destination %s is missing or not a folder", dest)
	}
	animalDir := filepath.Join(dest, animal)
	dir := filepath.Join(animalDir, subfolder)
	for _, d := range []string{animalDir, dir} {
		if err := mkdirOne(d); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// mkdirOne creates one folder whose parent exists, and checks that the result
// is a real folder and not a symlink.
func mkdirOne(dir string) error {
	err := os.Mkdir(dir, folderMode)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	// SAFETY: invariant 8 (symlinks on destination paths rejected).
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("checking %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a folder (it may be a symlink or a file)", dir)
	}
	return nil
}

// MakeHistoryDir creates the history folder for one run, and the folder for
// one animal inside it, and returns the latter. rsync puts each replaced
// file there under its path inside the animal folder.
func MakeHistoryDir(metaDir, runID, animal string) (string, error) {
	run := HistoryDir(metaDir, runID)
	if err := mkdirOne(run); err != nil {
		return "", err
	}
	dir := filepath.Join(run, animal)
	return dir, mkdirOne(dir)
}
