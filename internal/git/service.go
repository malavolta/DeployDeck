// Package git wraps the git CLI through internal/exec.Runner: repo/worktree
// validation, branch listing, commit search, and (later phases) the
// cherry-pick engine and repo-state reconciliation. It is the sole
// git-aware layer; internal/app composes it and never execs directly.
package git

import (
	"deploydeck/internal/exec"
)

// Service wraps a Runner to provide git operations. It carries no directory
// state: every method takes an explicit starting directory and resolves the
// repository root itself (via RepoRoot) rather than trusting a cached or
// shell-cd'd working directory.
type Service struct {
	runner exec.Runner
}

// New returns a Service backed by runner.
func New(runner exec.Runner) *Service {
	return &Service{runner: runner}
}
