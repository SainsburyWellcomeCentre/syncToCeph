// This file reads the latest run summary of every machine folder in an
// archive, for `synctoceph fleet`. It only reads; it never writes to the
// archive.
package archive

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MachineReport is the latest known state of one machine.
type MachineReport struct {
	Machine string      `json:"machine_name"`
	Latest  *RunSummary `json:"latest,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Fleet returns a report for every machine folder under archiveRoot that
// has run summaries, sorted by machine name.
func Fleet(archiveRoot string) ([]MachineReport, error) {
	entries, err := os.ReadDir(archiveRoot)
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", archiveRoot, err)
	}
	var reports []MachineReport
	for _, entry := range entries {
		// Only real folders are machine folders; symlinks are never followed.
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		machineDir := filepath.Join(archiveRoot, entry.Name())
		if !runsDirExists(machineDir) {
			continue
		}
		report := MachineReport{Machine: entry.Name()}
		if latest, err := ReadLatest(machineDir); err != nil {
			report.Error = err.Error()
		} else {
			report.Latest = &latest
		}
		reports = append(reports, report)
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].Machine < reports[j].Machine })
	return reports, nil
}
