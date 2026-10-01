// Tests for the rsync command, its output parsing, and graceful interruption.
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

// SAFETY: invariant 1, 3, 5: the rsync options are fixed and never destructive.
func TestRsyncArgsAreFixedAndNeverDestructive(t *testing.T) {
	forbidden := []string{"--delete", "--del", "--remove-source-files", "--inplace", "--append",
		"--checksum", "-c", "--whole-file", "--no-backup", "--force"}
	for _, existing := range []string{config.ExistingSkip, config.ExistingReplace} {
		for _, verify := range []string{config.VerifyNew, config.VerifyAll} {
			for _, dry := range []bool{false, true} {
				s := config.Settings{Source: "/src", MachineDir: "/archive/m", Existing: existing, Verify: verify, DryRun: dry}
				args := RsyncArgs(s, "/archive/m/.syncToCeph/history/run")
				for _, a := range args {
					for _, f := range forbidden {
						if a == f || strings.HasPrefix(a, f+"=") || (strings.HasPrefix(f, "--del") && strings.HasPrefix(a, "--del")) {
							t.Errorf("%s/%s: forbidden option %s", existing, verify, a)
						}
					}
				}
				joined := strings.Join(args, " ")
				for _, must := range []string{"--files-from=- --from0", "--times", "--omit-dir-times",
					"--partial-dir=.syncToCeph-partial", "--fsync", "--itemize-changes"} {
					if !strings.Contains(joined, must) {
						t.Errorf("%s: missing %s", existing, must)
					}
				}
				if existing == config.ExistingReplace && !strings.Contains(joined, "--backup --backup-dir=/archive/m/.syncToCeph/history/run") {
					t.Errorf("replace mode must back up into history: %s", joined)
				}
				if existing == config.ExistingSkip && (!strings.Contains(joined, "--ignore-existing") || strings.Contains(joined, "--ignore-times")) {
					t.Errorf("skip mode must use --ignore-existing: %s", joined)
				}
				if tail := args[len(args)-3:]; tail[0] != "--" || tail[1] != "/src/" || tail[2] != "/archive/m/" {
					t.Errorf("paths must follow --: %v", tail)
				}
			}
		}
	}
}

func TestRsyncEnvDropsOverrides(t *testing.T) {
	t.Setenv("RSYNC_CHECKSUM_LIST", "none")
	t.Setenv("RSYNC_OLD_ARGS", "1")
	t.Setenv("LC_ALL", "de_DE.UTF-8")
	env := strings.Join(rsyncEnv("/state"), "\n")
	if strings.Contains(env, "RSYNC_") || strings.Contains(env, "de_DE") {
		t.Fatalf("environment not cleaned: %s", env)
	}
	if !strings.Contains(env, "LC_ALL=C") || !strings.Contains(env, "HOME=/state") {
		t.Fatalf("missing LC_ALL=C or HOME: %s", env)
	}
}

func TestParseItemized(t *testing.T) {
	tests := []struct {
		line     string
		ok       bool
		received bool
		name     string
	}{
		{">f+++++++++ session 1/stack_0001.tif", true, true, "session 1/stack_0001.tif"},
		{">f.st...... data.bin", true, true, "data.bin"},
		{"cd+++++++++ session 1/", true, false, "session 1/"},
		{`>f+++++++++ new\#012line.dat`, true, true, "new\nline.dat"},
		{`>f+++++++++ back\\slash`, true, true, `back\\slash`},
		{"rsync: [sender] link_stat \"/x\" failed: No such file or directory (2)", false, false, ""},
		{"", false, false, ""},
	}
	for _, tt := range tests {
		item, ok := ParseItemized(tt.line)
		if ok != tt.ok || item.Received() != tt.received || item.Name != tt.name {
			t.Errorf("%q: got %+v ok=%t", tt.line, item, ok)
		}
	}
}

// SAFETY: invariant 5, 11: an interrupted copy never appears under its final name; the
// partial data waits in .syncToCeph-partial and the next run completes it.
func TestInterruptedCopyStaysOutOfPlaceAndResumes(t *testing.T) {
	s := testSettings(t)
	big := make([]byte, 4<<20)
	for i := range big {
		big[i] = byte(i * 7)
	}
	src := filepath.Join(s.Source, "large.bin")
	os.WriteFile(src, big, 0o644)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(src, old, old)
	write(t, filepath.Join(s.MachineDir, "large.bin"), "previous version")
	s.Existing = config.ExistingReplace

	rsyncPath, err := Preflight(s)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		// Wait until rsync has written some data to its temporary file.
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			matches, _ := filepath.Glob(filepath.Join(s.MachineDir, ".large.bin.*"))
			for _, m := range matches {
				if info, err := os.Stat(m); err == nil && info.Size() >= 64*1024 {
					cancel()
					return
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		cancel()
	}()
	args := append([]string{"--bwlimit=256"}, RsyncArgs(s, archive.HistoryDir(s.MachineDir, "test"))...)
	code, err := runRsync(ctx, rsyncJob{path: rsyncPath, args: args, files: []string{"large.bin"},
		env: rsyncEnv(s.StateDir), grace: func() time.Duration { return KillGrace }})
	if err != nil || code == 0 {
		t.Fatalf("rsync should have been interrupted: code %d err %v", code, err)
	}
	if read(t, filepath.Join(s.MachineDir, "large.bin")) != "previous version" {
		t.Fatal("unfinished data appeared under the final name")
	}
	if info, err := os.Stat(filepath.Join(s.MachineDir, archive.PartialName, "large.bin")); err != nil || info.Size() == 0 {
		t.Fatalf("partial file not kept in %s: %v", archive.PartialName, err)
	}
	out := runSync(t, s)
	if out.Summary.Result != archive.ResultOK || read(t, filepath.Join(s.MachineDir, "large.bin")) != string(big) {
		t.Fatalf("the next run did not complete the copy: %s %v", out.Summary.Result, out.Summary.Errors)
	}
}

// An interrupted run reports INTERRUPTED, verifies nothing, and leaves the
// copied files for the next run to verify.
func TestInterruptedRunIsNeverVerified(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "a"), "a")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := Run(ctx, Options{Settings: s, RunID: archive.NewRunID(time.Now()),
		PendingVerify: []string{"a"}})
	if out.Summary.Result != archive.ResultInterrupted || out.Summary.Verified != 0 || out.Summary.ExitCode != 130 {
		t.Fatalf("got %+v", out.Summary)
	}
	if len(out.PendingVerify) != 1 {
		t.Fatalf("pending files must be carried over, got %v", out.PendingVerify)
	}
}

func TestPendingFilesAreVerifiedByTheNextRun(t *testing.T) {
	s := testSettings(t)
	write(t, filepath.Join(s.Source, "a"), "a")
	info, _ := os.Stat(filepath.Join(s.Source, "a"))
	// As if a previous run copied the file and was stopped before verifying.
	write(t, filepath.Join(s.MachineDir, "a"), "a")
	os.Chtimes(filepath.Join(s.MachineDir, "a"), info.ModTime(), info.ModTime())
	out := Run(context.Background(), Options{Settings: s, RunID: archive.NewRunID(time.Now()), PendingVerify: []string{"a"}})
	if out.Summary.Verified != 1 || out.Summary.Unverified != 0 || len(out.PendingVerify) != 0 {
		t.Fatalf("got %+v pending %v", out.Summary, out.PendingVerify)
	}
}
