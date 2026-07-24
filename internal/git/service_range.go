package git

import (
	"context"
	"fmt"
	"strings"
)

// CommitsInRange returns commits reachable from origin/<source> but not
// from origin/<target>, ordered via
// `git rev-list --reverse --topo-order origin/<target>..origin/<source>`
// — NEVER by author or commit date, which do not survive rebases/amends
// reliably (HU-002). --no-commit-header strips rev-list's normal
// "commit <sha>" prefix line so the same commitLogFormat/parseCommitLog
// pair SearchCommits uses parses this output too, in one command per
// call (verified against git 2.50; --no-commit-header has been available
// since git 2.34).
func (s *Service) CommitsInRange(ctx context.Context, dir, target, source string) ([]Commit, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return nil, err
	}

	rangeArg := fmt.Sprintf("origin/%s..origin/%s", target, source)
	req := newRequest(root,
		"rev-list", "--reverse", "--topo-order", "--no-commit-header",
		"--format="+commitLogFormat, rangeArg,
	)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("git: listing commits in range %s in %s: %w", rangeArg, root, err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("git: listing commits in range %s in %s: %s", rangeArg, root, strings.TrimSpace(string(result.Stderr)))
	}

	commits, err := parseCommitLog(result.Stdout)
	if err != nil {
		return nil, fmt.Errorf("git: listing commits in range %s in %s: %w", rangeArg, root, err)
	}
	return commits, nil
}
