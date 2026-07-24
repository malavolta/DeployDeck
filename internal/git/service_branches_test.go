package git_test

import (
	"context"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
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
// branches that don't mention it (HU-002: "Buscar ramas remotas y locales
// que contengan el ticket").
func TestService_CandidateBranches_ReturnsLocalAndRemoteMatchesByTicketName(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()

	runGit(t, runner, dir, "branch", "feature/PROJ-1")
	runGit(t, runner, dir, "branch", "hotfix/PROJ-1")
	runGit(t, runner, dir, "push", "origin", "hotfix/PROJ-1")
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
