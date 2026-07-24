package git

import (
	"context"
	"fmt"
	"strings"
)

// SearchCommits searches commits whose message contains ticket via
// `git log --all --grep <ticket>`, across all refs (so commits reachable
// only via a branch other than the current HEAD are still found), and
// returns them normalized. No match is not an error: git log --grep exits
// 0 with empty output when nothing matches (verified against git 2.50;
// the only genuinely-nonzero exits here are real failures, e.g. running
// outside a repository, handled the same way as every other Service
// method — never treated as exit-code-as-data like merge-base/cherry-pick
// are).
func (s *Service) SearchCommits(ctx context.Context, dir, ticket string) ([]Commit, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return nil, err
	}

	req := newRequest(root, "log", "--all", "--grep", ticket, "--format="+commitLogFormat)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("git: searching commits for ticket %q in %s: %w", ticket, root, err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("git: searching commits for ticket %q in %s: %s", ticket, root, strings.TrimSpace(string(result.Stderr)))
	}

	commits, err := parseCommitLog(result.Stdout)
	if err != nil {
		return nil, fmt.Errorf("git: searching commits for ticket %q in %s: %w", ticket, root, err)
	}
	return commits, nil
}
