// This file lets people browse and restore previous versions of files. When
// a run in "replace" mode copies over a file, rsync first moves the old
// version to <destination>/.syncToCeph/<subfolder>/history/<run-id>/<path>,
// where <path> is the file's path in the source (animal folder first).
// History is never pruned automatically. Restoring copies an old version to
// a folder outside the destination; it never overwrites anything.
package destination

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/platform"
)

// runIDPattern matches the run IDs made by NewRunID.
var runIDPattern = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z-[0-9a-f]{6}$`)

// ValidRunID reports whether s looks like a run ID.
func ValidRunID(s string) bool { return runIDPattern.MatchString(s) }

// HistoryRun describes the previous versions kept by one run.
type HistoryRun struct {
	RunID string `json:"run_id"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
}

// HistoryFile is one previous version.
type HistoryFile struct {
	Path  string    `json:"path"`
	Size  int64     `json:"size"`
	MTime time.Time `json:"mtime"`
}

// ListHistory returns the runs that kept previous versions, oldest first.
// Runs whose history folder is empty (nothing was replaced) are left out.
func ListHistory(metaDir string) ([]HistoryRun, error) {
	root := HistoryRoot(metaDir)
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", root, err)
	}
	var runs []HistoryRun
	for _, entry := range entries {
		if !entry.IsDir() || !ValidRunID(entry.Name()) {
			continue
		}
		files, err := ListHistoryFiles(metaDir, entry.Name())
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			continue
		}
		run := HistoryRun{RunID: entry.Name(), Files: len(files)}
		for _, f := range files {
			run.Bytes += f.Size
		}
		runs = append(runs, run)
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].RunID < runs[j].RunID })
	return runs, nil
}

// ListHistoryFiles lists the previous versions kept by one run.
func ListHistoryFiles(metaDir, runID string) ([]HistoryFile, error) {
	if !ValidRunID(runID) {
		return nil, fmt.Errorf("%q is not a run ID", runID)
	}
	root := HistoryDir(metaDir, runID)
	var files []HistoryFile
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		files = append(files, HistoryFile{Path: rel, Size: info.Size(), MTime: info.ModTime()})
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no history for run %s in %s", runID, HistoryRoot(metaDir))
	}
	return files, err
}

// Restore copies the previous version of rel (a file or a folder, as a path
// in the source such as LUMS0014/session1) kept by run runID to toDir/rel.
// toDir must be outside dest, and existing files are never overwritten. Each
// copy is checked with SHA-256.
func Restore(dest, metaDir, runID, rel, toDir string) ([]string, error) {
	rel = filepath.Clean(rel)
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, "../") {
		return nil, fmt.Errorf("%s must be a path relative to the source folder, such as ANIMAL/session1", rel)
	}
	if err := checkOutside(toDir, dest); err != nil {
		return nil, err
	}
	files, err := ListHistoryFiles(metaDir, runID)
	if err != nil {
		return nil, err
	}
	var restored []string
	for _, f := range files {
		if rel != "." && f.Path != rel && !strings.HasPrefix(f.Path, rel+"/") {
			continue
		}
		target := filepath.Join(toDir, f.Path)
		if err := copyVerified(filepath.Join(HistoryDir(metaDir, runID), f.Path), target, f.MTime); err != nil {
			return restored, err
		}
		restored = append(restored, target)
	}
	if len(restored) == 0 {
		return nil, fmt.Errorf("run %s kept no previous version of %s", runID, rel)
	}
	return restored, nil
}

// checkOutside refuses a restore folder inside the destination, so a restore
// can never overwrite or mix with the copied files.
func checkOutside(toDir, dest string) error {
	resolved := toDir
	for probe := toDir; ; probe = filepath.Dir(probe) {
		if real, err := filepath.EvalSymlinks(probe); err == nil {
			rest, _ := filepath.Rel(probe, toDir)
			resolved = filepath.Join(real, rest)
			break
		}
		if probe == filepath.Dir(probe) {
			break
		}
	}
	root := dest
	if real, err := filepath.EvalSymlinks(dest); err == nil {
		root = real
	}
	if platform.Within(filepath.Clean(resolved), root) {
		return fmt.Errorf("the restore folder %s is inside the destination %s; choose a folder outside it", toDir, dest)
	}
	return nil
}

// copyVerified copies src to a new file dst (failing if dst exists), keeps
// the modification time, and checks both with SHA-256.
func copyVerified(src, dst string, mtime time.Time) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(dst), err)
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("creating %s (existing files are never overwritten): %w", dst, err)
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, h), in)
	if err == nil {
		err = out.Sync()
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("copying %s to %s: %w", src, dst, err)
	}
	os.Chtimes(dst, mtime, mtime)
	copied, err := hashFile(dst)
	if err != nil {
		return err
	}
	if string(h.Sum(nil)) != string(copied) {
		return fmt.Errorf("the restored copy %s does not match %s (SHA-256)", dst, src)
	}
	return nil
}

// hashFile returns the SHA-256 of a file.
func hashFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return h.Sum(nil), nil
}
