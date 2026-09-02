package prereq

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const deploydeckIgnoreEntry = ".deploydeck/"

// CheckGitignore blocks when .deploydeck/ is not present in the .gitignore
// governing the ARTIFACTS root, falling back to the git root's .gitignore
// (HU-001: ".deploydeck/ Gitignore Enforcement (Blocking)", corrected by
// directory-resolution's "Gitignore Check Anchored At The Artifacts Root" —
// manifests and run history live there and must never pollute the working
// tree the tool itself requires clean). WHEN ArtifactsRoot == GitRoot (flat
// layout, the default), this is observably identical to today's behavior.
func (c *Checker) CheckGitignore(ctx context.Context) (PrereqCheck, error) {
	path, content, err := c.readGitignore()
	if err != nil {
		return PrereqCheck{}, err
	}

	if hasDeploydeckIgnoreEntry(content) {
		return PrereqCheck{Name: ".deploydeck/ gitignore", Status: StatusOK, Detail: ".deploydeck/ is ignored"}, nil
	}

	return PrereqCheck{
		Name:       ".deploydeck/ gitignore",
		Status:     StatusBlocking,
		Detail:     ".deploydeck/ is not present in .gitignore",
		FixCommand: fmt.Sprintf("echo '%s' >> %s", deploydeckIgnoreEntry, path),
	}, nil
}

// AddGitignoreEntry appends ".deploydeck/" to the .gitignore governing the
// ARTIFACTS root (creating the file if it does not exist yet), implementing
// the "offers to add the entry" AC. It never writes at the git root, even
// when the fallback read found the entry there — the fix target is always
// ArtifactsRoot (ADR-9). It is a no-op if the entry is already present
// (checked via the same two-candidate read CheckGitignore uses).
func (c *Checker) AddGitignoreEntry(ctx context.Context) error {
	_, content, err := c.readGitignore()
	if err != nil {
		return err
	}

	if hasDeploydeckIgnoreEntry(content) {
		return nil
	}

	fixPath, fixContent, err := c.readGitignoreAt(c.ArtifactsRoot)
	if err != nil {
		return err
	}

	if fixContent != "" && !strings.HasSuffix(fixContent, "\n") {
		fixContent += "\n"
	}
	fixContent += deploydeckIgnoreEntry + "\n"

	if err := os.WriteFile(fixPath, []byte(fixContent), 0o644); err != nil {
		return fmt.Errorf("prereq: writing %s: %w", fixPath, err)
	}
	return nil
}

// readGitignore implements ADR-9's two-candidate read: it probes
// ArtifactsRoot's .gitignore first (the PRIMARY candidate, where
// .deploydeck/ actually lives) and, only when GitRoot differs, falls back
// to GitRoot's .gitignore. The entry passing in EITHER file is a pass. This
// is pure filesystem — no git.Service call, no exec — because both roots
// are already resolved by the caller (design.md ADR-9: "the RepoRoot exec
// call is DROPPED").
func (c *Checker) readGitignore() (path string, content string, err error) {
	path, content, err = c.readGitignoreAt(c.ArtifactsRoot)
	if err != nil {
		return "", "", err
	}
	if hasDeploydeckIgnoreEntry(content) {
		return path, content, nil
	}
	if c.GitRoot == c.ArtifactsRoot {
		return path, content, nil
	}

	fallbackPath, fallbackContent, err := c.readGitignoreAt(c.GitRoot)
	if err != nil {
		return "", "", err
	}
	if hasDeploydeckIgnoreEntry(fallbackContent) {
		return fallbackPath, fallbackContent, nil
	}
	// Neither candidate has the entry: report against the PRIMARY
	// (artifacts-root) path — CheckGitignore's FixCommand must always name
	// the artifacts path, never the git root (directory-resolution spec).
	return path, content, nil
}

// readGitignoreAt reads dir's .gitignore, returning ("", "", nil-error) for
// content when the file does not exist yet — an absent .gitignore is not a
// failure, just an empty candidate.
func (c *Checker) readGitignoreAt(dir string) (path string, content string, err error) {
	path = filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, "", nil
		}
		return "", "", fmt.Errorf("prereq: reading %s: %w", path, err)
	}
	return path, string(data), nil
}

// hasDeploydeckIgnoreEntry reports whether content has a non-comment line
// matching ".deploydeck" or ".deploydeck/".
func hasDeploydeckIgnoreEntry(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.TrimSuffix(line, "/") == strings.TrimSuffix(deploydeckIgnoreEntry, "/") {
			return true
		}
	}
	return false
}
