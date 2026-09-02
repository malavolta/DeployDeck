package prereq

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// interferingHookNames are git hooks that can intercept or block the
// checkout/cherry-pick operations DeployDeck relies on.
var interferingHookNames = map[string]bool{
	"pre-commit":         true,
	"commit-msg":         true,
	"prepare-commit-msg": true,
	"post-commit":        true,
	"post-checkout":      true,
	"post-merge":         true,
	"pre-rebase":         true,
}

// CheckHooks detects repository git hooks that could interfere with
// checkout or cherry-pick (HU-001 / RNF-005). This check is ALWAYS
// informative — it never blocks.
func (c *Checker) CheckHooks(ctx context.Context) (PrereqCheck, error) {
	root, err := c.Git.RepoRoot(ctx, c.GitRoot)
	if err != nil {
		return PrereqCheck{}, fmt.Errorf("prereq: checking git hooks: %w", err)
	}

	hooksDir := filepath.Join(root, ".git", "hooks")
	hooks, err := DetectInterferingHooks(hooksDir)
	if err != nil {
		return PrereqCheck{}, err
	}

	if len(hooks) == 0 {
		return PrereqCheck{Name: "git hooks", Status: StatusOK, Detail: "no interfering git hooks detected"}, nil
	}

	return PrereqCheck{
		Name:   "git hooks",
		Status: StatusWarning,
		Detail: fmt.Sprintf("hooks that may interfere with checkout/cherry-pick: %s", strings.Join(hooks, ", ")),
	}, nil
}

// DetectInterferingHooks scans hooksDir for non-sample hook files matching
// interferingHookNames and returns their names, sorted. A missing hooks
// directory is not an error — it simply means no hooks are installed.
func DetectInterferingHooks(hooksDir string) ([]string, error) {
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("prereq: reading hooks directory %s: %w", hooksDir, err)
	}

	var found []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".sample") {
			continue
		}
		if interferingHookNames[name] {
			found = append(found, name)
		}
	}

	sort.Strings(found)
	return found, nil
}
