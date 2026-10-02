// This file defines the config file: which keys exist, their defaults, and
// the checks that run before any command uses them. The file is strict:
// unknown keys are errors, so a typo can never silently change behaviour and
// nobody can pass extra options through to rsync.
package config

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/destination"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// Allowed values for the existing and verify settings.
const (
	ExistingSkip    = "skip"
	ExistingReplace = "replace"
	VerifyNew       = "new"
	VerifyAll       = "all"
)

// DefaultSettleTime is how long a file must be unchanged before it is copied.
// Files modified more recently are probably still being written by the
// acquisition software, so they are deferred to a later run.
const DefaultSettleTime = "10m"

// minInterval is the shortest schedule interval accepted.
const minInterval = time.Second

// Config mirrors the config file. Every field is a key in the TOML file.
type Config struct {
	Subfolder    string `toml:"subfolder"`
	Source       string `toml:"source"`
	Destination  string `toml:"destination"`
	RequireMount string `toml:"require_mount"`
	Existing     string `toml:"existing"`
	Verify       string `toml:"verify"`
	SettleTime   string `toml:"settle_time"`
	// Exclude holds the exclude patterns, including HiddenPattern when
	// exclude_hidden is on.
	Exclude []string `toml:"exclude"`
	// ExcludeHidden leaves out every file and folder whose name starts with
	// "." (such as .git or .DS_Store), as if ".*" were in Exclude.
	ExcludeHidden bool   `toml:"exclude_hidden"`
	Interval      string `toml:"interval"`
	At            string `toml:"at"`
	DryRun        bool   `toml:"dry_run"`
	Verbose       bool   `toml:"verbose"`
}

// Defaults returns the built-in values used for keys missing from the file.
// There are deliberately no default data paths: `synctoceph init` must be run.
func Defaults() Config {
	return Config{Existing: ExistingSkip, Verify: VerifyNew, SettleTime: DefaultSettleTime, Verbose: true}
}

// SuggestedExclude is what `init` writes for exclude: Windows thumbnail and
// settings files, and Microsoft Office lock files. `init` also turns on
// exclude_hidden.
var SuggestedExclude = []string{"Thumbs.db", "desktop.ini", "~$*"}

// HiddenPattern is the exclude pattern that exclude_hidden adds: any name
// starting with a dot.
const HiddenPattern = ".*"

// Load reads a config file on top of the defaults. Unknown keys are errors.
func Load(file string) (Config, error) {
	cfg := Defaults()
	meta, err := toml.DecodeFile(file, &cfg)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, ui.NoConfig(file)
	}
	if err != nil {
		return cfg, ui.BadConfigFile(file, err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, key := range undecoded {
			keys[i] = key.String()
		}
		return cfg, ui.UnknownConfigKeys(file, keys)
	}
	return cfg, nil
}

// Settings are the checked, ready-to-use values for one profile, after
// command-line flags, the config file and defaults have been combined.
type Settings struct {
	Profile string
	// Subfolder is this profile's folder inside every animal folder. Copies
	// go only to <Destination>/<animal>/<Subfolder>/.
	Subfolder   string
	Source      string // holds one folder per animal
	Destination string // holds one folder per animal, plus .syncToCeph/
	// MetaDir is <Destination>/.syncToCeph/<Subfolder>: the verified-file
	// record, history and run summaries of this subfolder.
	MetaDir      string
	RequireMount string
	Existing     string
	Verify       string
	SettleTime   time.Duration
	// Exclude holds the exclude patterns, including HiddenPattern when
	// exclude_hidden is on.
	Exclude  []string
	Interval time.Duration
	At       string
	DryRun   bool
	// Verbose lists every file as it is copied and verified. The -v and -q
	// flags override it for one command.
	Verbose  bool
	StateDir string
}

// subfolderPattern keeps subfolder names safe as a single folder name.
var subfolderPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$`)

// Resolve checks every value and returns the settings for a profile.
func (c Config) Resolve(profile string) (Settings, error) {
	s := Settings{Profile: profile, Existing: c.Existing, Verify: c.Verify,
		Exclude: slices.Clone(c.Exclude), At: c.At, DryRun: c.DryRun, Verbose: c.Verbose}
	if c.ExcludeHidden {
		s.Exclude = append(s.Exclude, HiddenPattern)
	}
	s.Subfolder = c.Subfolder
	if s.Subfolder == "" {
		return s, ui.MissingSetting("subfolder")
	}
	if !subfolderPattern.MatchString(s.Subfolder) {
		return s, ui.BadSetting("subfolder", s.Subfolder, ui.SubfolderRule)
	}
	var err error
	if s.Source, err = absolutePath("source", c.Source, true); err != nil {
		return s, err
	}
	if s.Destination, err = absolutePath("destination", c.Destination, true); err != nil {
		return s, err
	}
	if s.RequireMount, err = absolutePath("require_mount", c.RequireMount, false); err != nil {
		return s, err
	}
	s.MetaDir = destination.MetaDir(s.Destination, s.Subfolder)
	if s.Existing != ExistingSkip && s.Existing != ExistingReplace {
		return s, ui.BadSetting("existing", s.Existing, "use skip or replace")
	}
	if s.Verify != VerifyNew && s.Verify != VerifyAll {
		return s, ui.BadSetting("verify", s.Verify, "use new or all")
	}
	if s.SettleTime, err = ParseDuration(c.SettleTime); err != nil {
		return s, ui.BadSetting("settle_time", c.SettleTime, err.Error())
	}
	if err := c.checkSchedule(&s); err != nil {
		return s, err
	}
	for _, pattern := range s.Exclude {
		if _, err := path.Match(pattern, ""); err != nil || strings.Trim(pattern, "/") == "" {
			return s, ui.BadSetting("exclude", pattern, ui.ExcludeRule)
		}
	}
	if s.StateDir, err = StateDir(profile); err != nil {
		return s, err
	}
	return s, nil
}

// checkSchedule validates interval and at; at most one may be set.
func (c Config) checkSchedule(s *Settings) error {
	if c.Interval != "" && c.At != "" {
		return ui.BothSchedules()
	}
	if c.Interval != "" {
		d, err := ParseDuration(c.Interval)
		if err == nil && d < minInterval {
			err = errors.New("the interval must be at least 1s")
		}
		if err != nil {
			return ui.BadSetting("interval", c.Interval, err.Error())
		}
		s.Interval = d
	}
	if c.At != "" {
		if _, _, err := ParseAt(c.At); err != nil {
			return ui.BadSetting("at", c.At, err.Error())
		}
	}
	return nil
}

// absolutePath checks that a configured folder is an absolute path and
// returns it cleaned (no trailing slash, no "..").
func absolutePath(key, value string, required bool) (string, error) {
	if value == "" {
		if required {
			return "", ui.MissingSetting(key)
		}
		return "", nil
	}
	if !filepath.IsAbs(value) {
		return "", ui.BadSetting(key, value, ui.AbsolutePathRule)
	}
	return filepath.Clean(value), nil
}

// DestRel returns where a source file goes, relative to the destination:
// "LUMS0014/session1/a.bin" goes to "LUMS0014/<subfolder>/session1/a.bin".
func (s Settings) DestRel(rel string) string { return destination.Rel(s.Subfolder, rel) }

// Target describes where data goes, for messages, e.g.
// "/mnt/ceph/project/<animal>/behaviour".
func (s Settings) Target() string {
	return filepath.Join(s.Destination, "<animal>", s.Subfolder)
}
