// This file defines the reports about files already copied to the
// destination, produced by `synctoceph check-copied` and `synctoceph verify`. They are written as
// JSON with --json, so their fields are a contract.
package destination

// File states reported by CheckCopied.
const (
	StatusVerified   = "verified"
	StatusNotCopied  = "not-copied"
	StatusUnverified = "unverified"
	StatusDiffers    = "differs"
	StatusExcluded   = "excluded"
	StatusSkipped    = "skipped"
)

// FileStatus is the state on the destination of one source file.
type FileStatus struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

// VerifyReport is the result of `synctoceph verify`.
type VerifyReport struct {
	Checked       int      `json:"checked_files"`
	Verified      int      `json:"verified_files"`
	VerifiedBytes int64    `json:"verified_bytes"`
	Differing     []string `json:"differing"`
	NotCopied     []string `json:"not_copied"`
	Changed       []string `json:"changed"`
	Errors        []string `json:"errors"`
}
