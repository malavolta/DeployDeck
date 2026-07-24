package git_test

import (
	"context"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

// TestService_CherryPick_RerereAutoResolves_RequiresConfirmation pins AC9
// (tasks 10.31/10.32): when git rerere auto-resolves a repeat conflict, the
// pick outcome labels the affected paths as auto-resolved-from-prior, and the
// file remains unmerged so the continue-gate still forces explicit user
// confirmation (staging) before continuing.
func TestService_CherryPick_RerereAutoResolves_RequiresConfirmation(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	feature1, _ := seedConflictingFeature(t, runner, dir)
	runGit(t, runner, dir, "config", "rerere.enabled", "true")

	// rerere must report as enabled, and be suggested only when it is not.
	enabled, err := svc.RerereEnabled(context.Background(), dir)
	if err != nil {
		t.Fatalf("RerereEnabled error: %v", err)
	}
	if !enabled {
		t.Fatalf("expected rerere.enabled=true")
	}
	if git.SuggestEnableRerere(enabled) {
		t.Errorf("should not suggest enabling rerere when it is already enabled")
	}

	commit := []git.DiscoveredCommit{{Commit: git.Commit{SHA: feature1}}}

	// First occurrence: real conflict, resolve + record the resolution.
	if _, err := svc.CherryPick(context.Background(), dir, commit, true); err != nil {
		t.Fatalf("first CherryPick error: %v", err)
	}
	writeFileHelper(t, dir, "a.cls", "l1\nRESOLVED\nl3\n")
	runGit(t, runner, dir, "add", "a.cls")
	if _, err := svc.ContinueCherryPick(context.Background(), dir); err != nil {
		t.Fatalf("first ContinueCherryPick error: %v", err)
	}

	// Undo the pick so the SAME conflict recurs.
	runGit(t, runner, dir, "reset", "--hard", "origin/UAT")

	// Second occurrence: rerere replays the recorded resolution.
	outcome, err := svc.CherryPick(context.Background(), dir, commit, true)
	if err != nil {
		t.Fatalf("second CherryPick error: %v", err)
	}
	if len(outcome.RerereResolved) != 1 || outcome.RerereResolved[0] != "a.cls" {
		t.Fatalf("expected rerere to auto-resolve a.cls, got RerereResolved=%v", outcome.RerereResolved)
	}
	// Confirmation still required: the file is unmerged until the user stages
	// it, so the continue-gate is disabled.
	if len(outcome.State.Unmerged) != 1 {
		t.Fatalf("expected a.cls to remain unmerged pending confirmation, got %+v", outcome.State.Unmerged)
	}
	gate := git.EvaluateContinueGate(outcome.State, nil)
	if gate.Enabled {
		t.Errorf("continue must stay disabled until the user confirms the rerere resolution by staging")
	}
}
