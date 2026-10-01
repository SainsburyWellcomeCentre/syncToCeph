// Tests for a whole run with the real rsync, in temporary folders only.
package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/archive"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
)

// testSettings returns settings for a fresh source, archive and state folder.
func testSettings(t *testing.T) config.Settings {
	t.Helper()
	root := t.TempDir()
	s := config.Settings{Profile: "default", MachineName: "scope-01",
		Source: filepath.Join(root, "src"), Archive: filepath.Join(root, "archive"),
		StateDir: filepath.Join(root, "state"), Existing: config.ExistingSkip, Verify: config.VerifyNew}
	s.MachineDir = filepath.Join(s.Archive, s.MachineName)
	for _, dir := range []string{s.Source, s.Archive, s.StateDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// write creates a file (and its folders) with content and an old mtime.
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func runSync(t *testing.T, s config.Settings) Outcome {
	t.Helper()
	return Run(context.Background(), Options{Settings: s, RunID: archive.NewRunID(time.Now())})
}

func TestRunCopiesAndVerifiesUnusualNamesLiterally(t *testing.T) {
	s := testSettings(t)
	names := []string{"plain.txt", "with space.tif", "new\nline.dat", `quote"s and 'single'.txt`,
		"$(touch pwned).txt", "-leading-dash.txt", "semi;colon&amp.txt", "ünïcödé/файл.bin", "sub dir/*glob?.txt"}
	for i, name := range names {
		write(t, filepath.Join(s.Source, name), strings.Repeat("x", i+1))
	}
	out := runSync(t, s)
	if out.Summary.Result != archive.ResultOK {
		t.Fatalf("result %s, errors %v", out.Summary.Result, out.Summary.Errors)
	}
	if out.Summary.Copied != len(names) || out.Summary.Verified != len(names) {
		t.Fatalf("copied %d verified %d, want %d", out.Summary.Copied, out.Summary.Verified, len(names))
	}
	for _, name := range names {
		if read(t, filepath.Join(s.MachineDir, name)) != read(t, filepath.Join(s.Source, name)) {
			t.Errorf("%q differs in the archive", name)
		}
	}
	if _, err := os.Stat(filepath.Join(s.Source, "pwned")); err == nil {
		t.Fatal("a file name was run as a shell command")
	}
	m, err := archive.LoadManifest(s.MachineDir)
	if err != nil || len(m.Entries) != len(names) {
		t.Fatalf("manifest has %d entries (%v), want %d", len(m.Entries), err, len(names))
	}
}

// SAFETY: invariant 1: files deleted from the source stay in the archive.
func TestSourceDeletionNeverDeletesFromArchive(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "a", "keep.txt"), "keep")
	runSync(t, s)
	os.RemoveAll(filepath.Join(s.Source, "a"))
	if out := runSync(t, s); out.Summary.Result != archive.ResultOK {
		t.Fatalf("second run: %s %v", out.Summary.Result, out.Summary.Errors)
	}
	if read(t, filepath.Join(s.MachineDir, "a", "keep.txt")) != "keep" {
		t.Fatal("archive file was deleted")
	}
}

// SAFETY: invariant 3: replace mode keeps the previous version in history.
func TestReplaceKeepsPreviousVersionInHistory(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "data.bin"), "version one")
	runSync(t, s)
	write(t, filepath.Join(s.Source, "data.bin"), "version two!")
	if out := runSync(t, s); out.Summary.DifferingCount != 1 || read(t, filepath.Join(s.MachineDir, "data.bin")) != "version one" {
		t.Fatalf("skip mode must leave the archive copy alone: %+v", out.Summary)
	}
	s.Existing = config.ExistingReplace
	out := runSync(t, s)
	if out.Summary.Result != archive.ResultOK || out.Summary.Replaced != 1 {
		t.Fatalf("replace run: %s %+v", out.Summary.Result, out.Summary.Errors)
	}
	if read(t, filepath.Join(s.MachineDir, "data.bin")) != "version two!" {
		t.Fatal("archive copy was not replaced")
	}
	kept := filepath.Join(archive.HistoryDir(s.MachineDir, out.Summary.RunID), "data.bin")
	if read(t, kept) != "version one" {
		t.Fatal("previous version not kept in history")
	}
}

// SAFETY: invariant 4: a copy whose content differs is never reported as verified.
func TestVerifyAllDetectsSilentCorruption(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "a.dat"), "correct")
	runSync(t, s)
	target := filepath.Join(s.MachineDir, "a.dat")
	info, _ := os.Stat(target)
	os.WriteFile(target, []byte("CORRUPT"), 0o644) // same size
	os.Chtimes(target, info.ModTime(), info.ModTime())
	s.Verify = config.VerifyAll
	out := runSync(t, s)
	if out.Summary.DifferingCount != 1 || out.Summary.Verified != 0 {
		t.Fatalf("corruption not detected: %+v", out.Summary)
	}
	s.Existing = config.ExistingReplace
	out = runSync(t, s)
	if out.Summary.Result != archive.ResultOK || read(t, target) != "correct" {
		t.Fatalf("replace --verify all did not repair the copy: %s %v", out.Summary.Result, out.Summary.Errors)
	}
	if read(t, filepath.Join(archive.HistoryDir(s.MachineDir, out.Summary.RunID), "a.dat")) != "CORRUPT" {
		t.Fatal("the damaged copy must be kept in history, not deleted")
	}
}

// SAFETY: invariant 4: a source file that changes after the scan is deferred, not archived.
func TestChangedOrVanishedSourceIsDeferred(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "a.dat"), "first")
	write(t, filepath.Join(s.Source, "b.dat"), "first")
	scan, err := ScanSource(context.Background(), s.Source, "", nil, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(s.MachineDir, "a.dat"), "first")
	os.WriteFile(filepath.Join(s.Source, "a.dat"), []byte("second"), 0o644)
	os.Remove(filepath.Join(s.Source, "b.dat"))
	for _, f := range scan.Files {
		c := VerifyFile(context.Background(), s.Source, s.MachineDir, f)
		want := map[string]string{"a.dat": archive.ReasonChanged, "b.dat": archive.ReasonVanished}[f.Path]
		if c.SHA256 != "" || c.Defer != want {
			t.Errorf("%s: got %+v, want deferral %q", f.Path, c, want)
		}
	}
}

// SAFETY: invariant 6: a missing archive root is an error and is never created.
func TestMissingArchiveRootIsNeverCreated(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "a"), "a")
	os.Remove(s.Archive)
	out := runSync(t, s)
	if out.Summary.Result != archive.ResultFailed {
		t.Fatalf("result %s, want FAILED", out.Summary.Result)
	}
	if _, err := os.Lstat(s.Archive); !os.IsNotExist(err) {
		t.Fatal("archive root was created")
	}
}

// SAFETY: invariant 6: require_mount must be a mount point.
func TestUnmountedRequiredMountStopsTheRun(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "a"), "a")
	s.RequireMount = filepath.Dir(s.Archive) // a plain folder, not a mount point
	out := runSync(t, s)
	if out.Summary.Result != archive.ResultFailed || !strings.Contains(strings.Join(out.Summary.Errors, " "), "is not mounted") {
		t.Fatalf("got %s %v", out.Summary.Result, out.Summary.Errors)
	}
	if _, err := os.Lstat(s.MachineDir); !os.IsNotExist(err) {
		t.Fatal("wrote into the archive although the mount was missing")
	}
}

// SAFETY: invariant 7: nested source, archive or state folders are refused.
func TestNestedPathsAreRefused(t *testing.T) {
	for _, nest := range []string{"archive-in-source", "state-in-archive", "source-in-archive"} {
		s := testSettings(t)
		switch nest {
		case "archive-in-source":
			s.Archive = filepath.Join(s.Source, "archive")
		case "state-in-archive":
			s.StateDir = filepath.Join(s.Archive, "state")
		case "source-in-archive":
			s.Source = filepath.Join(s.Archive, "src")
		}
		os.MkdirAll(s.Archive, 0o700)
		os.MkdirAll(s.Source, 0o700)
		s.MachineDir = filepath.Join(s.Archive, s.MachineName)
		if err := CheckSeparate(s); err == nil {
			t.Errorf("%s: not refused", nest)
		}
	}
}

// SAFETY: invariant 8: symlinks and type conflicts in the archive block the copy of
// that file only; nothing outside the archive is written.
func TestArchiveSymlinksAndTypeConflictsAreRejected(t *testing.T) {
	s := testSettings(t)
	outside := t.TempDir()
	write(t, filepath.Join(s.Source, "linked", "a.txt"), "a")
	write(t, filepath.Join(s.Source, "conflict", "b.txt"), "b")
	write(t, filepath.Join(s.Source, "fine.txt"), "fine")
	os.MkdirAll(s.MachineDir, 0o755)
	os.Symlink(outside, filepath.Join(s.MachineDir, "linked"))
	write(t, filepath.Join(s.MachineDir, "conflict"), "a file where the source has a folder")
	out := runSync(t, s)
	if out.Summary.Result != archive.ResultFailed || out.Summary.ErrorCount != 2 {
		t.Fatalf("got %s %v", out.Summary.Result, out.Summary.Errors)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatal("a file was written through the symlink")
	}
	if read(t, filepath.Join(s.MachineDir, "conflict")) != "a file where the source has a folder" {
		t.Fatal("the conflicting archive file was changed")
	}
	if read(t, filepath.Join(s.MachineDir, "fine.txt")) != "fine" {
		t.Fatal("other files must still be copied")
	}
	s.Archive = filepath.Join(t.TempDir(), "via-link")
	os.Symlink(filepath.Dir(s.MachineDir), s.Archive)
	s.MachineDir = filepath.Join(s.Archive, s.MachineName)
	if _, err := Preflight(s); err == nil {
		t.Fatal("an archive path through a symlink must be refused")
	}
}

// SAFETY: invariant 9: a dry run writes nothing to the archive.
func TestDryRunWritesNothing(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "a"), "a")
	s.DryRun = true
	out := runSync(t, s)
	if out.Summary.Result != archive.ResultOK || len(out.Summary.WouldCopy) != 1 {
		t.Fatalf("dry run: %+v", out.Summary)
	}
	if entries, _ := os.ReadDir(s.Archive); len(entries) != 0 {
		t.Fatalf("dry run wrote %v", entries)
	}
}

func TestSettlingFilesAreDeferred(t *testing.T) {
	s := testSettings(t)
	s.SettleTime = 10 * time.Minute
	write(t, filepath.Join(s.Source, "old.dat"), "old")
	os.WriteFile(filepath.Join(s.Source, "fresh.dat"), []byte("fresh"), 0o644)
	out := runSync(t, s)
	if out.Summary.Result != archive.ResultPartial || out.Summary.DeferredCount != 1 ||
		out.Summary.Deferred[0].Path != "fresh.dat" || out.Summary.ExitCode != archive.ExitPartial {
		t.Fatalf("got %+v", out.Summary)
	}
	if _, err := os.Stat(filepath.Join(s.MachineDir, "fresh.dat")); err == nil {
		t.Fatal("a settling file was copied")
	}
}
