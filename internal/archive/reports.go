// This file defines the reports about already-archived files produced by
// `synctoceph check-archived` and `synctoceph verify`. They are written as
// JSON with --json, so their fields are a contract.
package archive

// File states reported by CheckArchived.
const (
	StatusVerified    = "verified"
	StatusNotArchived = "not-archived"
	StatusUnverified  = "unverified"
	StatusDiffers     = "differs"
	StatusExcluded    = "excluded"
	StatusSkipped     = "skipped"
)

// FileStatus is the archive state of one source file.
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
	NotArchived   []string `json:"not_archived"`
	Changed       []string `json:"changed"`
	Errors        []string `json:"errors"`
}
