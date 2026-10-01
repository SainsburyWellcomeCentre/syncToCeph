// This file writes the log file (sync.log in the state folder) and reads it
// back for `synctoceph logs`. Every line has a time, a level and the run ID,
// so the lines of one run can be picked out later. When the log reaches
// logMaxBytes it is renamed to sync.log.1 (older ones shift up to
// sync.log.5) and a new one is started, so logs never fill the disk.
package state

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// logMaxBytes is the size at which the log file is rotated.
const logMaxBytes = 5 * 1000 * 1000

// logBackups is how many rotated log files are kept (sync.log.1 to .5).
const logBackups = 5

// Logger appends lines to the log file. It is safe to use from several
// goroutines (rsync's output and error streams are logged concurrently).
type Logger struct {
	mu    sync.Mutex
	dir   string
	file  *os.File
	size  int64
	runID string
	// Echo, if set, receives a copy of every line (used by `schedule` when
	// run in a terminal).
	Echo io.Writer
}

// OpenLog opens (or creates) the log file in the state folder.
func OpenLog(dir string) (*Logger, error) {
	l := &Logger{dir: dir}
	if err := l.open(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *Logger) open() error {
	path := filepath.Join(l.dir, LogName)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("opening log %s: %w", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return fmt.Errorf("reading log %s: %w", path, err)
	}
	l.file, l.size = f, info.Size()
	return nil
}

// SetRun sets the run ID written on following lines ("" between runs).
func (l *Logger) SetRun(runID string) {
	l.mu.Lock()
	l.runID = runID
	l.mu.Unlock()
}

// Info logs a normal event.
func (l *Logger) Info(format string, args ...any) { l.write("INFO ", format, args...) }

// Warn logs something the user should know about.
func (l *Logger) Warn(format string, args ...any) { l.write("WARN ", format, args...) }

// Error logs a failure.
func (l *Logger) Error(format string, args ...any) { l.write("ERROR", format, args...) }

func (l *Logger) write(level, format string, args ...any) {
	if l == nil {
		return
	}
	message := fmt.Sprintf(format, args...)
	// One record per line: escape line breaks (file names can contain them).
	message = strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\r", "\\r").Replace(message)
	l.mu.Lock()
	defer l.mu.Unlock()
	run := l.runID
	if run == "" {
		run = "-"
	}
	line := fmt.Sprintf("%s %s [%s] %s\n", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), level, run, message)
	if l.Echo != nil {
		io.WriteString(l.Echo, line)
	}
	if l.file == nil {
		return
	}
	if l.size+int64(len(line)) > logMaxBytes {
		l.rotate()
	}
	if n, err := l.file.WriteString(line); err == nil {
		l.size += int64(n)
	}
}

// rotate shifts sync.log.N to sync.log.N+1 and starts a new sync.log. It is
// called with l.mu held.
func (l *Logger) rotate() {
	l.file.Close()
	l.file = nil
	base := filepath.Join(l.dir, LogName)
	os.Remove(fmt.Sprintf("%s.%d", base, logBackups))
	for i := logBackups - 1; i >= 1; i-- {
		os.Rename(fmt.Sprintf("%s.%d", base, i), fmt.Sprintf("%s.%d", base, i+1))
	}
	os.Rename(base, base+".1")
	if err := l.open(); err != nil {
		l.file = nil
	}
}

// Close closes the log file.
func (l *Logger) Close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
}

// Tail returns the last n log lines, oldest first, across rotated files. If
// runID is not empty, only lines from that run are returned.
func Tail(dir string, n int, runID string) ([]string, error) {
	var lines []string
	base := filepath.Join(dir, LogName)
	names := []string{}
	for i := logBackups; i >= 1; i-- {
		names = append(names, fmt.Sprintf("%s.%d", base, i))
	}
	names = append(names, base)
	for _, name := range names {
		f, err := os.Open(name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("reading log %s: %w", name, err)
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if runID != "" && !strings.Contains(line, "["+runID+"]") {
				continue
			}
			lines = append(lines, line)
			if n > 0 && len(lines) > 2*n {
				lines = append([]string(nil), lines[len(lines)-n:]...)
			}
		}
		f.Close()
	}
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}
