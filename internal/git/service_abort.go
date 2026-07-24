package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// AbortCherryPick runs `git cherry-pick --abort`, returning the working tree
// and HEAD to the state before the cherry-pick started (HU-006 AC7a). The
// caller marks the run aborted.
func (s *Service) AbortCherryPick(ctx context.Context, dir string) error {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return err
	}
	req := newRequest(root, "cherry-pick", "--abort")
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return fmt.Errorf("git: aborting cherry-pick in %s: %w", root, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("git: aborting cherry-pick in %s: %s", root, strings.TrimSpace(string(result.Stderr)))
	}
	return nil
}

// AppliedPickCount reports how many commits the current branch (HEAD) has
// beyond baseRef — i.e. how many picks were already applied to the temp
// branch in the current sequence — via `git rev-list --count <baseRef>..HEAD`.
// Used to decide whether a mid-sequence abort left partial picks.
func (s *Service) AppliedPickCount(ctx context.Context, dir, baseRef string) (int, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return 0, err
	}
	req := newRequest(root, "rev-list", "--count", baseRef+"..HEAD")
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("git: counting applied picks in %s: %w", root, err)
	}
	if result.ExitCode != 0 {
		return 0, fmt.Errorf("git: counting applied picks in %s: %s", root, strings.TrimSpace(string(result.Stderr)))
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(result.Stdout)))
	if err != nil {
		return 0, fmt.Errorf("git: parsing applied-pick count %q in %s: %w", result.Stdout, root, err)
	}
	return n, nil
}

// OfferPartialBranchCleanup reports whether the user should be offered
// cleanup of the temp promotion branch after an abort: true when at least
// one pick was already applied mid-sequence (HU-006 AC7b). Pure.
func OfferPartialBranchCleanup(appliedCount int) bool {
	return appliedCount > 0
}
