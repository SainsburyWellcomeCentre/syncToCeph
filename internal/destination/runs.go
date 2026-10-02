// This file saves run summaries into a subfolder's records folder
// (<destination>/.syncToCeph/<subfolder>/runs/<run-id>.json and
// runs/latest.json) and reads them back. They let anyone with access to the
// destination see how each acquisition machine's syncs are going, without
// logging in to that machine (`synctoceph fleet`).
package destination

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/platform"
)

// latestName is the file that always holds the most recent run summary.
const latestName = "latest.json"

// WriteRunSummary saves a run summary as runs/<run-id>.json and as
// runs/latest.json. Long file lists are shortened (see Capped).
func WriteRunSummary(metaDir string, s RunSummary) error {
	dir := RunsDir(metaDir)
	if err := NoSymlinks(dir); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.Capped(), "", "  ")
	if err != nil {
		return fmt.Errorf("encoding run summary: %w", err)
	}
	data = append(data, '\n')
	if err := platform.WriteFileAtomic(filepath.Join(dir, s.RunID+".json"), data, 0o644); err != nil {
		return err
	}
	return platform.WriteFileAtomic(filepath.Join(dir, latestName), data, 0o644)
}

// ReadLatest reads runs/latest.json of a records folder.
func ReadLatest(metaDir string) (RunSummary, error) {
	var s RunSummary
	f, err := OpenRegular(RunsDir(metaDir), latestName)
	if err != nil {
		return s, err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return s, fmt.Errorf("reading %s: %w", filepath.Join(RunsDir(metaDir), latestName), err)
	}
	return s, nil
}

// runsDirExists reports whether a records folder has any run summaries.
func runsDirExists(metaDir string) bool {
	info, err := os.Lstat(RunsDir(metaDir))
	return err == nil && info.IsDir()
}
