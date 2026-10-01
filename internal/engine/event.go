// This file defines the progress events a run sends while it works: a file
// was copied, a copy was verified, a file was left for later, and the results
// of the scan and the plan. The engine only reports facts; the command that
// started the run decides how to show them (see internal/cli/runview.go).
package engine

import "github.com/SainsburyWellcomeCentre/syncToCeph/internal/archive"

// Kinds of Event.
const (
	EventScanned  = "scanned"  // the source was scanned; Summary holds the counts
	EventPlanned  = "planned"  // the copy list is ready; Summary holds the counts
	EventCopied   = "copied"   // rsync finished copying one file
	EventVerified = "verified" // a copy's SHA-256 matched its source file
	EventDeferred = "deferred" // a file changed or vanished and is left for a later run
)

// Event is one piece of progress in a run.
type Event struct {
	Kind   string
	Path   string // the file, relative to the source (file events only)
	Size   int64  // the file's size in bytes (file events only)
	Reason string // why a file was deferred
	// Done and Total give the file's position in the current step, for
	// example the 3rd of 40 files to verify. Both are 0 when not known.
	Done, Total int
	// Summary is the run summary so far (scanned and planned events only).
	Summary archive.RunSummary
}
