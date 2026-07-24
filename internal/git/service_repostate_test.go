package git_test

import (
	"context"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

// TestService_RepoState_DuringMultiCommitPick_AssembledFromRepo pins tasks
// 10.5/10.6: RepoState is reconciled from CHERRY_PICK_HEAD, .git/sequencer,
// and `git status --porcelain -z` during a REAL multi-commit cherry-pick
// sequence (so .git/sequencer/todo is genuinely populated), never inferred
// from an in-memory model.
func TestService_RepoState_DuringMultiCommitPick_AssembledFromRepo(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	feature1, _ := seedConflictingFeature(t, runner, dir)

	// Drive a real multi-commit cherry-pick that conflicts on the first
	// commit; both commits land in .git/sequencer/todo.
	res := runGitAllow(t, runner, dir, "cherry-pick", feature1+"^..feature")
	if res.ExitCode == 0 {
		t.Fatalf("expected the multi-commit cherry-pick to conflict, but it exited 0")
	}

	state, err := svc.RepoState(context.Background(), dir)
	if err != nil {
		t.Fatalf("RepoState returned error: %v", err)
	}

	if !state.InProgress {
		t.Errorf("expected InProgress=true during a cherry-pick, got false")
	}
	if state.CurrentSHA != feature1 {
		t.Errorf("expected CurrentSHA=%s (CHERRY_PICK_HEAD), got %s", feature1, state.CurrentSHA)
	}
	if state.SequencerRemaining < 1 {
		t.Errorf("expected SequencerRemaining>=1 from .git/sequencer/todo, got %d", state.SequencerRemaining)
	}
	if len(state.Unmerged) != 1 || state.Unmerged[0].Path != "a.cls" {
		t.Fatalf("expected exactly one unmerged conflict on a.cls, got %+v", state.Unmerged)
	}
	if state.Unmerged[0].Kind != git.ConflictText {
		t.Errorf("expected a.cls classified as text conflict, got %s", state.Unmerged[0].Kind)
	}
	if state.Clean {
		t.Errorf("expected Clean=false during a conflict")
	}
}

// TestService_RepoState_ReflectsExternalAbort pins tasks 10.23/10.24: an
// external `git cherry-pick --abort` run outside DeployDeck is detected on
// the next RepoState read (state re-read from the repo every call, never
// cached).
func TestService_RepoState_ReflectsExternalAbort(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	feature1, _ := seedConflictingFeature(t, runner, dir)
	runGitAllow(t, runner, dir, "cherry-pick", feature1+"^..feature")

	// Confirm we really are mid-pick before the external abort.
	mid, err := svc.RepoState(context.Background(), dir)
	if err != nil {
		t.Fatalf("RepoState (mid) error: %v", err)
	}
	if !mid.InProgress {
		t.Fatalf("setup precondition failed: expected a cherry-pick in progress")
	}

	// External action, NOT through the Service.
	runGit(t, runner, dir, "cherry-pick", "--abort")

	after, err := svc.RepoState(context.Background(), dir)
	if err != nil {
		t.Fatalf("RepoState (after abort) error: %v", err)
	}
	if after.InProgress {
		t.Errorf("expected InProgress=false after external abort, got true")
	}
	if after.SequencerRemaining != 0 {
		t.Errorf("expected SequencerRemaining=0 after external abort, got %d", after.SequencerRemaining)
	}
	if len(after.Unmerged) != 0 {
		t.Errorf("expected no unmerged files after external abort, got %+v", after.Unmerged)
	}
	if !after.Clean {
		t.Errorf("expected Clean=true after external abort")
	}
}

// TestService_RepoState_ReflectsExternalContinue pins the other half of
// reconciliation (10.23/10.24): an external resolve + `--continue` is
// likewise reflected on the next read.
func TestService_RepoState_ReflectsExternalContinue(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	feature1, _ := seedConflictingFeature(t, runner, dir)
	runGitAllow(t, runner, dir, "cherry-pick", feature1+"^..feature")

	// Resolve + stage + continue entirely outside the Service.
	writeFileHelper(t, dir, "a.cls", "l1\nRESOLVED\nl3\n")
	runGit(t, runner, dir, "add", "a.cls")
	runGitAllow(t, runner, dir, "cherry-pick", "--continue")

	after, err := svc.RepoState(context.Background(), dir)
	if err != nil {
		t.Fatalf("RepoState (after continue) error: %v", err)
	}
	if after.InProgress {
		t.Errorf("expected the sequence to complete after external continue, got InProgress=true")
	}
	if !after.Clean {
		t.Errorf("expected Clean=true after the sequence completed")
	}
}

// TestService_RepoState_CleanRepo confirms RepoState on a clean repo with no
// cherry-pick in progress.
func TestService_RepoState_CleanRepo(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	state, err := svc.RepoState(context.Background(), dir)
	if err != nil {
		t.Fatalf("RepoState error: %v", err)
	}
	if state.InProgress {
		t.Errorf("expected InProgress=false on a clean repo")
	}
	if !state.Clean {
		t.Errorf("expected Clean=true on a freshly seeded repo")
	}
	if state.SequencerRemaining != 0 {
		t.Errorf("expected SequencerRemaining=0 on a clean repo, got %d", state.SequencerRemaining)
	}
}
