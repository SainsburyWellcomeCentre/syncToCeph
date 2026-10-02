// This file holds the parts of `synctoceph doctor` that look at several
// profiles: checking every profile at once (--all-profiles), and noticing
// when two profiles copy into the same subfolder of the same destination.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/config"
	"github.com/SainsburyWellcomeCentre/syncToCeph/internal/ui"
)

// profileChecks is one profile's part of `doctor --all-profiles --json`.
type profileChecks struct {
	Profile  string  `json:"profile"`
	Problems int     `json:"problems"`
	Checks   []check `json:"checks"`
}

// doctorAllProfiles checks every profile in turn.
func doctorAllProfiles(cmd *cobra.Command, g *globals, asJSON bool) error {
	profiles, err := allProfiles(cmd)
	if err != nil {
		return err
	}
	p := printer(cmd, g)
	var results []profileChecks
	total := 0
	for i, profile := range profiles {
		d := diagnose(profile)
		total += d.problems()
		results = append(results, profileChecks{Profile: profile, Problems: d.problems(), Checks: d.checks})
		if asJSON {
			continue
		}
		if i > 0 {
			p.Blank()
		}
		ui.ProfileHeading(p, profile, i+1, len(profiles))
		d.print(p)
	}
	if asJSON {
		writeJSON(cmd.OutOrStdout(), map[string]any{"schema_version": JSONSchemaVersion,
			"problems": total, "profiles": results})
	} else {
		p.Blank()
		p.Line(ui.MarkResult, ui.DoctorResult(total))
	}
	return exitWith(min(total, 1))
}

// sharingProfiles returns the other profiles that copy into the same
// subfolder of the same destination as s. Profiles whose config has a problem are left out
// (doctor reports those on their own).
func sharingProfiles(s config.Settings) []string {
	profiles, err := config.Profiles()
	if err != nil {
		return nil
	}
	var others []string
	for _, name := range profiles {
		if name == s.Profile {
			continue
		}
		if other, err := loadProfile(name, nil); err == nil && other.MetaDir == s.MetaDir {
			others = append(others, name)
		}
	}
	return others
}
