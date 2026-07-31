package git_test

import (
	"context"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

func TestService_ListBranches_ReturnsLocalAndRemoteBranches(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()

	// A second local branch that is never pushed, to prove local-only
	// branches are listed alongside remote-tracking ones.
	runGit(t, runner, dir, "branch", "feature/OTACUPYR-123")

	svc := git.New(runner)

	branches, err := svc.ListBranches(context.Background(), dir)
	if err != nil {
		t.Fatalf("expected ListBranches to succeed, got error: %v", err)
	}

	var hasLocalMain, hasLocalFeature, hasRemoteMain bool
	for _, b := range branches {
		switch {
		case b.Name == "main" && !b.Remote:
			hasLocalMain = true
		case b.Name == "feature/OTACUPYR-123" && !b.Remote:
			hasLocalFeature = true
		case b.Name == "origin/main" && b.Remote:
			hasRemoteMain = true
		}
	}

	if !hasLocalMain {
		t.Errorf("expected local branch %q in %+v", "main", branches)
	}
	if !hasLocalFeature {
		t.Errorf("expected local branch %q in %+v", "feature/OTACUPYR-123", branches)
	}
	if !hasRemoteMain {
		t.Errorf("expected remote branch %q in %+v", "origin/main", branches)
	}
}

// TestService_CandidateBranches_ReturnsLocalAndRemoteMatchesByTicketName
// proves CandidateBranches finds both a local-only branch and a
// remote-tracking branch whose name contains the ticket, while excluding
// branches that don't mention it. hotfix/PROJ-1 is pushed directly to a NEW
// remote ref (no local branch ever created), so it stays genuinely
// remote-only — the local+origin twin case is covered separately by
// TestService_CandidateBranches_DedupesLocalAndRemoteTrackingOfSameBranch.
func TestService_CandidateBranches_ReturnsLocalAndRemoteMatchesByTicketName(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()

	runGit(t, runner, dir, "branch", "feature/PROJ-1")
	runGit(t, runner, dir, "push", "origin", "HEAD:refs/heads/hotfix/PROJ-1")
	runGit(t, runner, dir, "branch", "feature/PROJ-2")

	svc := git.New(runner)

	branches, err := svc.CandidateBranches(context.Background(), dir, "PROJ-1")
	if err != nil {
		t.Fatalf("expected CandidateBranches to succeed, got error: %v", err)
	}

	var hasLocalFeature, hasRemoteHotfix, hasUnrelated bool
	for _, b := range branches {
		switch {
		case b.Name == "feature/PROJ-1" && !b.Remote:
			hasLocalFeature = true
		case b.Name == "origin/hotfix/PROJ-1" && b.Remote:
			hasRemoteHotfix = true
		case b.Name == "feature/PROJ-2":
			hasUnrelated = true
		}
	}

	if !hasLocalFeature {
		t.Errorf("expected local candidate %q in %+v", "feature/PROJ-1", branches)
	}
	if !hasRemoteHotfix {
		t.Errorf("expected remote candidate %q in %+v", "origin/hotfix/PROJ-1", branches)
	}
	if hasUnrelated {
		t.Errorf("expected feature/PROJ-2 excluded (does not contain the searched ticket), got %+v", branches)
	}
}

// TestService_CandidateBranches_DedupesLocalAndRemoteTrackingOfSameBranch is
// task 1.1 (RED): a pushed feature branch present both locally and as its
// origin/ counterpart collapses into ONE candidate (the bare/local form)
// instead of being counted twice (proposal D1 repro).
func TestService_CandidateBranches_DedupesLocalAndRemoteTrackingOfSameBranch(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()

	runGit(t, runner, dir, "branch", "feature/PROJ-7")
	runGit(t, runner, dir, "push", "origin", "feature/PROJ-7")

	svc := git.New(runner)

	candidates, err := svc.CandidateBranches(context.Background(), dir, "PROJ-7")
	if err != nil {
		t.Fatalf("expected CandidateBranches to succeed, got error: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected exactly 1 deduped candidate for a local+origin twin, got %d: %+v", len(candidates), candidates)
	}
	if got := candidates[0]; got.Name != "feature/PROJ-7" || got.Remote {
		t.Fatalf("expected the surviving candidate to be the bare/local form {feature/PROJ-7, Remote:false}, got %+v", got)
	}
}

// TestService_CandidateBranches_DoesNotOverCollapseDistinctNames is task 1.2
// (RED): the over-aggression guard — two genuinely distinct logical names
// must remain two separate candidates after dedupe.
func TestService_CandidateBranches_DoesNotOverCollapseDistinctNames(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()

	runGit(t, runner, dir, "branch", "feature/PROJ-8-a")
	runGit(t, runner, dir, "branch", "hotfix/PROJ-8-b")
	runGit(t, runner, dir, "push", "origin", "hotfix/PROJ-8-b")

	svc := git.New(runner)

	candidates, err := svc.CandidateBranches(context.Background(), dir, "PROJ-8")
	if err != nil {
		t.Fatalf("expected CandidateBranches to succeed, got error: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 distinct candidates (never collapsed), got %d: %+v", len(candidates), candidates)
	}

	// hotfix/PROJ-8-b was itself pushed (its own twin dedupes to its bare
	// form); it must survive as its OWN candidate, never collapsed into a.
	var hasA, hasB bool
	for _, b := range candidates {
		switch {
		case b.Name == "feature/PROJ-8-a" && !b.Remote:
			hasA = true
		case b.Name == "hotfix/PROJ-8-b" && !b.Remote:
			hasB = true
		}
	}
	if !hasA || !hasB {
		t.Fatalf("expected both distinct candidates preserved untouched, got %+v", candidates)
	}
}

// Task 1.3's guard: TestService_ListBranches_ReturnsLocalAndRemoteBranches
// (above, unmodified) already proves ListBranches returns local "main" AND
// remote-tracking "origin/main" as separate rows — dedupe never leaks there.
