// This file reads and writes status.json in the state folder: what the
// profile is doing now, the last run's result, recent runs, and bookkeeping
// that must survive between runs (when each deferred file was first seen,
// files still waiting for verification, the last daily run). The file is
// replaced atomically, so a crash never leaves it half-written.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/platform"
)

// StatusSchemaVersion is the version of the status.json format.
const StatusSchemaVersion = 2

// recentRunsKept is how many short run records status.json keeps.
const recentRunsKept = 20

// States a profile can be in, as written by the running process. Whether a
// process is really alive is decided by the lock, not by this field.
const (
	StateIdle    = "idle"
	StateRunning = "running"
	StateWaiting = "waiting"
	StateStopped = "stopped"
)

// Status is the content of status.json.
type Status struct {
	SchemaVersion int        `json:"schema_version"`
	Profile       string     `json:"profile"`
	UpdatedAt     time.Time  `json:"updated_at"`
	State         string     `json:"state"`
	Mode          string     `json:"mode,omitempty"` // "run" or "schedule"
	PID           int        `json:"pid,omitempty"`
	Phase         string     `json:"phase,omitempty"`
	Schedule      string     `json:"schedule,omitempty"`
	NextRunAt     *time.Time `json:"next_run_at,omitempty"`

	LastRun       *destination.RunSummary `json:"last_run,omitempty"`
	LastSuccessAt *time.Time              `json:"last_success_at,omitempty"`
	RecentRuns    []RunBrief              `json:"recent_runs"`

	// DeferredSince remembers when each deferred file was first deferred.
	DeferredSince map[string]time.Time `json:"deferred_since,omitempty"`
	// PendingVerify lists files copied by a run that ended before verifying
	// them; the next run verifies them.
	PendingVerify []string `json:"pending_verify,omitempty"`
	// LastDailyDate is the local date (YYYY-MM-DD) of the last daily run, and
	// LastDailyAt the daily time it was for.
	LastDailyDate string `json:"last_daily_date,omitempty"`
	// UnverifiedReported is the number of unverified files on the destination last
	// reported, so the same suggestion is not repeated on every run.
	UnverifiedReported int    `json:"unverified_reported,omitempty"`
	LastDailyAt        string `json:"last_daily_at,omitempty"`
}

// RunBrief is a one-line record of a past run.
type RunBrief struct {
	RunID       string    `json:"run_id"`
	Result      string    `json:"result"`
	DryRun      bool      `json:"dry_run"`
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  time.Time `json:"finished_at"`
	Copied      int       `json:"copied_files"`
	CopiedBytes int64     `json:"copied_bytes"`
	Deferred    int       `json:"deferred_count"`
}

// AddRun records a finished run as the last run and in the recent list.
func (s *Status) AddRun(r destination.RunSummary) {
	s.LastRun = &r
	s.RecentRuns = append(s.RecentRuns, RunBrief{RunID: r.RunID, Result: r.Result, DryRun: r.DryRun,
		StartedAt: r.StartedAt, FinishedAt: r.FinishedAt, Copied: r.Copied,
		CopiedBytes: r.CopiedBytes, Deferred: r.DeferredCount})
	if len(s.RecentRuns) > recentRunsKept {
		s.RecentRuns = s.RecentRuns[len(s.RecentRuns)-recentRunsKept:]
	}
	if r.Succeeded() && !r.DryRun {
		t := r.FinishedAt
		s.LastSuccessAt = &t
	}
}

// ReadStatus reads status.json. A missing file gives an idle status.
func ReadStatus(dir, profile string) (Status, error) {
	s := Status{SchemaVersion: StatusSchemaVersion, Profile: profile, State: StateIdle}
	data, err := os.ReadFile(filepath.Join(dir, StatusName))
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("reading %s: %w", filepath.Join(dir, StatusName), err)
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("reading %s: %w", filepath.Join(dir, StatusName), err)
	}
	return s, nil
}

// WriteStatus saves status.json atomically.
func WriteStatus(dir string, s *Status) error {
	s.SchemaVersion = StatusSchemaVersion
	s.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding status: %w", err)
	}
	return platform.WriteFileAtomic(filepath.Join(dir, StatusName), append(data, '\n'), 0o600)
}
