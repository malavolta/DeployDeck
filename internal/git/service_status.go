package git

import (
	"context"
	"fmt"
	"strings"
)

// RepoState reflects the git working tree/index state as read directly
// from the repository (never cached), per the "repo is source of truth"
// decision. Status() populates only Clean; RepoState() (Phase 10) also
// reconciles the cherry-pick-in-progress fields from CHERRY_PICK_HEAD,
// .git/sequencer, and `git status --porcelain -z`.
type RepoState struct {
	// Clean is true when `git status --porcelain` produced no output.
	Clean bool

	// InProgress is true when a cherry-pick is underway, i.e. the
	// CHERRY_PICK_HEAD ref exists in the git dir.
	InProgress bool
	// CurrentSHA is the full SHA of the commit currently being
	// cherry-picked (the CHERRY_PICK_HEAD content), empty when idle.
	CurrentSHA string
	// Unmerged holds the classified conflict files (from
	// `git status --porcelain -z` unmerged entries), empty when there are
	// no conflicts.
	Unmerged []ConflictFile
	// SequencerRemaining is the number of pending pick instructions in
	// .git/sequencer/todo (includes the currently-conflicting commit).
	// It is 0 for a single-commit pick or when no sequence is active.
	SequencerRemaining int
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
