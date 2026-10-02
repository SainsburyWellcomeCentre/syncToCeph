// This file reads the latest run summary of every subfolder recorded on a
// destination, for `synctoceph fleet`. It only reads; it never writes to the
// destination.
package destination

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// SubfolderReport is the latest known state of one subfolder (usually one
// acquisition machine).
type SubfolderReport struct {
	Subfolder string      `json:"subfolder"`
	Latest    *RunSummary `json:"latest,omitempty"`
	Error     string      `json:"error,omitempty"`
}

// Fleet returns a report for every subfolder recorded under
// <dest>/.syncToCeph/ that has run summaries, sorted by name.
func Fleet(dest string) ([]SubfolderReport, error) {
	if info, err := os.Stat(dest); err != nil || !info.IsDir() {
		if err == nil {
			err = errors.New("not a folder")
		}
		return nil, fmt.Errorf("opening %s: %w", dest, err)
	}
	entries, err := os.ReadDir(MetaRoot(dest))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", MetaRoot(dest), err)
	}
	var reports []SubfolderReport
	for _, entry := range entries {
		// Only real folders count; symlinks are never followed.
		if !entry.IsDir() {
			continue
		}
		meta := filepath.Join(MetaRoot(dest), entry.Name())
		if !runsDirExists(meta) {
			continue
		}
		report := SubfolderReport{Subfolder: entry.Name()}
		if latest, err := ReadLatest(meta); err != nil {
			report.Error = err.Error()
		} else {
			report.Latest = &latest
		}
		reports = append(reports, report)
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].Subfolder < reports[j].Subfolder })
	return reports, nil
}
