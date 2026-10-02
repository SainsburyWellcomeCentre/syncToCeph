// Tests for scanning the source and planning.
package engine

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"testing"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
)

// SAFETY: invariant 8: symlinks and special files in the source are skipped and
// reported, never followed. Files outside an animal folder are skipped too.
func TestScanSkipsSymlinksSpecialFilesAndReservedNames(t *testing.T) {
	s := testSettings(t)
	outside := t.TempDir()
	write(t, filepath.Join(outside, "secret.txt"), "not part of the source")
	write(t, filepath.Join(s.Source, "A1", "real.txt"), "real")
	write(t, filepath.Join(s.Source, "loose.txt"), "not in an animal folder")
	os.Symlink(outside, filepath.Join(s.Source, "dirlink"))
	os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(s.Source, "filelink"))
	if err := syscall.Mkfifo(filepath.Join(s.Source, "pipe"), 0o644); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(s.Source, destination.MetaName, "x"), "reserved")
	write(t, filepath.Join(s.Source, "sub", destination.PartialName), "reserved")
	scan, err := ScanSource(context.Background(), s.Source, "", nil, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Files) != 1 || scan.Files[0].Path != "A1/real.txt" {
		t.Fatalf("files: %+v", scan.Files)
	}
	var skipped []string
	for _, sk := range scan.Skipped {
		skipped = append(skipped, sk.Path)
	}
	sort.Strings(skipped)
	want := []string{destination.MetaName, "dirlink", "filelink", "loose.txt", "pipe", "sub/" + destination.PartialName}
	if len(skipped) != len(want) {
		t.Fatalf("skipped %v, want %v", skipped, want)
	}
	for i := range want {
		if skipped[i] != want[i] {
			t.Fatalf("skipped %v, want %v", skipped, want)
		}
	}
}

func TestExcludePatterns(t *testing.T) {
	tests := []struct {
		rel, pattern string
		isDir, want  bool
	}{
		{"Thumbs.db", "Thumbs.db", false, true},
		{"a/b/Thumbs.db", "Thumbs.db", false, true},
		{"a/~$report.docx", "~$*", false, true},
		{"a/report.docx", "~$*", false, false},
		{"raw/scratch", "raw/scratch", true, true},
		{"other/raw/scratch", "raw/scratch", true, false},
		{"other/raw/scratch", "/other/raw/scratch", true, true},
		{"x.tmp", "*.tmp", false, true},
		{"x.tmp.keep", "*.tmp", false, false},
		// A trailing / matches folders only, anywhere (or from the source with a /).
		{"A1/tmp", "tmp/", true, true},
		{"A1/tmp", "tmp/", false, false},
		{"A1/s/tmp", "A1/tmp/", true, false},
		{"A1/tmp", "A1/tmp/", true, true},
		// exclude_hidden adds ".*": any name starting with a dot, file or folder.
		{".git", ".*", true, true},
		{"A1/.DS_Store", ".*", false, true},
		{"A1/x.bin", ".*", false, false},
	}
	for _, tt := range tests {
		if got := excluded(tt.rel, filepath.Base(tt.rel), tt.isDir, []string{tt.pattern}); got != tt.want {
			t.Errorf("excluded(%q, %q, folder %t) = %t", tt.rel, tt.pattern, tt.isDir, got)
		}
	}
}

func TestExcludedFoldersAreNotEntered(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "A1", "keep", "a"), "a")
	write(t, filepath.Join(s.Source, "A1", "scratch", "b"), "b")
	scan, _ := ScanSource(context.Background(), s.Source, "", []string{"scratch"}, 0, time.Now())
	if len(scan.Files) != 1 || scan.Excluded != 1 || scan.ExcludedPaths[0] != "A1/scratch" {
		t.Fatalf("got %+v", scan)
	}
}

func TestPlanClassifiesFiles(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "A1", "new"), "new")
	write(t, filepath.Join(s.Source, "A1", "same"), "same")
	write(t, filepath.Join(s.Source, "A1", "differs"), "source version")
	write(t, dest(s, "A1/differs"), "on ceph")
	src, _ := os.Stat(filepath.Join(s.Source, "A1", "same"))
	write(t, dest(s, "A1/same"), "same")
	os.Chtimes(dest(s, "A1/same"), src.ModTime(), src.ModTime())
	// The same name in another machine's subfolder is a different file.
	write(t, filepath.Join(s.Destination, "A1", "ephys", "new"), "other machine")
	scan, _ := ScanSource(context.Background(), s.Source, "", nil, 0, time.Now())
	plan := MakePlan(s, scan.Files, destination.Manifest{Entries: map[string]destination.Entry{}})
	if len(plan.Copy) != 1 || plan.Copy[0].Path != "A1/new" {
		t.Errorf("copy: %+v", plan.Copy)
	}
	if len(plan.Matching) != 1 || plan.Matching[0].Path != "A1/same" || plan.Verified["A1/same"] {
		t.Errorf("matching: %+v", plan.Matching)
	}
	if len(plan.Differing) != 1 || plan.Differing[0] != "A1/differs" {
		t.Errorf("differing: %+v", plan.Differing)
	}
}
