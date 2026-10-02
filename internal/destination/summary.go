// This file defines the run summary: the record of what one run did. The same
// record is shown by `synctoceph status`, saved in the local status file, and
// written to the destination (<destination>/.syncToCeph/<subfolder>/runs/)
// so that `synctoceph fleet` can report on every acquisition machine. Its
// JSON form is a contract (see docs/operations.md); change schema_version if
// you change its meaning.
package destination

import (
	"time"
)

// SummarySchemaVersion is the version of the RunSummary JSON format.
const SummarySchemaVersion = 2

// The four possible results of a run.
const (
	ResultOK          = "OK"          // everything planned was copied and verified
	ResultPartial     = "PARTIAL"     // some files were deferred to a later run
	ResultFailed      = "FAILED"      // a preflight, rsync or verification error
	ResultInterrupted = "INTERRUPTED" // stopped by the user or a signal
)

// Exit codes returned by `synctoceph run` for each result.
const (
	ExitOK          = 0
	ExitFailed      = 1
	ExitUsage       = 2
	ExitPartial     = 3
	ExitInterrupted = 130
)

// ExitCode returns the process exit code for a result.
func ExitCode(result string) int {
	switch result {
	case ResultOK:
		return ExitOK
	case ResultPartial:
		return ExitPartial
	case ResultInterrupted:
		return ExitInterrupted
	default:
		return ExitFailed
	}
}

// DeferredFile is a file left for a later run.
type DeferredFile struct {
	Path       string    `json:"path"`
	Reason     string    `json:"reason"`
	ModifiedAt time.Time `json:"modified_at"`
	// Since is when the file was first deferred (kept across runs), so files
	// stuck for more than a day can be escalated.
	Since time.Time `json:"since"`
}

// Deferral reasons.
const (
	ReasonSettling = "recently modified"
	ReasonChanged  = "changed during the sync"
	ReasonVanished = "disappeared during the sync"
)

// SkippedFile is a source entry that is never copied (symlinks, special
// files such as pipes or devices, and reserved names).
type SkippedFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// RunSummary is the record of one run.
type RunSummary struct {
	SchemaVersion int       `json:"schema_version"`
	RunID         string    `json:"run_id"`
	Subfolder     string    `json:"subfolder"`
	Hostname      string    `json:"hostname"`
	Profile       string    `json:"profile"`
	Version       string    `json:"synctoceph_version"`
	Source        string    `json:"source"`
	Destination   string    `json:"destination"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	DryRun        bool      `json:"dry_run"`
	Existing      string    `json:"existing"`
	Verify        string    `json:"verify"`
	Result        string    `json:"result"`
	ExitCode      int       `json:"exit_code"`

	Scanned       int   `json:"scanned_files"`
	ScannedBytes  int64 `json:"scanned_bytes"`
	Excluded      int   `json:"excluded_files"`
	Planned       int   `json:"planned_files"`
	PlannedBytes  int64 `json:"planned_bytes"`
	Replaced      int   `json:"replaced_files"`
	Copied        int   `json:"copied_files"`
	CopiedBytes   int64 `json:"copied_bytes"`
	Verified      int   `json:"verified_files"`
	VerifiedBytes int64 `json:"verified_bytes"`
	UpToDate      int   `json:"up_to_date_files"`
	Unverified    int   `json:"unverified_files"`

	DeferredCount  int            `json:"deferred_count"`
	Deferred       []DeferredFile `json:"deferred"`
	DifferingCount int            `json:"differing_count"`
	Differing      []string       `json:"differing"`
	SkippedCount   int            `json:"skipped_count"`
	Skipped        []SkippedFile  `json:"skipped"`
	ErrorCount     int            `json:"error_count"`
	Errors         []string       `json:"errors"`
	WouldCopy      []string       `json:"would_copy,omitempty"`

	// NewUnverified is true when the number of unverified files changed
	// since the last report, so the suggestion to verify is shown only once.
	NewUnverified bool `json:"-"`

	RsyncExitCode *int       `json:"rsync_exit_code"`
	HistoryDir    string     `json:"history_dir,omitempty"`
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
}

// summaryListLimit is how many entries of each file list are written into
// the run summaries on the destination (the counts are always complete).
const summaryListLimit = 100

// Capped returns a copy whose file lists hold at most summaryListLimit
// entries. The *Count fields keep the full numbers.
func (s RunSummary) Capped() RunSummary {
	c := s
	if len(c.Deferred) > summaryListLimit {
		c.Deferred = c.Deferred[:summaryListLimit]
	}
	if len(c.Differing) > summaryListLimit {
		c.Differing = c.Differing[:summaryListLimit]
	}
	if len(c.Skipped) > summaryListLimit {
		c.Skipped = c.Skipped[:summaryListLimit]
	}
	if len(c.Errors) > summaryListLimit {
		c.Errors = c.Errors[:summaryListLimit]
	}
	if len(c.WouldCopy) > summaryListLimit {
		c.WouldCopy = c.WouldCopy[:summaryListLimit]
	}
	return c
}

// OldestDeferral returns when the longest-waiting deferred file was first
// deferred, or the zero time if nothing is deferred.
func (s RunSummary) OldestDeferral() time.Time {
	var oldest time.Time
	for _, d := range s.Deferred {
		if !d.Since.IsZero() && (oldest.IsZero() || d.Since.Before(oldest)) {
			oldest = d.Since
		}
	}
	return oldest
}

// Succeeded reports whether the run finished without errors (OK or PARTIAL).
func (s RunSummary) Succeeded() bool {
	return s.Result == ResultOK || s.Result == ResultPartial
}
