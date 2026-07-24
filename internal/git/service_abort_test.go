package git_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

// TestService_AbortCherryPick_ReturnsToPrePickState pins AC7a (tasks
// 10.25/10.26): a confirmed abort runs `git cherry-pick --abort` and the
// repo returns to a clean, not-in-progress state.
func TestService_AbortCherryPick_ReturnsToPrePickState(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	feature1, feature2 := seedConflictingFeature(t, runner, dir)
	commits := []git.DiscoveredCommit{
		{Commit: git.Commit{SHA: feature1}},
		{Commit: git.Commit{SHA: feature2}},
	}
	if _, err := svc.CherryPick(context.Background(), dir, commits, true); err != nil {
		t.Fatalf("CherryPick error: %v", err)
	}

	if err := svc.AbortCherryPick(context.Background(), dir); err != nil {
		t.Fatalf("AbortCherryPick error: %v", err)
	}

	state, _ := svc.RepoState(context.Background(), dir)
	if state.InProgress {
		t.Errorf("expected InProgress=false after abort")
	}
	if !state.Clean {
		t.Errorf("expected a clean tree after abort")
	}
}

// TestService_AbortCherryPick_MidSequence_OffersBranchCleanup pins AC7b
// (tasks 10.27/10.28): when the abort happens after some commits were already
// picked, the tool detects the partial picks and offers temp-branch cleanup.
func TestService_AbortCherryPick_MidSequence_OffersBranchCleanup(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	feature1, feature2 := seedPartialThenConflict(t, runner, dir)
	commits := []git.DiscoveredCommit{
		{Commit: git.Commit{SHA: feature1}},
		{Commit: git.Commit{SHA: feature2}},
	}
	outcome, err := svc.CherryPick(context.Background(), dir, commits, true)
	if err != nil {
		t.Fatalf("CherryPick error: %v", err)
	}
	if !outcome.State.InProgress || len(outcome.State.Unmerged) == 0 {
		t.Fatalf("expected a mid-sequence conflict (first pick applied, second conflicts): %+v", outcome.State)
	}

	// The first pick (c.cls) was already applied to the temp branch.
	applied, err := svc.AppliedPickCount(context.Background(), dir, "origin/UAT")
	if err != nil {
		t.Fatalf("AppliedPickCount error: %v", err)
	}
	if applied < 1 {
		t.Fatalf("expected at least one applied pick before abort, got %d", applied)
	}
	if !git.OfferPartialBranchCleanup(applied) {
		t.Errorf("expected a partial-branch cleanup offer when %d picks were applied", applied)
	}

	if err := svc.AbortCherryPick(context.Background(), dir); err != nil {
		t.Fatalf("AbortCherryPick error: %v", err)
	}
	state, _ := svc.RepoState(context.Background(), dir)
	if state.InProgress {
		t.Errorf("expected InProgress=false after mid-sequence abort")
	}
}

// TestService_EmptyPick_DetectedAndSkipped pins AC8 (tasks 10.29/10.30): a
// pick that becomes empty because the content is already in the target is
// detected with an explanatory message, and `--skip` drops it and lets the
// sequence complete (the squash safety-net).
func TestService_EmptyPick_DetectedAndSkipped(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	feature1, feature2 := seedEmptyThenClean(t, runner, dir)
	commits := []git.DiscoveredCommit{
		{Commit: git.Commit{SHA: feature1}},
		{Commit: git.Commit{SHA: feature2}},
	}
	outcome, err := svc.CherryPick(context.Background(), dir, commits, true)
	if err != nil {
		t.Fatalf("CherryPick error: %v", err)
	}
	if !outcome.Empty {
		t.Fatalf("expected the first pick to be detected as empty, got %+v", outcome)
	}
	if outcome.EmptyMessage == "" {
		t.Errorf("expected an explanatory empty-pick message")
	}
	if len(outcome.State.Unmerged) != 0 {
		t.Errorf("an empty pick has no conflicts, got %+v", outcome.State.Unmerged)
	}

	// Offer accepted: skip the empty pick; the clean second commit applies
	// and the sequence completes.
	skipped, err := svc.SkipCherryPick(context.Background(), dir)
	if err != nil {
		t.Fatalf("SkipCherryPick error: %v", err)
	}
	if skipped.State.InProgress {
		t.Errorf("expected the sequence to complete after skipping the empty pick, got %+v", skipped.State)
	}
	if _, err := os.Stat(filepath.Join(dir, "d.cls")); err != nil {
		t.Errorf("expected the clean second commit (d.cls) applied after skip: %v", err)
	}
}
