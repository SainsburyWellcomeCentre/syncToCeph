// Tests for the manifest, history, run summaries and fleet.
package archive

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManifestAppendLoadAndCompact(t *testing.T) {
	machine := filepath.Join(t.TempDir(), "m")
	root := filepath.Dir(machine)
	if err := EnsureMetaDirs(root, machine); err != nil {
		t.Fatal(err)
	}
	mtime := time.Date(2026, 1, 1, 0, 0, 0, 123456789, time.UTC)
	w, err := OpenManifestWriter(machine)
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
	f, _ := os.OpenFile(ManifestPath(machine), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(`{"path":"cut short`) // as after a crash mid-write
	f.Close()
	m, err := LoadManifest(machine)
	if err != nil || m.Lines != 5 || len(m.Entries) != 2 || m.Entries["a"].Size != 2 {
		t.Fatalf("got %+v %v", m, err)
	}
	if !m.Entries["a"].Matches(2, mtime.Add(500*time.Millisecond)) || m.Entries["a"].Matches(2, mtime.Add(2*time.Second)) {
		t.Fatal("time tolerance is wrong")
	}
	if err := Compact(machine); err != nil {
		t.Fatal(err)
	}
	m, _ = LoadManifest(machine)
	if m.Lines != 2 || m.Entries["a"].Size != 2 || m.Entries["b"].SHA256 != "bb" {
		t.Fatalf("compaction lost data: %+v", m)
	}
}

// SAFETY: invariant 6: EnsureMetaDirs never creates the archive root.
func TestEnsureMetaDirsNeedsArchiveRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing-root")
	if err := EnsureMetaDirs(root, filepath.Join(root, "m")); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("archive root was created")
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

func TestRestoreNeverOverwritesAndStaysOutsideTheArchive(t *testing.T) {
	root := t.TempDir()
	machine := filepath.Join(root, "m")
	run := "20260101T000000Z-abcdef"
	kept := filepath.Join(HistoryDir(machine, run), "session", "a.txt")
	os.MkdirAll(filepath.Dir(kept), 0o755)
	os.WriteFile(kept, []byte("old"), 0o644)
	runs, err := ListHistory(machine)
	if err != nil || len(runs) != 1 || runs[0].Files != 1 || runs[0].Bytes != 3 {
		t.Fatalf("list: %+v %v", runs, err)
	}
	if _, err := Restore(root, machine, run, "session", filepath.Join(root, "inside")); err == nil {
		t.Fatal("restoring into the archive must be refused")
	}
	to := t.TempDir()
	got, err := Restore(root, machine, run, "session", to)
	if err != nil || len(got) != 1 {
		t.Fatalf("restore: %v %v", got, err)
	}
	if data, _ := os.ReadFile(filepath.Join(to, "session", "a.txt")); string(data) != "old" {
		t.Fatal("wrong content restored")
	}
	if _, err := Restore(root, machine, run, "session/a.txt", to); err == nil {
		t.Fatal("an existing file must never be overwritten")
	}
	if _, err := Restore(root, machine, run, "../etc", to); err == nil {
		t.Fatal("paths outside the machine folder must be refused")
	}
}

func TestRunSummariesAndFleet(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"scope-a", "scope-b"} {
		machine := filepath.Join(root, name)
		EnsureMetaDirs(root, machine)
		s := RunSummary{RunID: NewRunID(time.Now()), Machine: name, Result: ResultOK}
		for i := 0; i < 150; i++ {
			s.Deferred = append(s.Deferred, DeferredFile{Path: "f"})
		}
		s.DeferredCount = len(s.Deferred)
		if err := WriteRunSummary(machine, s); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll(filepath.Join(root, "not-a-machine"), 0o755)
	reports, err := Fleet(root)
	if err != nil || len(reports) != 2 || reports[0].Machine != "scope-a" {
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
