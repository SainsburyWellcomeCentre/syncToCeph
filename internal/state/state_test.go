// Tests for the lock, status file, logs and control socket.
package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
)

// SAFETY: invariant 10: a second holder is refused while the first holds the lock.
func TestLockIsExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), LockName)
	if IsHeld(path) {
		t.Fatal("a missing lock file is not held")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("IsHeld must not create the lock file")
	}
	first, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(path); err != ErrLocked {
		t.Fatalf("second Acquire: got %v, want ErrLocked", err)
	}
	if !IsHeld(path) {
		t.Fatal("IsHeld must see the lock")
	}
	first.Release()
	if IsHeld(path) {
		t.Fatal("lock still held after Release")
	}
	second, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	second.Release()
}

func TestStatusRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := ReadStatus(dir, "default")
	if err != nil || st.State != StateIdle {
		t.Fatalf("missing status: %+v %v", st, err)
	}
	for i := 0; i < recentRunsKept+5; i++ {
		st.AddRun(destination.RunSummary{RunID: "r", Result: destination.ResultOK, FinishedAt: time.Now()})
	}
	if err := WriteStatus(dir, &st); err != nil {
		t.Fatal(err)
	}
	got, err := ReadStatus(dir, "default")
	if err != nil || len(got.RecentRuns) != recentRunsKept || got.LastSuccessAt == nil {
		t.Fatalf("got %+v %v", got, err)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, ".*.tmp")); len(leftovers) != 0 {
		t.Fatalf("temporary files left behind: %v", leftovers)
	}
}

func TestLogRotationAndTail(t *testing.T) {
	dir := t.TempDir()
	log, err := OpenLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	log.SetRun("run-a")
	log.Info("first line with a\nnewline")
	line := strings.Repeat("x", 1000)
	for i := 0; i < logMaxBytes/1000+10; i++ {
		log.Info("%s", line)
	}
	log.SetRun("run-b")
	log.Warn("last line")
	log.Close()
	if _, err := os.Stat(filepath.Join(dir, LogName+".1")); err != nil {
		t.Fatal("log was not rotated")
	}
	lines, err := Tail(dir, 1, "")
	if err != nil || len(lines) != 1 || !strings.Contains(lines[0], "WARN  [run-b] last line") {
		t.Fatalf("tail: %v %v", lines, err)
	}
	lines, _ = Tail(dir, 0, "run-a")
	if len(lines) == 0 || !strings.Contains(lines[0], `a\nnewline`) {
		t.Fatalf("filter by run, newline escaped: %v", lines[:1])
	}
}

func TestControlSocketWorksWithLongPaths(t *testing.T) {
	dir := filepath.Join(t.TempDir(), strings.Repeat("long-folder-name-", 8))
	os.MkdirAll(dir, 0o700)
	if len(filepath.Join(dir, ControlName)) <= maxSocketPath {
		t.Fatal("test path is not long enough")
	}
	stopped := make(chan int, 1)
	server, err := ServeControl(dir, func(r Request) Response {
		if r.Command == "stop" {
			stopped <- r.GraceSeconds
		}
		return Response{OK: true, Live: &Live{Phase: "copying"}}
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := SendControl(dir, Request{Command: "stop", GraceSeconds: 7})
	if err != nil || !resp.OK || resp.Live.Phase != "copying" || <-stopped != 7 {
		t.Fatalf("got %+v %v", resp, err)
	}
	server.Close()
	if _, err := os.Lstat(filepath.Join(dir, ControlName)); !os.IsNotExist(err) {
		t.Fatal("socket file not removed on Close")
	}
	if _, err := SendControl(dir, Request{Command: "status"}); err != ErrNotRunning {
		t.Fatalf("got %v, want ErrNotRunning", err)
	}
}

func TestPrepareRefusesSharedFolders(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	if err := Prepare(dir); err != nil {
		t.Fatal(err)
	}
	os.Chmod(dir, 0o777)
	if err := Prepare(dir); err == nil {
		t.Fatal("a folder others can write to must be refused")
	}
	if err := CheckLocal("/mnt/z/state"); err == nil {
		t.Fatal("a Windows drive path must be refused")
	}
}
