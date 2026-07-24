package git

import (
	"context"
	"fmt"
	"strings"
)

// RepoState reflects the git working tree/index state as read directly
// from the repository (never cached), per the "repo is source of truth"
// decision. This is a partial parse: Clean is enough to distinguish a
// clean working tree from a dirty one. Phase 10 extends this with
// cherry-pick-in-progress fields (InProgress, CurrentSHA, Unmerged,
// SequencerRemaining) reconciled from CHERRY_PICK_HEAD/.git/sequencer.
type RepoState struct {
	// Clean is true when `git status --porcelain` produced no output.
	Clean bool
}

// Status returns the working-tree state for the repository containing dir.
func (s *Service) Status(ctx context.Context, dir string) (RepoState, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return RepoState{}, err
	}

	req := newRequest(root, "status", "--porcelain")
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return RepoState{}, fmt.Errorf("git: status in %s: %w", root, err)
	}
	if result.ExitCode != 0 {
		return RepoState{}, fmt.Errorf("git: status in %s: %s", root, strings.TrimSpace(string(result.Stderr)))
	}

	clean := strings.TrimSpace(string(result.Stdout)) == ""
	return RepoState{Clean: clean}, nil
}
