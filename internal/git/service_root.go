package git

import (
	"context"
	"fmt"
	"strings"
)

// RepoRoot resolves the git repository root containing dir via
// `git rev-parse --show-toplevel`. It errors when dir is not inside a git
// repository. Every other Service method resolves the root through this
// method and uses the RESOLVED path as the exec request's Dir, rather than
// trusting the caller-provided dir directly or shelling out to `cd`.
func (s *Service) RepoRoot(ctx context.Context, dir string) (string, error) {
	req := newRequest(dir, "rev-parse", "--show-toplevel")

	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return "", fmt.Errorf("git: resolving repo root for %s: %w", dir, err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("git: %s is not inside a git repository: %s", dir, strings.TrimSpace(string(result.Stderr)))
	}

	return strings.TrimSpace(string(result.Stdout)), nil
}
