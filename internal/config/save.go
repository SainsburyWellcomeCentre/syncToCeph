// This file writes a profile's config file (used by `synctoceph init`). The
// file is written with an explanation above every key, so people can edit it
// by hand. It is written to a temporary file first and then renamed, so a
// crash can never leave a half-written config behind.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// configTemplate is the layout of a written config file. %s placeholders are
// filled with TOML-quoted values in the order of the keys.
const configTemplate = `# synctoceph configuration, profile %q.
# Edit this file freely; unknown keys are errors. Check it with: synctoceph doctor
# Full reference: docs/configuration.md

# Folder to copy from, holding one folder per animal. It is only ever read,
# never changed. Files directly in it (outside an animal folder) are not copied.
source        = %s

# Folder on ceph (or another mounted network drive) that holds the animal
# folders. It must already exist; it is never created.
destination   = %s

# This machine's folder inside every animal folder, e.g. "behaviour" or
# "ephys": <source>/<animal>/... goes to <destination>/<animal>/<subfolder>/...
subfolder     = %s

# Mount point that must be mounted and contain the destination ("" = no check).
require_mount = %s

# Files already on ceph that differ from the source:
# "skip" leaves them alone; "replace" copies over them and keeps the old
# version under <destination>/.syncToCeph/<subfolder>/history/.
existing      = %s

# Which files to check with SHA-256 after copying:
# "new" = files copied in this run; "all" = every file.
verify        = %s

# Files modified more recently than this are left for a later run.
settle_time   = %s

# File or folder names to leave out (simple patterns: * ? [abc]). A name
# matches anywhere; a pattern with / matches a path from the source folder,
# e.g. "LUMS0014/scratch"; a trailing / matches folders only, e.g. "tmp/".
exclude       = %s

# true = leave out every file and folder whose name starts with "."
# (such as .git, .DS_Store or .Trash-1000), as if ".*" were in exclude.
exclude_hidden = %t

# Schedule for the service: set interval OR at, not both.
%s
%s

# true = only show what would be copied; never write to the destination.
dry_run       = %t

# true = list every file as it is copied and verified; false = show only
# each step and the result. Override once with -v or --verbose=false.
verbose       = %t
`

// Render returns the text of a config file for profile.
func (c Config) Render(profile string) string {
	interval, at := "# interval    = \"4h\"", "# at          = \"02:00\""
	if c.Interval != "" {
		interval = "interval      = " + quote(c.Interval)
	}
	if c.At != "" {
		at = "at            = " + quote(c.At)
	}
	quoted := make([]string, len(c.Exclude))
	for i, pattern := range c.Exclude {
		quoted[i] = quote(pattern)
	}
	return fmt.Sprintf(configTemplate, profile, quote(c.Source), quote(c.Destination),
		quote(c.Subfolder), quote(c.RequireMount), quote(c.Existing), quote(c.Verify),
		quote(c.SettleTime), "["+strings.Join(quoted, ", ")+"]", c.ExcludeHidden, interval, at, c.DryRun, c.Verbose)
}

// quote writes s as a TOML basic string, escaping quotes, backslashes and
// control characters.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f || !unicode.IsPrint(r) && r < 0x10000:
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Save writes the config for profile to file atomically. It refuses to
// replace an existing file unless overwrite is true. The written file is read
// back and checked before it takes the place of the old one.
func (c Config) Save(profile, file string, overwrite bool) error {
	if _, err := os.Lstat(file); err == nil && !overwrite {
		return fmt.Errorf("config %s already exists", file)
	}
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config folder %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(file)+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary config in %s: %w", dir, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(c.Render(profile)); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", tmp.Name(), err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("saving %s to disk: %w", tmp.Name(), err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", tmp.Name(), err)
	}
	if _, err := Load(tmp.Name()); err != nil {
		return fmt.Errorf("checking the written config: %w", err)
	}
	if err := os.Rename(tmp.Name(), file); err != nil {
		return fmt.Errorf("moving config into place at %s: %w", file, err)
	}
	return nil
}
