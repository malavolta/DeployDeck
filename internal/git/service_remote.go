package git

import (
	"context"
	"fmt"
)

// HasRemote reports whether a remote named name is configured in the
// repository containing dir. `git remote get-url <name>` exits non-zero
// (data, not a Runner failure) when the remote does not exist.
func (s *Service) HasRemote(ctx context.Context, dir, name string) (bool, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return false, err
	}

	req := newRequest(root, "remote", "get-url", name)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return false, fmt.Errorf("git: checking remote %q in %s: %w", name, root, err)
	}

	return result.ExitCode == 0, nil
}
