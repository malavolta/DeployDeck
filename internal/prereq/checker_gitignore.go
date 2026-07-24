package prereq

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const deploydeckIgnoreEntry = ".deploydeck/"

// CheckGitignore blocks when .deploydeck/ is not present in the
// repository's .gitignore (HU-001: ".deploydeck/ Gitignore Enforcement
// (Blocking)" — manifests and run history live there and must never
// pollute the working tree the tool itself requires clean).
func (c *Checker) CheckGitignore(ctx context.Context) (PrereqCheck, error) {
	path, content, err := c.readGitignore(ctx)
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

// AddGitignoreEntry appends ".deploydeck/" to the repository's .gitignore
// (creating the file if it does not exist yet), implementing the "offers to
// add the entry" AC. It is a no-op if the entry is already present.
func (c *Checker) AddGitignoreEntry(ctx context.Context) error {
	path, content, err := c.readGitignore(ctx)
	if err != nil {
		return err
	}

	if hasDeploydeckIgnoreEntry(content) {
		return nil
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += deploydeckIgnoreEntry + "\n"

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("prereq: writing %s: %w", path, err)
	}
	return nil
}

func (c *Checker) readGitignore(ctx context.Context) (path string, content string, err error) {
	root, err := c.Git.RepoRoot(ctx, c.Dir)
	if err != nil {
		return "", "", fmt.Errorf("prereq: checking .gitignore: %w", err)
	}

	path = filepath.Join(root, ".gitignore")
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
