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
	"strings"
	"time"

	"github.com/BurntSushi/toml"

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
	MachineName  string   `toml:"machine_name"`
	Source       string   `toml:"source"`
	Archive      string   `toml:"archive"`
	RequireMount string   `toml:"require_mount"`
	Existing     string   `toml:"existing"`
	Verify       string   `toml:"verify"`
	SettleTime   string   `toml:"settle_time"`
	Exclude      []string `toml:"exclude"`
	Interval     string   `toml:"interval"`
	At           string   `toml:"at"`
	DryRun       bool     `toml:"dry_run"`
	Verbose      bool     `toml:"verbose"`
}

// Defaults returns the built-in values used for keys missing from the file.
// There are deliberately no default data paths: `synctoceph init` must be run.
func Defaults() Config {
	return Config{Existing: ExistingSkip, Verify: VerifyNew, SettleTime: DefaultSettleTime, Verbose: true}
}

// SuggestedExclude is what `init` writes for exclude: Windows thumbnail and
// settings files, and Microsoft Office lock files.
var SuggestedExclude = []string{"Thumbs.db", "desktop.ini", "~$*"}

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
	Profile      string
	MachineName  string
	Source       string
	Archive      string
	MachineDir   string // Archive/MachineName: the only folder this machine writes to
	RequireMount string
	Existing     string
	Verify       string
	SettleTime   time.Duration
	Exclude      []string
	Interval     time.Duration
	At           string
	DryRun       bool
	// Verbose lists every file as it is copied and verified. The -v and -q
	// flags override it for one command.
	Verbose  bool
	StateDir string
}

// machinePattern keeps machine names safe as a single folder name.
var machinePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$`)

// Resolve checks every value and returns the settings for a profile.
func (c Config) Resolve(profile string) (Settings, error) {
	s := Settings{Profile: profile, Existing: c.Existing, Verify: c.Verify,
		Exclude: c.Exclude, At: c.At, DryRun: c.DryRun, Verbose: c.Verbose}
	s.MachineName = c.MachineName
	if s.MachineName == "" {
		s.MachineName = DefaultMachineName()
	}
	if !machinePattern.MatchString(s.MachineName) {
		return s, ui.BadSetting("machine_name", s.MachineName, ui.MachineNameRule)
	}
	var err error
	if s.Source, err = absolutePath("source", c.Source, true); err != nil {
		return s, err
	}
	if s.Archive, err = absolutePath("archive", c.Archive, true); err != nil {
		return s, err
	}
	if s.RequireMount, err = absolutePath("require_mount", c.RequireMount, false); err != nil {
		return s, err
	}
	s.MachineDir = filepath.Join(s.Archive, s.MachineName)
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
		if _, err := path.Match(pattern, ""); err != nil || pattern == "" {
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

// DefaultMachineName returns this computer's host name, changed where needed
// so it is a valid machine_name.
func DefaultMachineName() string {
	host, err := os.Hostname()
	if err != nil {
		return "machine"
	}
	host = strings.Split(host, ".")[0]
	var b strings.Builder
	for _, r := range host {
		if r < 128 && (r == '-' || r == '_' || r == '.' || ('a' <= r && r <= 'z') ||
			('A' <= r && r <= 'Z') || ('0' <= r && r <= '9')) {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	name := strings.TrimLeft(b.String(), "-_.")
	if len(name) > 63 {
		name = name[:63]
	}
	if name == "" {
		return "machine"
	}
	return name
}
