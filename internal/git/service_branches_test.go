package git_test

import (
	"context"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
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

	branches, err := svc.CandidateBranches(context.Background(), dir, "PROJ-1", config.Config{})
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

	candidates, err := svc.CandidateBranches(context.Background(), dir, "PROJ-7", config.Config{})
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

	candidates, err := svc.CandidateBranches(context.Background(), dir, "PROJ-8", config.Config{})
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

// TestService_CandidateBranches_ExcludesPromotionBranch is the D1
// over-exclusion remediation (RED first): proposal D1's repro — a leftover
// promotion branch from a prior run (deploy/DEMO-2-to-INT, matching the
// default config.DefaultBranchFormat's FULL shape) must never surface as a
// spurious second source candidate, in both its bare and origin/-prefixed
// form, while the ticket's real feature branch survives untouched. It also
// proves the fix itself: a LEGITIMATE branch that merely starts with the
// same literal prefix but does NOT match the full "-to-<target>" shape
// (deploy/DEMO-2, with no "-to-" suffix at all) must be KEPT, not dropped —
// a prefix-only match wrongly excluded it before this fix.
func TestService_CandidateBranches_ExcludesPromotionBranch(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()

	// The ticket's genuine feature branch.
	runGit(t, runner, dir, "branch", "feature/DEMO-2")
	// A legitimate branch that merely starts with the promotion prefix but
	// does not match the full promotion-branch shape (no "-to-<target>").
	runGit(t, runner, dir, "branch", "deploy/DEMO-2")
	// A leftover promotion branch from a prior run of the tool itself,
	// present both locally and pushed to origin.
	runGit(t, runner, dir, "branch", "deploy/DEMO-2-to-INT")
	runGit(t, runner, dir, "push", "origin", "deploy/DEMO-2-to-INT")

	svc := git.New(runner)
	cfg := config.Config{BranchFormat: config.DefaultBranchFormat}

	candidates, err := svc.CandidateBranches(context.Background(), dir, "DEMO-2", cfg)
	if err != nil {
		t.Fatalf("expected CandidateBranches to succeed, got error: %v", err)
	}

	var hasFeature, hasPrefixOnly, hasBarePromotion, hasOriginPromotion bool
	for _, b := range candidates {
		switch {
		case b.Name == "feature/DEMO-2" && !b.Remote:
			hasFeature = true
		case b.Name == "deploy/DEMO-2" && !b.Remote:
			hasPrefixOnly = true
		case b.Name == "deploy/DEMO-2-to-INT" && !b.Remote:
			hasBarePromotion = true
		case b.Name == "origin/deploy/DEMO-2-to-INT" && b.Remote:
			hasOriginPromotion = true
		}
	}

	if !hasFeature {
		t.Errorf("expected the real feature branch %q to survive, got %+v", "feature/DEMO-2", candidates)
	}
	if !hasPrefixOnly {
		t.Errorf("expected the prefix-only (non-shape-matching) branch %q to survive, got %+v", "deploy/DEMO-2", candidates)
	}
	if hasBarePromotion {
		t.Errorf("expected local deploy/DEMO-2-to-INT excluded, got %+v", candidates)
	}
	if hasOriginPromotion {
		t.Errorf("expected origin/deploy/DEMO-2-to-INT excluded, got %+v", candidates)
	}
	if len(candidates) != 2 {
		t.Errorf("expected exactly 2 surviving candidates (feature/DEMO-2 + deploy/DEMO-2), got %d: %+v", len(candidates), candidates)
	}
}
