package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ProjectRoot resolves the SFDX project root from gitRoot and c.ProjectDir
// (design.md "Interfaces / Contracts"). An empty ProjectDir means the SFDX
// project root IS configDir — today's flat-layout behavior, unchanged
// (directory-resolution spec: "projectDir Defaults To The Config File's
// Directory"). A non-empty ProjectDir is interpreted RELATIVE TO gitRoot,
// never configDir — see the ProjectDir field's own doc comment.
//
// This is the resolution path production actually executes (cmd/deploydeck's
// resolveRoots), so the absolute/".." guard lives here, not only inside
// Validate() — see this package's Validate doc comment and the deferred
// follow-up recorded in this change's proposal.
func (c Config) ProjectRoot(gitRoot, configDir string) (string, error) {
	if c.ProjectDir == "" {
		return configDir, nil
	}
	if err := validateProjectDir(c.ProjectDir); err != nil {
		return "", err
	}
	return filepath.Join(gitRoot, c.ProjectDir), nil
}

// validateProjectDir rejects an absolute projectDir or one containing a
// ".." path segment (directory-resolution spec's "Path Traversal Via
// Config" threat). Shared by ProjectRoot (the path production actually
// executes) and Validate (a secondary surfacing point, task 1.7) so the two
// entry points enforce exactly the same rule.
func validateProjectDir(v string) error {
	if v == "" {
		return nil
	}
	if filepath.IsAbs(v) {
		return fmt.Errorf("config: projectDir %q must be a relative path, not absolute", v)
	}
	for _, segment := range strings.Split(filepath.ToSlash(v), "/") {
		if segment == ".." {
			return fmt.Errorf("config: projectDir %q must not contain a %q segment", v, "..")
		}
	}
	return nil
}
