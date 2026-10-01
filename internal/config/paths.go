// This file decides where synctoceph keeps its own files: the config file for
// each profile and the state folder (status, logs, lock). It follows the XDG
// convention used by most Linux tools, so nothing depends on the folder the
// command happens to be run from.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// DefaultProfile is used when --profile is not given.
const DefaultProfile = "default"

// appName is the folder name used under the XDG config and state folders.
const appName = "synctoceph"

// profilePattern limits profile names to characters that are safe in file
// names and systemd unit names.
var profilePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// ValidateProfile checks that a profile name can be used as a file name.
func ValidateProfile(name string) error {
	if !profilePattern.MatchString(name) {
		return ui.BadProfile(name)
	}
	return nil
}

// xdgDir returns the XDG folder named by env, or home/fallback when the
// variable is unset or not an absolute path (the XDG specification says
// relative values must be ignored).
func xdgDir(env, fallback string) (string, error) {
	if value := os.Getenv(env); filepath.IsAbs(value) {
		return filepath.Clean(value), nil
	}
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return "", ui.NoHome(err)
	}
	return filepath.Join(home, fallback), nil
}

// ConfigDir returns the folder that holds one config file per profile.
func ConfigDir() (string, error) {
	dir, err := xdgDir("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appName), nil
}

// ConfigFile returns the path of the config file for a profile.
func ConfigFile(profile string) (string, error) {
	if err := ValidateProfile(profile); err != nil {
		return "", err
	}
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, profile+".toml"), nil
}

// StateRoot returns the folder that holds one state folder per profile.
func StateRoot() (string, error) {
	dir, err := xdgDir("XDG_STATE_HOME", filepath.Join(".local", "state"))
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appName), nil
}

// StateDir returns the state folder for a profile.
func StateDir(profile string) (string, error) {
	if err := ValidateProfile(profile); err != nil {
		return "", err
	}
	root, err := StateRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, profile), nil
}

// Profiles lists the profiles that have a config file, sorted by name.
func Profiles() ([]string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.toml"))
	if err != nil {
		return nil, fmt.Errorf("listing profiles in %s: %w", dir, err)
	}
	var names []string
	for _, match := range matches {
		name := filepath.Base(match)
		name = name[:len(name)-len(".toml")]
		if ValidateProfile(name) == nil {
			names = append(names, name)
		}
	}
	return names, nil
}
