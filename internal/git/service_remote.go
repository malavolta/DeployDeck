package git

import (
	"context"
	"fmt"
	"strings"
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

// RemoteURL returns the trimmed URL configured for remote name in the
// repository containing dir, via `git remote get-url <name>` — a sibling of
// HasRemote, but returning the URL itself rather than a boolean presence
// check. HU-014's CompareURL normalizer consumes this value to derive the
// PR compare link's host/org/repo.
func (s *Service) RemoteURL(ctx context.Context, dir, name string) (string, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return "", err
	}

	req := newRequest(root, "remote", "get-url", name)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return "", fmt.Errorf("git: getting remote %q url in %s: %w", name, root, err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("git: getting remote %q url in %s: %s", name, root, strings.TrimSpace(string(result.Stderr)))
	}

	return strings.TrimSpace(string(result.Stdout)), nil
}
