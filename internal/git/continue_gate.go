package git

import (
	"context"
	"fmt"
	"strings"
)

// ContinueGate is the pure decision of whether `git cherry-pick --continue`
// may run (HU-006 AC3). Continue is enabled ONLY when there are zero unmerged
// paths AND no staged file still contains conflict markers. When disabled,
// Pending explains exactly what remains and MarkerFiles names any staged
// files with leftover `<<<<<<<` markers.
type ContinueGate struct {
	Enabled     bool
	Pending     []string
	MarkerFiles []string
}

// EvaluateContinueGate computes the continue-gate from the reconciled repo
// state and the set of staged files still containing conflict markers. Pure.
func EvaluateContinueGate(state RepoState, markerFiles []string) ContinueGate {
	gate := ContinueGate{MarkerFiles: markerFiles}

	for _, f := range state.Unmerged {
		gate.Pending = append(gate.Pending, fmt.Sprintf("%s (%s) is unmerged/unstaged", f.Path, f.Kind))
	}
	for _, m := range markerFiles {
		gate.Pending = append(gate.Pending, fmt.Sprintf("%s still contains conflict markers (<<<<<<<)", m))
	}

	gate.Enabled = len(state.Unmerged) == 0 && len(markerFiles) == 0
	return gate
}

// StagedConflictMarkers returns the staged files that still contain leftover
// conflict markers, via `git diff --cached --check`. An empty result means
// no staged file has markers. A clean tree exits 0; found markers exit 2 —
// both are DATA, never a Runner failure.
func (s *Service) StagedConflictMarkers(ctx context.Context, dir string) ([]string, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return nil, err
	}

	req := newRequest(root, "diff", "--cached", "--check")
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("git: checking staged conflict markers in %s: %w", root, err)
	}
	// Exit 0 (clean) and exit 2 (issues found) are both expected; any other
	// exit is a genuine error.
	if result.ExitCode != 0 && result.ExitCode != 2 {
		return nil, fmt.Errorf("git: checking staged conflict markers in %s: %s", root, strings.TrimSpace(string(result.Stderr)))
	}

	return parseCheckMarkers(result.Stdout), nil
}

// parseCheckMarkers extracts the unique file paths flagged with "leftover
// conflict marker" from `git diff --check` output (lines are
// "<path>:<line>: leftover conflict marker"). Whitespace warnings and other
// lines are ignored. Paths containing spaces are handled by trimming the
// trailing ":<line>: leftover conflict marker" suffix.
func parseCheckMarkers(raw []byte) []string {
	const suffix = ": leftover conflict marker"

	var paths []string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		idx := strings.LastIndex(line, suffix)
		if idx < 0 {
			continue
		}
		prefix := line[:idx] // "<path>:<line>"
		colon := strings.LastIndex(prefix, ":")
		if colon < 0 {
			continue
		}
		path := prefix[:colon]
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}
	return paths
}
