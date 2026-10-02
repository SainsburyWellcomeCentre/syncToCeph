// Tests for the layout, manifest, history, run summaries and fleet.
package destination

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManifestAppendLoadAndCompact(t *testing.T) {
	root := t.TempDir()
	if err := EnsureMetaDirs(root, "behaviour"); err != nil {
		t.Fatal(err)
	}
	meta := MetaDir(root, "behaviour")
	mtime := time.Date(2026, 1, 1, 0, 0, 0, 123456789, time.UTC)
	w, err := OpenManifestWriter(meta)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		w.Append(Entry{Path: "a", Size: int64(i), MTime: mtime, SHA256: "aa", RunID: "r"})
	}
	w.Append(Entry{Path: "b", Size: 1, MTime: mtime, SHA256: "bb", RunID: "r"})
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f, _ := os.OpenFile(ManifestPath(meta), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(`{"path":"cut short`) // as after a crash mid-write
	f.Close()
	m, err := LoadManifest(meta)
	if err != nil || m.Lines != 5 || len(m.Entries) != 2 || m.Entries["a"].Size != 2 {
		t.Fatalf("got %+v %v", m, err)
	}
	if !m.Entries["a"].Matches(2, mtime.Add(500*time.Millisecond)) || m.Entries["a"].Matches(2, mtime.Add(2*time.Second)) {
		t.Fatal("time tolerance is wrong")
	}
	if err := Compact(meta); err != nil {
		t.Fatal(err)
	}
	m, _ = LoadManifest(meta)
	if m.Lines != 2 || m.Entries["a"].Size != 2 || m.Entries["b"].SHA256 != "bb" {
		t.Fatalf("compaction lost data: %+v", m)
	}
}

// SAFETY: invariant 6: EnsureMetaDirs and MakeAnimalDirs never create the destination.
func TestFoldersNeedTheDestination(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing-root")
	if err := EnsureMetaDirs(root, "behaviour"); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := MakeAnimalDirs(root, "LUMS0014", "behaviour"); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("destination was created")
	}
}

// Each meta writes only into its own subfolder of each animal folder.
func TestAnimalFirstLayout(t *testing.T) {
	if got := Rel("behaviour", "LUMS0014/session1/a.bin"); got != "LUMS0014/behaviour/session1/a.bin" {
		t.Fatalf("Rel = %s", got)
	}
	if a, rest := SplitAnimal("LUMS0014/x"); a != "LUMS0014" || rest != "x" {
		t.Fatalf("SplitAnimal = %s %s", a, rest)
	}
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "LUMS0014", "ephys"), 0o755) // made by another meta
	dir, err := MakeAnimalDirs(root, "LUMS0014", "behaviour")
	if err != nil || dir != filepath.Join(root, "LUMS0014", "behaviour") {
		t.Fatalf("got %s %v", dir, err)
	}
	if _, err := os.Stat(filepath.Join(root, "LUMS0014", "ephys")); err != nil {
		t.Fatal("another meta's subfolder was touched")
	}
}

// SAFETY: invariant 8: an animal folder that is a symlink is never written through.
func TestMakeAnimalDirsRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	os.Symlink(outside, filepath.Join(root, "LUMS0014"))
	if _, err := MakeAnimalDirs(root, "LUMS0014", "behaviour"); err == nil {
		t.Fatal("a symlinked animal folder must be refused")
	}
	if _, err := os.Lstat(filepath.Join(outside, "behaviour")); !os.IsNotExist(err) {
		t.Fatal("a folder was created through the symlink")
	}
}

// SAFETY: invariant 8: NoSymlinks finds a symlink anywhere in the path.
func TestNoSymlinks(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "real", "sub"), 0o755)
	os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "link"))
	if err := NoSymlinks(filepath.Join(dir, "real", "sub", "missing")); err != nil {
		t.Fatal(err)
	}
	if err := NoSymlinks(filepath.Join(dir, "link", "sub")); err == nil {
		t.Fatal("symlink not detected")
	}
}

func TestRestoreNeverOverwritesAndStaysOutsideTheDestination(t *testing.T) {
	root := t.TempDir()
	meta := MetaDir(root, "behaviour")
	run := "20260101T000000Z-abcdef"
	kept := filepath.Join(HistoryDir(meta, run), "session", "a.txt")
	os.MkdirAll(filepath.Dir(kept), 0o755)
	os.WriteFile(kept, []byte("old"), 0o644)
	runs, err := ListHistory(meta)
	if err != nil || len(runs) != 1 || runs[0].Files != 1 || runs[0].Bytes != 3 {
		t.Fatalf("list: %+v %v", runs, err)
	}
	if _, err := Restore(root, meta, run, "session", filepath.Join(root, "inside")); err == nil {
		t.Fatal("restoring into the destination must be refused")
	}
	to := t.TempDir()
	got, err := Restore(root, meta, run, "session", to)
	if err != nil || len(got) != 1 {
		t.Fatalf("restore: %v %v", got, err)
	}
	if data, _ := os.ReadFile(filepath.Join(to, "session", "a.txt")); string(data) != "old" {
		t.Fatal("wrong content restored")
	}
	if _, err := Restore(root, meta, run, "session/a.txt", to); err == nil {
		t.Fatal("an existing file must never be overwritten")
	}
	if _, err := Restore(root, meta, run, "../etc", to); err == nil {
		t.Fatal("paths outside the source layout must be refused")
	}
}

func TestRunSummariesAndFleet(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"behaviour", "ephys"} {
		meta := MetaDir(root, name)
		EnsureMetaDirs(root, name)
		s := RunSummary{RunID: NewRunID(time.Now()), Subfolder: name, Result: ResultOK}
		for i := 0; i < 150; i++ {
			s.Deferred = append(s.Deferred, DeferredFile{Path: "f"})
		}
		s.DeferredCount = len(s.Deferred)
		if err := WriteRunSummary(meta, s); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll(filepath.Join(MetaRoot(root), "no-runs-yet"), 0o755)
	os.MkdirAll(filepath.Join(root, "LUMS0014", "behaviour"), 0o755)
	reports, err := Fleet(root)
	if err != nil || len(reports) != 2 || reports[0].Subfolder != "behaviour" {
		t.Fatalf("fleet: %+v %v", reports, err)
	}
	if got := reports[1].Latest; got == nil || len(got.Deferred) != summaryListLimit || got.DeferredCount != 150 {
		t.Fatalf("summary lists must be capped, counts kept: %+v", got)
	}
}

func TestNewRunIDSortsByTime(t *testing.T) {
	a := NewRunID(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	b := NewRunID(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	if !(a < b) || !ValidRunID(a) || ValidRunID("../x") {
		t.Fatalf("got %s %s", a, b)
	}
}
