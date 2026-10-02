// This file holds the wording of problems found while writing to or checking
// the destination on ceph: conflicts, verification failures, rsync errors and
// synctoceph's records there. See messages.go for how the message files are organised.
package ui

import (
	"fmt"
)

// Conflict: a source file cannot be written to the destination safely.
func Conflict(path string, err error) string {
	return fmt.Sprintf("%s was not copied: %v. Rename or move the entry on the destination by hand; synctoceph never deletes or follows it", path, err)
}

// VerifyFailed: a copy could not be verified.
func VerifyFailed(path string, err error) string {
	return fmt.Sprintf("%s is NOT copied and verified: %v. It will be checked again on the next run; if this repeats, compare the two copies by hand", path, err)
}

// MetaDirFailed: synctoceph's records folder cannot be created.
func MetaDirFailed(dir string, err error) error {
	return &Problem{What: "cannot prepare the records folder " + dir, Err: err,
		Why: "run summaries, the verified-file record and history live there",
		Fix: "check that you can write to the destination folder and that nothing in the path is a symlink"}
}

// AnimalDirFailed: an animal folder or its subfolder cannot be created.
func AnimalDirFailed(dir string, err error) error {
	return &Problem{What: "cannot prepare the folder " + dir, Err: err,
		Why: "the files of this animal are copied there",
		Fix: "check that you can write to the destination folder and that nothing in the path is a symlink or a file"}
}

// DestinationBusy: another run on this computer writes the same subfolder.
func DestinationBusy(subfolder string) error {
	return &Problem{What: "another synctoceph run on this computer is copying into the subfolder " + subfolder,
		Why: "two runs writing the same folders at once could get in each other's way",
		Fix: "wait for it to finish; if two profiles use the same destination and subfolder, give each its own subfolder"}
}

// LockUnsupported: flock does not work on the destination filesystem.
func LockUnsupported(path string) string {
	return "the destination filesystem does not support locks (" + path + "); relying on the local state lock only"
}

// ManifestUnreadable: the verified-file record cannot be read.
func ManifestUnreadable(path string, err error) error {
	return &Problem{What: "cannot read the verified-file record " + path, Err: err,
		Fix: "check permissions on the destination folder"}
}

// ManifestUnwritable: the verified-file record cannot be written.
func ManifestUnwritable(path string, err error) error {
	return &Problem{What: "cannot write the verified-file record " + path, Err: err,
		Why: "without it, synctoceph cannot report files as verified",
		Fix: "check that ceph is mounted read-write and not full"}
}

// RsyncStartFailed: rsync could not be started.
func RsyncStartFailed(path string, err error) error {
	return &Problem{What: "cannot start rsync (" + path + ")", Err: err,
		Fix: "run `synctoceph doctor` to check the rsync installation"}
}

// RsyncFailed: rsync exited with an error.
func RsyncFailed(code int) error {
	return &Problem{What: fmt.Sprintf("rsync reported an error (exit code %d: %s)", code, rsyncMeaning(code)),
		Why: "some files may not have been copied; each one is checked and listed",
		Fix: "see the rsync lines in `synctoceph logs`, fix the cause, and run again; completed copies are kept"}
}

// rsyncMeaning explains common rsync exit codes.
func rsyncMeaning(code int) string {
	switch code {
	case 1:
		return "syntax or usage error"
	case 3:
		return "errors selecting input/output files or folders"
	case 11:
		return "error in file input/output, often a full or disconnected drive"
	case 12:
		return "error in the rsync protocol data stream"
	case 20:
		return "rsync was interrupted"
	case 23:
		return "some files could not be transferred"
	case 30:
		return "timeout in data send/receive"
	default:
		if code > 128 {
			return fmt.Sprintf("rsync was killed by signal %d", code-128)
		}
		return "see `man rsync`, section EXIT VALUES"
	}
}

// NoControlSocket: the control socket could not be started.
func NoControlSocket(err error) string {
	return fmt.Sprintf("the control socket is not available (%v); `synctoceph stop` will not reach this process, use Ctrl-C or `kill -TERM` instead", err)
}

// SummaryNotWritten: the run summary could not be saved on the destination.
func SummaryNotWritten(err error) string {
	return fmt.Sprintf("could not save the run summary on the destination (%v); `synctoceph fleet` will show an older run", err)
}
