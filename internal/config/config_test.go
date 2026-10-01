// Tests for reading, checking and writing the config file.
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"10m", 10 * time.Minute, true}, {"4h", 4 * time.Hour, true}, {"1h30m", 90 * time.Minute, true},
		{"1d", 24 * time.Hour, true}, {"0", 0, true}, {"0s", 0, true}, {"30s", 30 * time.Second, true},
		{"-1h", 0, false}, {"4", 0, false}, {"1.5d", 0, false}, {"inf", 0, false}, {"", 0, false},
	}
	for _, tt := range tests {
		got, err := ParseDuration(tt.in)
		if (err == nil) != tt.ok || got != tt.want {
			t.Errorf("ParseDuration(%q) = %v, %v", tt.in, got, err)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{10 * time.Minute: "10m", 4 * time.Hour: "4h",
		90 * time.Minute: "1h30m", 20 * time.Second: "20s", 48 * time.Hour: "2d", 0: "0s"} {
		if got := FormatDuration(d); got != want {
			t.Errorf("FormatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestParseAt(t *testing.T) {
	for _, ok := range []string{"00:00", "02:00", "23:59"} {
		if _, _, err := ParseAt(ok); err != nil {
			t.Errorf("%s rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"24:00", "2:00", "12:60", "noon", ""} {
		if _, _, err := ParseAt(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "default.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// SAFETY: invariant 1: unknown keys, including any attempt to pass rsync options,
// are errors.
func TestUnknownKeysAreErrors(t *testing.T) {
	for _, extra := range []string{`rsync_options = "--delete"`, `delete = true`, `sourse = "/typo"`} {
		path := writeConfig(t, "source = \"/src\"\narchive = \"/archive\"\n"+extra+"\n")
		if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "unknown setting") {
			t.Errorf("%s: got %v", extra, err)
		}
	}
}

func TestResolveChecksValues(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	good := Config{MachineName: "scope-01", Source: "/src", Archive: "/archive", Existing: "skip",
		Verify: "new", SettleTime: "10m", Interval: "4h", Exclude: []string{"*.tmp"}}
	s, err := good.Resolve("default")
	if err != nil {
		t.Fatal(err)
	}
	if s.MachineDir != "/archive/scope-01" || s.SettleTime != 10*time.Minute || s.Interval != 4*time.Hour {
		t.Fatalf("got %+v", s)
	}
	bad := map[string]func(*Config){
		"relative source": func(c *Config) { c.Source = "data" },
		"no archive":      func(c *Config) { c.Archive = "" },
		"machine slash":   func(c *Config) { c.MachineName = "a/b" },
		"machine dots":    func(c *Config) { c.MachineName = ".." },
		"existing":        func(c *Config) { c.Existing = "overwrite" },
		"verify":          func(c *Config) { c.Verify = "some" },
		"settle":          func(c *Config) { c.SettleTime = "soon" },
		"both schedules":  func(c *Config) { c.At = "02:00" },
		"bad at":          func(c *Config) { c.Interval, c.At = "", "25:00" },
		"bad exclude":     func(c *Config) { c.Exclude = []string{"[abc"} },
		"relative mount":  func(c *Config) { c.RequireMount = "mnt/z" },
	}
	for name, change := range bad {
		c := good
		change(&c)
		if _, err := c.Resolve("default"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "p.toml")
	c := Defaults()
	c.MachineName, c.Source, c.Archive, c.At = "scope-01", `/data/with "quotes" and \back`, "/archive", "02:00"
	c.Exclude = []string{"~$*", "Thumbs.db"}
	if err := c.Save("p", path, false); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != c.Source || got.At != "02:00" || got.Interval != "" || len(got.Exclude) != 2 {
		t.Fatalf("round trip changed values: %+v", got)
	}
	if err := c.Save("p", path, false); err == nil {
		t.Fatal("Save must not overwrite without permission")
	}
}

func TestPathsFollowXDG(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "relative/ignored")
	file, _ := ConfigFile("default")
	state, _ := StateDir("scope2")
	if file != filepath.Join(home, ".config/synctoceph/default.toml") ||
		state != filepath.Join(home, ".local/state/synctoceph/scope2") {
		t.Fatalf("got %s and %s", file, state)
	}
	if _, err := ConfigFile("../escape"); err == nil {
		t.Fatal("profile names must not contain path separators")
	}
}

func TestVerboseIsOnUnlessTurnedOff(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	for content, want := range map[string]bool{"": true, "verbose = true\n": true, "verbose = false\n": false} {
		cfg, err := Load(writeConfig(t, "source = \"/src\"\narchive = \"/archive\"\n"+content))
		if err != nil {
			t.Fatal(err)
		}
		s, err := cfg.Resolve("default")
		if err != nil {
			t.Fatal(err)
		}
		if s.Verbose != want {
			t.Errorf("%q: verbose = %t, want %t", content, s.Verbose, want)
		}
	}
	// A config written by init keeps the choice.
	path := filepath.Join(t.TempDir(), "p.toml")
	c := Defaults()
	c.Source, c.Archive, c.Verbose = "/src", "/archive", false
	if err := c.Save("p", path, false); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || got.Verbose {
		t.Fatalf("verbose = false was not kept: %+v, %v", got, err)
	}
}
