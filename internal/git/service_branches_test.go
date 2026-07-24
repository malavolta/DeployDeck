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
