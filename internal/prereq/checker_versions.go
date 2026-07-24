package prereq

import (
	"context"
	"fmt"

	"deploydeck/internal/salesforce"
)

// FixCommand suggestions for below-minimum/missing tool versions.
const (
	fixCommandGitUpgrade  = "https://git-scm.com/downloads"
	fixCommandSFUpgrade   = "sf update"
	fixCommandDeltaManage = "sf plugins install sfdx-git-delta"
)

// pluginNameDelta is the sfdx-git-delta plugin name as reported by
// `sf plugins --json`.
const pluginNameDelta = "sfdx-git-delta"

// CheckVersions validates the installed git, sf and sfdx-git-delta versions
// against Config.MinVersions, and sfdx-git-delta plugin presence
// (HU-001: "Binary, Plugin, And Minimum Version Validation").
//
// A binary that cannot even be launched (missing from PATH) is reported as
// a blocking PrereqCheck with a corrective FixCommand rather than as a Go
// error, so Check() can still render a full report naming every other
// prerequisite's state (HU-001: "Missing git binary blocks the flow").
func (c *Checker) CheckVersions(ctx context.Context) ([]PrereqCheck, error) {
	var checks []PrereqCheck

	gitVersion, err := c.Git.Version(ctx)
	if err != nil {
		checks = append(checks, PrereqCheck{
			Name:       "git version",
			Status:     StatusBlocking,
			Detail:     fmt.Sprintf("git is not available: %v", err),
			FixCommand: fixCommandGitUpgrade,
		})
	} else {
		checks = append(checks, versionCheck("git", gitVersion, c.Config.MinVersions["git"], fixCommandGitUpgrade))
	}

	sfInfo, sfErr := c.SF.Version(ctx)
	if sfErr != nil {
		checks = append(checks, PrereqCheck{
			Name:       "sf version",
			Status:     StatusBlocking,
			Detail:     fmt.Sprintf("sf is not available: %v", sfErr),
			FixCommand: fixCommandSFUpgrade,
		})
		// Without a working sf CLI, plugin presence cannot be determined
		// either — skip it rather than emitting a second, redundant
		// blocking check for the same root cause.
		return checks, nil
	}
	checks = append(checks, versionCheck("sf", sfInfo.CLIVersion, c.Config.MinVersions["sf"], fixCommandSFUpgrade))

	plugins, err := c.SF.Plugins(ctx)
	if err != nil {
		checks = append(checks, PrereqCheck{
			Name:       pluginNameDelta + " plugin",
			Status:     StatusBlocking,
			Detail:     fmt.Sprintf("could not list sf plugins: %v", err),
			FixCommand: fixCommandDeltaManage,
		})
		return checks, nil
	}
	checks = append(checks, deltaPluginCheck(plugins, c.Config.MinVersions[pluginNameDelta]))

	return checks, nil
}

// versionCheck compares installed against min (when min is configured) and
// reports a blocking check with fixCommand when installed is lower.
func versionCheck(tool, installed, min, fixCommand string) PrereqCheck {
	name := tool + " version"
	if min == "" || installed == "" {
		return PrereqCheck{Name: name, Status: StatusOK, Detail: fmt.Sprintf("%s %s", tool, installed)}
	}
	if compareVersions(installed, min) < 0 {
		return PrereqCheck{
			Name:       name,
			Status:     StatusBlocking,
			Detail:     fmt.Sprintf("%s %s installed, minimum %s required", tool, installed, min),
			FixCommand: fixCommand,
		}
	}
	return PrereqCheck{Name: name, Status: StatusOK, Detail: fmt.Sprintf("%s %s (minimum %s)", tool, installed, min)}
}

// deltaPluginCheck reports the sfdx-git-delta plugin's presence and, when
// present and a minimum is configured, its version.
func deltaPluginCheck(plugins []salesforce.Plugin, min string) PrereqCheck {
	for _, p := range plugins {
		if p.Name != pluginNameDelta {
			continue
		}
		if min != "" && compareVersions(p.Version, min) < 0 {
			return PrereqCheck{
				Name:       pluginNameDelta + " version",
				Status:     StatusBlocking,
				Detail:     fmt.Sprintf("%s %s installed, minimum %s required", pluginNameDelta, p.Version, min),
				FixCommand: fixCommandDeltaManage,
			}
		}
		return PrereqCheck{Name: pluginNameDelta + " version", Status: StatusOK, Detail: fmt.Sprintf("%s %s", pluginNameDelta, p.Version)}
	}

	return PrereqCheck{
		Name:       pluginNameDelta + " plugin",
		Status:     StatusBlocking,
		Detail:     pluginNameDelta + " plugin is not installed",
		FixCommand: fixCommandDeltaManage,
	}
}
