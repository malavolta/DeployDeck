package git

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// rerereResolvedRe matches git's "Resolved '<path>' using previous
// resolution." lines, emitted when rerere auto-resolves a conflict from a
// recorded prior resolution.
var rerereResolvedRe = regexp.MustCompile(`Resolved '(.+?)' using previous resolution\.`)

// rerereResolvedPaths returns the unique paths git's rerere auto-resolved in
// this command's output. When non-empty, those files were resolved from a
// PRIOR resolution and must be presented to the user as auto-resolved and
// require explicit confirmation before continuing (HU-006 AC9). Pure.
func rerereResolvedPaths(output string) []string {
	matches := rerereResolvedRe.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var paths []string
	for _, m := range matches {
		p := m[1]
		if seen[p] {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	return paths
}

// RerereEnabled reports whether `git rerere` is enabled in the repo, via
// `git config --get rerere.enabled` (exit 0 with "true" => enabled; exit 1
// => unset/false, which is DATA, not an error).
func (s *Service) RerereEnabled(ctx context.Context, dir string) (bool, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return false, err
	}
	req := newRequest(root, "config", "--get", "rerere.enabled")
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return false, fmt.Errorf("git: reading rerere.enabled in %s: %w", root, err)
	}
	switch result.ExitCode {
	case 0:
		return strings.TrimSpace(string(result.Stdout)) == "true", nil
	case 1:
		return false, nil
	default:
		return false, fmt.Errorf("git: reading rerere.enabled in %s: %s", root, strings.TrimSpace(string(result.Stderr)))
	}
}

// SuggestEnableRerere reports whether the tool should suggest enabling
// `git rerere` (i.e. it is not already enabled). Pure.
func SuggestEnableRerere(enabled bool) bool {
	return !enabled
}
