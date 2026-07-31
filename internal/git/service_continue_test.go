package git_test

import (
	"context"
	"errors"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestService_ContinueCherryPick_GatedThenRuns pins AC3+AC5 (tasks
// 10.17/10.18, 10.21/10.22): continue is blocked while a conflict is
// unresolved (and while a staged file still has markers), and runs
// non-interactively only once everything is resolved and staged.
func TestService_ContinueCherryPick_GatedThenRuns(t *testing.T) {
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

	// 1) Unresolved conflict -> continue blocked, no --continue issued.
	if _, err := svc.ContinueCherryPick(context.Background(), dir); !errors.Is(err, git.ErrContinueBlocked) {
		t.Fatalf("expected ErrContinueBlocked while unmerged, got %v", err)
	}
	if mid, _ := svc.RepoState(context.Background(), dir); !mid.InProgress {
		t.Fatalf("expected the pick to still be in progress after a blocked continue")
	}

	// 2) Stage a resolution that STILL has conflict markers -> blocked,
	//    naming the offending file.
	writeFileHelper(t, dir, "a.cls", "l1\n<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> x\nl3\n")
	runGit(t, runner, dir, "add", "a.cls")
	if _, err := svc.ContinueCherryPick(context.Background(), dir); !errors.Is(err, git.ErrContinueBlocked) {
		t.Fatalf("expected ErrContinueBlocked while a staged file has markers, got %v", err)
	}

	// 3) Properly resolve + stage -> gate passes, --continue runs and the
	//    sequence completes non-interactively (no hang).
	writeFileHelper(t, dir, "a.cls", "l1\nRESOLVED\nl3\n")
	runGit(t, runner, dir, "add", "a.cls")
	outcome, err := svc.ContinueCherryPick(context.Background(), dir)
	if err != nil {
		t.Fatalf("expected continue to succeed once resolved, got %v", err)
	}
	if outcome.State.InProgress {
		t.Errorf("expected the sequence to complete after continue, got InProgress=true: %+v", outcome.State)
	}
}
