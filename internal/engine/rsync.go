// This file builds the rsync command and runs it. The option list is fixed in
// code: nothing from the config file or the command line is passed through,
// and options that could delete or overwrite data in place are never used.
// rsync runs in its own process group so that stopping can be done
// gracefully: SIGINT first (rsync keeps the partial file for next time), then
// SIGKILL if it has not exited after the grace period.
package engine

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
)

// KillGrace is how long rsync gets to exit after SIGINT before SIGKILL.
const KillGrace = 30 * time.Second

// RsyncArgs returns the rsync options (without the program name) for copying
// the listed files from one animal folder in the source (from) to that
// animal's subfolder on the destination (to). existing is the existing
// setting; historyDir is where replace mode keeps previous versions.
func RsyncArgs(existing, historyDir, from, to string) []string {
	// SAFETY: invariants 1 and 2 (synctoceph never deletes from the destination or
	// the source): the options are fixed here. Never add --delete*,
	// --remove-source-files, --inplace or --append, and never add options
	// taken from config or user input.
	// SAFETY: invariant 5 (unfinished copies never appear under the final name): rsync
	// writes to a temporary name and moves interrupted copies to
	// --partial-dir; --fsync flushes each file before it is renamed.
	args := []string{
		"--files-from=-", "--from0",
		"--times", "--omit-dir-times",
		"--partial-dir=" + destination.PartialName,
		"--fsync",
		"--itemize-changes",
	}
	if existing == config.ExistingReplace {
		// SAFETY: invariant 3 (replaced files are kept): the old version is moved to
		// the run's history folder before the new one takes its place.
		// --ignore-times makes rsync copy every listed file, including ones
		// whose size and time match but whose content was found to differ.
		args = append(args, "--backup", "--backup-dir="+historyDir, "--ignore-times")
	} else {
		// Second line of defence for skip mode: never touch existing files.
		args = append(args, "--ignore-existing")
	}
	return append(args, "--", from+"/", to+"/")
}

// rsyncEnv returns the environment for rsync: the current environment
// without RSYNC_* variables (which can change rsync's behaviour), with
// messages in plain English (LC_ALL=C) so they can be parsed, and with HOME
// set to the state folder so personal rsync option aliases (~/.popt) cannot
// add options.
func rsyncEnv(stateDir string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "RSYNC_") || strings.HasPrefix(kv, "LC_ALL=") || strings.HasPrefix(kv, "HOME=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "LC_ALL=C", "HOME="+stateDir)
}

// rsyncJob holds what runRsync needs.
type rsyncJob struct {
	path      string
	args      []string
	files     []string // relative paths to copy
	env       []string
	lockFiles []*os.File
	grace     func() time.Duration
	onStdout  func(line string)
	onStderr  func(line string)
}

// runRsync runs rsync and waits for it. If ctx is cancelled it sends SIGINT
// to rsync's process group, then SIGKILL after the grace period. It returns
// rsync's exit code (128+signal if rsync was killed by a signal).
func runRsync(ctx context.Context, job rsyncJob) (int, error) {
	cmd := exec.Command(job.path, job.args...)
	cmd.Env = job.env
	// SAFETY: invariant 10 (one run per profile): rsync inherits the lock files, so the
	// locks stay held while rsync runs, even if synctoceph is killed.
	cmd.ExtraFiles = job.lockFiles
	// SAFETY: invariant 11 (stopping is graceful): rsync gets its own process group so
	// signals reach it and its helpers together.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return -1, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return -1, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return -1, err
	}
	if err := cmd.Start(); err != nil {
		return -1, err
	}
	var readers sync.WaitGroup
	readers.Add(3)
	go func() { defer readers.Done(); writeFileList(stdin, job.files) }()
	go func() { defer readers.Done(); readLines(stdout, job.onStdout) }()
	go func() { defer readers.Done(); readLines(stderr, job.onStderr) }()

	done := make(chan struct{})
	go func() { readers.Wait(); close(done) }()
	stopRsyncOnCancel(ctx, cmd.Process.Pid, done, job.grace)
	<-done
	err = cmd.Wait()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal()), nil
		}
		return exitErr.ExitCode(), nil
	}
	if err != nil {
		return -1, err
	}
	return 0, nil
}

// stopRsyncOnCancel waits until rsync's output closes (done). If ctx is
// cancelled first, it sends SIGINT to the process group, and SIGKILL if rsync
// is still running after the grace period.
func stopRsyncOnCancel(ctx context.Context, pid int, done <-chan struct{}, grace func() time.Duration) {
	select {
	case <-done:
		return
	case <-ctx.Done():
	}
	// SAFETY: invariant 11 (stopping is graceful): SIGINT lets rsync keep its partial file.
	syscall.Kill(-pid, syscall.SIGINT)
	timer := time.NewTimer(grace())
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		syscall.Kill(-pid, syscall.SIGKILL)
	}
}

// writeFileList sends the NUL-separated file list to rsync's standard input.
// NUL separation means any file name (spaces, newlines, quotes) is passed
// literally.
func writeFileList(w io.WriteCloser, files []string) {
	defer w.Close()
	out := bufio.NewWriter(w)
	for _, f := range files {
		out.WriteString(f)
		out.WriteByte(0)
	}
	out.Flush()
}

// readLines calls handle for each line read from r.
func readLines(r io.Reader, handle func(string)) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if handle != nil {
			handle(scanner.Text())
		}
	}
	io.Copy(io.Discard, r) // keep draining if a line was too long
}
