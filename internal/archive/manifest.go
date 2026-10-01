// This file manages the verified-file record (manifest.jsonl): one line per
// verification, saying that a file of this size and modification time had
// the same SHA-256 in the source and in the archive. `check-archived` relies
// on it to say whether a source file is safe to delete. Lines are only ever
// appended; when the file holds many outdated lines it is rewritten with the
// latest line per file (compacted), atomically.
package archive

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/platform"
)

// ManifestSchemaVersion is the version of each manifest line's JSON format.
const ManifestSchemaVersion = 1

// TimeTolerance is how far apart two modification times may be and still
// count as equal. Windows drives and some network filesystems store times
// with less precision than Linux, so exact comparison would report
// differences that do not exist.
const TimeTolerance = time.Second

// compactMinLines and compactRatio decide when the manifest is compacted: once
// it has at least compactMinLines lines and more than compactRatio lines per
// file it describes.
const (
	compactMinLines = 10000
	compactRatio    = 2
)

// Entry is one line of the manifest.
type Entry struct {
	SchemaVersion int       `json:"schema_version"`
	Path          string    `json:"path"`
	Size          int64     `json:"size"`
	MTime         time.Time `json:"mtime"`
	SHA256        string    `json:"sha256"`
	VerifiedAt    time.Time `json:"verified_at"`
	RunID         string    `json:"run_id"`
}

// Matches reports whether the entry describes a file of this size and
// modification time.
func (e Entry) Matches(size int64, mtime time.Time) bool {
	return e.Size == size && SameTime(e.MTime, mtime)
}

// SameTime reports whether two modification times are equal within
// TimeTolerance.
func SameTime(a, b time.Time) bool {
	d := a.Sub(b)
	return d > -TimeTolerance && d < TimeTolerance
}

// Manifest maps a relative path to its latest verification.
type Manifest struct {
	Entries map[string]Entry
	Lines   int // lines in the file, including outdated ones
}

// LoadManifest reads the manifest of a machine folder. A missing manifest is
// empty. Damaged lines (for example a line cut short by a crash) are skipped.
func LoadManifest(machineDir string) (Manifest, error) {
	m := Manifest{Entries: map[string]Entry{}}
	f, err := OpenRegular(MetaDir(machineDir), "manifest.jsonl")
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, fmt.Errorf("opening %s: %w", ManifestPath(machineDir), err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		m.Lines++
		var e Entry
		if json.Unmarshal(scanner.Bytes(), &e) != nil || e.Path == "" || e.SHA256 == "" {
			continue
		}
		m.Entries[e.Path] = e
	}
	if err := scanner.Err(); err != nil {
		return m, fmt.Errorf("reading %s: %w", ManifestPath(machineDir), err)
	}
	return m, nil
}

// ManifestWriter appends entries to the manifest.
type ManifestWriter struct {
	file *os.File
	out  *bufio.Writer
}

// OpenManifestWriter opens the manifest for appending, creating it if needed.
func OpenManifestWriter(machineDir string) (*ManifestWriter, error) {
	path := ManifestPath(machineDir)
	if err := NoSymlinks(path); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	return &ManifestWriter{file: f, out: bufio.NewWriter(f)}, nil
}

// Append adds one verified file. Call it only after the SHA-256 of the
// source and the archive copy have been compared and found equal.
func (w *ManifestWriter) Append(e Entry) error {
	e.SchemaVersion = ManifestSchemaVersion
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("encoding manifest entry: %w", err)
	}
	_, err = w.out.Write(append(data, '\n'))
	return err
}

// Close flushes the entries to disk and closes the file.
func (w *ManifestWriter) Close() error {
	if w == nil || w.file == nil {
		return nil
	}
	err := w.out.Flush()
	if syncErr := w.file.Sync(); err == nil {
		err = syncErr
	}
	if closeErr := w.file.Close(); err == nil {
		err = closeErr
	}
	w.file = nil
	if err != nil {
		return fmt.Errorf("saving the manifest: %w", err)
	}
	return nil
}

// NeedsCompaction reports whether the manifest has grown enough outdated
// lines to be worth rewriting.
func (m Manifest) NeedsCompaction() bool {
	return m.Lines >= compactMinLines && m.Lines > compactRatio*len(m.Entries)
}

// Compact rewrites the manifest with only the latest entry for each file.
// The new file replaces the old one atomically, so no record is ever lost.
func Compact(machineDir string) error {
	m, err := LoadManifest(machineDir)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(m.Entries))
	for path := range m.Entries {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var data []byte
	for _, path := range paths {
		line, err := json.Marshal(m.Entries[path])
		if err != nil {
			return fmt.Errorf("encoding manifest entry: %w", err)
		}
		data = append(append(data, line...), '\n')
	}
	return platform.WriteFileAtomic(ManifestPath(machineDir), data, 0o644)
}
