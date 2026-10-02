// Tests for a whole run with the real rsync, in temporary folders only.
package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
)

// testSettings returns settings for a fresh source, destination and state
// folder. Source files go in animal folders, such as src/A1/x.
func testSettings(t *testing.T) config.Settings {
	t.Helper()
	root := t.TempDir()
	s := config.Settings{Profile: "default", Subfolder: "behaviour",
		Source: filepath.Join(root, "src"), Destination: filepath.Join(root, "ceph"),
		StateDir: filepath.Join(root, "state"), Existing: config.ExistingSkip, Verify: config.VerifyNew}
	s.MetaDir = destination.MetaDir(s.Destination, s.Subfolder)
	for _, dir := range []string{s.Source, s.Destination, s.StateDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// dest returns where the source file rel is copied to.
func dest(s config.Settings, rel string) string {
	return filepath.Join(s.Destination, s.DestRel(rel))
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
	return Run(context.Background(), Options{Settings: s, RunID: destination.NewRunID(time.Now())})
}

func TestRunCopiesAndVerifiesUnusualNamesLiterally(t *testing.T) {
	s := testSettings(t)
	names := []string{"A1/plain.txt", "A1/with space.tif", "A1/new\nline.dat", `A1/quote"s and 'single'.txt`,
		"A1/$(touch pwned).txt", "-leading dash/-x.txt", "A1/semi;colon&amp.txt", "ünïcödé/файл.bin", "A 2/sub dir/*glob?.txt"}
	for i, name := range names {
		write(t, filepath.Join(s.Source, name), strings.Repeat("x", i+1))
	}
	out := runSync(t, s)
	if out.Summary.Result != destination.ResultOK {
		t.Fatalf("result %s, errors %v", out.Summary.Result, out.Summary.Errors)
	}
	if out.Summary.Copied != len(names) || out.Summary.Verified != len(names) {
		t.Fatalf("copied %d verified %d, want %d", out.Summary.Copied, out.Summary.Verified, len(names))
	}
	for _, name := range names {
		if read(t, dest(s, name)) != read(t, filepath.Join(s.Source, name)) {
			t.Errorf("%q differs on the destination", name)
		}
	}
	if _, err := os.Stat(filepath.Join(s.Source, "A1", "pwned")); err == nil {
		t.Fatal("a file name was run as a shell command")
	}
	m, err := destination.LoadManifest(s.MetaDir)
	if err != nil || len(m.Entries) != len(names) {
		t.Fatalf("manifest has %d entries (%v), want %d", len(m.Entries), err, len(names))
	}
}

// SAFETY: invariant 1: files deleted from the source stay on the destination.
func TestSourceDeletionNeverDeletesFromDestination(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "A1", "a", "keep.txt"), "keep")
	write(t, filepath.Join(s.Source, "A2", "b.txt"), "b")
	runSync(t, s)
	os.RemoveAll(filepath.Join(s.Source, "A1"))
	if out := runSync(t, s); out.Summary.Result != destination.ResultOK {
		t.Fatalf("second run: %s %v", out.Summary.Result, out.Summary.Errors)
	}
	if read(t, dest(s, "A1/a/keep.txt")) != "keep" {
		t.Fatal("a file on the destination was deleted")
	}
}

// Each animal folder goes to <destination>/<animal>/<subfolder>/, next to
// other machines' subfolders, which are never touched. Files directly in the
// source belong to no animal and are skipped and reported.
func TestAnimalFoldersGetThisMachinesSubfolder(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "LUMS0014", "session1", "a.bin"), "a")
	write(t, filepath.Join(s.Source, "LUMS0015", "b.bin"), "b")
	write(t, filepath.Join(s.Source, "loose.txt"), "no animal")
	write(t, filepath.Join(s.Destination, "LUMS0014", "ephys", "probe.bin"), "another machine")
	out := runSync(t, s)
	if out.Summary.Result != destination.ResultOK || out.Summary.Verified != 2 {
		t.Fatalf("got %s %+v", out.Summary.Result, out.Summary)
	}
	if read(t, filepath.Join(s.Destination, "LUMS0014", "behaviour", "session1", "a.bin")) != "a" ||
		read(t, filepath.Join(s.Destination, "LUMS0015", "behaviour", "b.bin")) != "b" {
		t.Fatal("files are not in <animal>/<subfolder>/")
	}
	if read(t, filepath.Join(s.Destination, "LUMS0014", "ephys", "probe.bin")) != "another machine" {
		t.Fatal("another machine's subfolder was changed")
	}
	if out.Summary.SkippedCount != 1 || out.Summary.Skipped[0].Path != "loose.txt" {
		t.Fatalf("a file outside an animal folder must be skipped: %+v", out.Summary.Skipped)
	}
	if _, err := os.Stat(filepath.Join(s.Destination, "loose.txt")); err == nil {
		t.Fatal("a file outside an animal folder was copied")
	}
}

// SAFETY: invariant 3: replace mode keeps the previous version in history.
func TestReplaceKeepsPreviousVersionInHistory(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "A1", "data.bin"), "version one")
	runSync(t, s)
	write(t, filepath.Join(s.Source, "A1", "data.bin"), "version two!")
	if out := runSync(t, s); out.Summary.DifferingCount != 1 || read(t, dest(s, "A1/data.bin")) != "version one" {
		t.Fatalf("skip mode must leave the destination copy alone: %+v", out.Summary)
	}
	s.Existing = config.ExistingReplace
	out := runSync(t, s)
	if out.Summary.Result != destination.ResultOK || out.Summary.Replaced != 1 {
		t.Fatalf("replace run: %s %+v", out.Summary.Result, out.Summary.Errors)
	}
	if read(t, dest(s, "A1/data.bin")) != "version two!" {
		t.Fatal("the destination copy was not replaced")
	}
	kept := filepath.Join(destination.HistoryDir(s.MetaDir, out.Summary.RunID), "A1", "data.bin")
	if read(t, kept) != "version one" {
		t.Fatal("previous version not kept in history")
	}
}

// SAFETY: invariant 4: a copy whose content differs is never reported as verified.
func TestVerifyAllDetectsSilentCorruption(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "A1", "a.dat"), "correct")
	runSync(t, s)
	target := dest(s, "A1/a.dat")
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
	if out.Summary.Result != destination.ResultOK || read(t, target) != "correct" {
		t.Fatalf("replace --verify all did not repair the copy: %s %v", out.Summary.Result, out.Summary.Errors)
	}
	if read(t, filepath.Join(destination.HistoryDir(s.MetaDir, out.Summary.RunID), "A1", "a.dat")) != "CORRUPT" {
		t.Fatal("the damaged copy must be kept in history, not deleted")
	}
}

// SAFETY: invariant 4: a source file that changes after the scan is deferred, not copied.
func TestChangedOrVanishedSourceIsDeferred(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "A1", "a.dat"), "first")
	write(t, filepath.Join(s.Source, "A1", "b.dat"), "first")
	scan, err := ScanSource(context.Background(), s.Source, "", nil, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	write(t, dest(s, "A1/a.dat"), "first")
	os.WriteFile(filepath.Join(s.Source, "A1", "a.dat"), []byte("second"), 0o644)
	os.Remove(filepath.Join(s.Source, "A1", "b.dat"))
	for _, f := range scan.Files {
		c := VerifyFile(context.Background(), s, f)
		want := map[string]string{"A1/a.dat": destination.ReasonChanged, "A1/b.dat": destination.ReasonVanished}[f.Path]
		if c.SHA256 != "" || c.Defer != want {
			t.Errorf("%s: got %+v, want deferral %q", f.Path, c, want)
		}
	}
}

// SAFETY: invariant 6: a missing destination is an error and is never created.
func TestMissingDestinationIsNeverCreated(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "A1", "a"), "a")
	os.Remove(s.Destination)
	out := runSync(t, s)
	if out.Summary.Result != destination.ResultFailed {
		t.Fatalf("result %s, want FAILED", out.Summary.Result)
	}
	if _, err := os.Lstat(s.Destination); !os.IsNotExist(err) {
		t.Fatal("the destination was created")
	}
}

// SAFETY: invariant 6: require_mount must be a mount point.
func TestUnmountedRequiredMountStopsTheRun(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "A1", "a"), "a")
	s.RequireMount = filepath.Dir(s.Destination) // a plain folder, not a mount point
	out := runSync(t, s)
	if out.Summary.Result != destination.ResultFailed || !strings.Contains(strings.Join(out.Summary.Errors, " "), "is not mounted") {
		t.Fatalf("got %s %v", out.Summary.Result, out.Summary.Errors)
	}
	if entries, _ := os.ReadDir(s.Destination); len(entries) != 0 {
		t.Fatal("wrote into the destination although the mount was missing")
	}
}

// SAFETY: invariant 7: nested source, destination or state folders are refused.
func TestNestedPathsAreRefused(t *testing.T) {
	for _, nest := range []string{"destination-in-source", "state-in-destination", "source-in-destination"} {
		s := testSettings(t)
		switch nest {
		case "destination-in-source":
			s.Destination = filepath.Join(s.Source, "ceph")
		case "state-in-destination":
			s.StateDir = filepath.Join(s.Destination, "state")
		case "source-in-destination":
			s.Source = filepath.Join(s.Destination, "src")
		}
		os.MkdirAll(s.Destination, 0o700)
		os.MkdirAll(s.Source, 0o700)
		s.MetaDir = destination.MetaDir(s.Destination, s.Subfolder)
		if err := CheckSeparate(s); err == nil {
			t.Errorf("%s: not refused", nest)
		}
	}
}

// SAFETY: invariant 8: symlinks and type conflicts on the destination (in an
// animal folder, its subfolder, or below) block the copy of those files only;
// nothing outside the destination is written.
func TestDestinationSymlinksAndTypeConflictsAreRejected(t *testing.T) {
	s := testSettings(t)
	outside := t.TempDir()
	write(t, filepath.Join(s.Source, "A1", "linked", "a.txt"), "a")
	write(t, filepath.Join(s.Source, "A1", "conflict", "b.txt"), "b")
	write(t, filepath.Join(s.Source, "A1", "fine.txt"), "fine")
	write(t, filepath.Join(s.Source, "A2", "c.txt"), "c")
	write(t, filepath.Join(s.Source, "A3", "d.txt"), "d")
	os.MkdirAll(filepath.Join(s.Destination, "A1", "behaviour"), 0o755)
	os.Symlink(outside, filepath.Join(s.Destination, "A1", "behaviour", "linked"))
	write(t, filepath.Join(s.Destination, "A1", "behaviour", "conflict"), "a file where the source has a folder")
	os.Symlink(outside, filepath.Join(s.Destination, "A2"))                      // a symlinked animal folder
	write(t, filepath.Join(s.Destination, "A3"), "a file named like the animal") // a file in its place
	out := runSync(t, s)
	if out.Summary.Result != destination.ResultFailed || out.Summary.ErrorCount != 4 {
		t.Fatalf("got %s %v", out.Summary.Result, out.Summary.Errors)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatal("a file was written through a symlink")
	}
	if read(t, filepath.Join(s.Destination, "A1", "behaviour", "conflict")) != "a file where the source has a folder" {
		t.Fatal("the conflicting file on the destination was changed")
	}
	if read(t, dest(s, "A1/fine.txt")) != "fine" {
		t.Fatal("other files must still be copied")
	}
	s.Destination = filepath.Join(t.TempDir(), "via-link")
	os.Symlink(filepath.Dir(s.MetaDir), s.Destination)
	s.MetaDir = destination.MetaDir(s.Destination, s.Subfolder)
	if _, err := Preflight(s); err == nil {
		t.Fatal("a destination path through a symlink must be refused")
	}
}

// SAFETY: invariant 9: a dry run writes nothing to the destination.
func TestDryRunWritesNothing(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "A1", "a"), "a")
	s.DryRun = true
	out := runSync(t, s)
	if out.Summary.Result != destination.ResultOK || len(out.Summary.WouldCopy) != 1 {
		t.Fatalf("dry run: %+v", out.Summary)
	}
	if entries, _ := os.ReadDir(s.Destination); len(entries) != 0 {
		t.Fatalf("dry run wrote %v", entries)
	}
}

func TestSettlingFilesAreDeferred(t *testing.T) {
	s := testSettings(t)
	s.SettleTime = 10 * time.Minute
	write(t, filepath.Join(s.Source, "A1", "old.dat"), "old")
	os.WriteFile(filepath.Join(s.Source, "A1", "fresh.dat"), []byte("fresh"), 0o644)
	out := runSync(t, s)
	if out.Summary.Result != destination.ResultPartial || out.Summary.DeferredCount != 1 ||
		out.Summary.Deferred[0].Path != "A1/fresh.dat" || out.Summary.ExitCode != destination.ExitPartial {
		t.Fatalf("got %+v", out.Summary)
	}
	if _, err := os.Stat(dest(s, "A1/fresh.dat")); err == nil {
		t.Fatal("a settling file was copied")
	}
}

// Progress events report each file once, after rsync copied it and after its
// SHA-256 matched, so the terminal never shows a file as verified early.
func TestEventsFollowTheRun(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "A1", "a.txt"), "alpha")
	write(t, filepath.Join(s.Source, "A2", "dir", "b.txt"), "beta")
	var kinds []string
	files := map[string][]string{}
	out := Run(context.Background(), Options{Settings: s, RunID: destination.NewRunID(time.Now()),
		Event: func(e Event) {
			kinds = append(kinds, e.Kind)
			if e.Path != "" {
				files[e.Kind] = append(files[e.Kind], e.Path)
			}
			if e.Kind == EventVerified && e.Total != 2 {
				t.Errorf("verified %s as %d of %d, want a total of 2", e.Path, e.Done, e.Total)
			}
		}})
	if out.Summary.Result != destination.ResultOK {
		t.Fatalf("result %s: %v", out.Summary.Result, out.Summary.Errors)
	}
	want := "scanned planned copied copied verified verified"
	if got := strings.Join(kinds, " "); got != want {
		t.Fatalf("events %q, want %q", got, want)
	}
	if len(files[EventCopied]) != 2 || len(files[EventVerified]) != 2 {
		t.Fatalf("file events: %v", files)
	}
}
