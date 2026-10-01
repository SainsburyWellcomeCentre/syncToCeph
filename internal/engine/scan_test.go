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

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/archive"
)

// SAFETY: invariant 8: symlinks and special files in the source are skipped and
// reported, never followed.
func TestScanSkipsSymlinksSpecialFilesAndReservedNames(t *testing.T) {
	s := testSettings(t)
	outside := t.TempDir()
	write(t, filepath.Join(outside, "secret.txt"), "not part of the source")
	write(t, filepath.Join(s.Source, "real.txt"), "real")
	os.Symlink(outside, filepath.Join(s.Source, "dirlink"))
	os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(s.Source, "filelink"))
	if err := syscall.Mkfifo(filepath.Join(s.Source, "pipe"), 0o644); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(s.Source, archive.MetaName, "x"), "reserved")
	write(t, filepath.Join(s.Source, "sub", archive.PartialName), "reserved")
	scan, err := ScanSource(context.Background(), s.Source, "", nil, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Files) != 1 || scan.Files[0].Path != "real.txt" {
		t.Fatalf("files: %+v", scan.Files)
	}
	var skipped []string
	for _, sk := range scan.Skipped {
		skipped = append(skipped, sk.Path)
	}
	sort.Strings(skipped)
	want := []string{archive.MetaName, "dirlink", "filelink", "pipe", "sub/" + archive.PartialName}
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
		want         bool
	}{
		{"Thumbs.db", "Thumbs.db", true},
		{"a/b/Thumbs.db", "Thumbs.db", true},
		{"a/~$report.docx", "~$*", true},
		{"a/report.docx", "~$*", false},
		{"raw/scratch", "raw/scratch", true},
		{"other/raw/scratch", "raw/scratch", false},
		{"x.tmp", "*.tmp", true},
		{"x.tmp.keep", "*.tmp", false},
	}
	for _, tt := range tests {
		if got := excluded(tt.rel, filepath.Base(tt.rel), []string{tt.pattern}); got != tt.want {
			t.Errorf("excluded(%q, %q) = %t", tt.rel, tt.pattern, got)
		}
	}
}

func TestExcludedFoldersAreNotEntered(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "keep", "a"), "a")
	write(t, filepath.Join(s.Source, "scratch", "b"), "b")
	scan, _ := ScanSource(context.Background(), s.Source, "", []string{"scratch"}, 0, time.Now())
	if len(scan.Files) != 1 || scan.Excluded != 1 || scan.ExcludedPaths[0] != "scratch" {
		t.Fatalf("got %+v", scan)
	}
}

func TestPlanClassifiesFiles(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "new"), "new")
	write(t, filepath.Join(s.Source, "same"), "same")
	write(t, filepath.Join(s.Source, "differs"), "source version")
	write(t, filepath.Join(s.MachineDir, "differs"), "archive")
	src, _ := os.Stat(filepath.Join(s.Source, "same"))
	write(t, filepath.Join(s.MachineDir, "same"), "same")
	os.Chtimes(filepath.Join(s.MachineDir, "same"), src.ModTime(), src.ModTime())
	scan, _ := ScanSource(context.Background(), s.Source, "", nil, 0, time.Now())
	plan := MakePlan(s, scan.Files, archive.Manifest{Entries: map[string]archive.Entry{}})
	if len(plan.Copy) != 1 || plan.Copy[0].Path != "new" {
		t.Errorf("copy: %+v", plan.Copy)
	}
	if len(plan.Matching) != 1 || plan.Matching[0].Path != "same" || plan.Verified["same"] {
		t.Errorf("matching: %+v", plan.Matching)
	}
	if len(plan.Differing) != 1 || plan.Differing[0] != "differs" {
		t.Errorf("differing: %+v", plan.Differing)
	}
}
