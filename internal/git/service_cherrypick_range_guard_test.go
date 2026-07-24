package git_test

import (
	"context"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

// TestHU006_CherryPick_RangeGuard_DoesNotPromoteInterleavedUnselected
// reproduces the H2 latent hole: the contiguous range form <A>^..<B> includes
// EVERY commit between A and B in the DAG, not just the selected ones. On a
// history base ─ A(selected) ─ X(unselected) ─ B(selected), cherry-picking the
// selection [A,B] must NOT promote the interleaved X — regardless of how
// contiguity was judged. The range optimization may only be used when its
// commit set provably equals the selected set; otherwise it must fall back to
// the explicit ordered SHA list.
func TestHU006_CherryPick_RangeGuard_DoesNotPromoteInterleavedUnselected(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	// UAT touches only its own file so all feature commits apply cleanly.
	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	writeFileHelper(t, dir, "uat-only.txt", "uat\n")
	runGit(t, runner, dir, "add", "uat-only.txt")
	runGit(t, runner, dir, "commit", "-m", "UAT-only change")
	runGit(t, runner, dir, "push", "origin", "UAT")

	// feature history: A(a.cls) ─ X(x.cls, UNSELECTED) ─ B(b.cls). Distinct
	// files, so applying only A and B (skipping X) is a clean sequence.
	runGit(t, runner, dir, "checkout", "-b", "feature", "main")
	a := writeAndCommit(t, runner, dir, "a.cls", "A\n", "PROJ-1: add A (selected)")
	_ = writeAndCommit(t, runner, dir, "x.cls", "X\n", "PROJ-9: add X (unselected, interleaved)")
	b := writeAndCommit(t, runner, dir, "b.cls", "B\n", "PROJ-1: add B (selected)")
	runGit(t, runner, dir, "push", "origin", "feature")

	runGit(t, runner, dir, "checkout", "UAT")

	// Selection is [A, B]; X is deliberately NOT selected. Pass contiguous=true
	// to exercise the range path (the mis-wire this guard defends against).
	commits := []git.DiscoveredCommit{
		{Commit: git.Commit{SHA: a}},
		{Commit: git.Commit{SHA: b}},
	}
	outcome, err := svc.CherryPick(context.Background(), dir, commits, true)
	if err != nil {
		t.Fatalf("CherryPick error: %v", err)
	}
	if outcome.State.InProgress {
		t.Fatalf("expected the selected picks to complete cleanly, got %+v", outcome.State)
	}

	// A and B were promoted.
	if runGitAllow(t, runner, dir, "cat-file", "-e", "HEAD:a.cls").ExitCode != 0 {
		t.Errorf("expected selected A (a.cls) to be promoted")
	}
	if runGitAllow(t, runner, dir, "cat-file", "-e", "HEAD:b.cls").ExitCode != 0 {
		t.Errorf("expected selected B (b.cls) to be promoted")
	}
	// The interleaved UNSELECTED X must NOT have been promoted.
	if runGitAllow(t, runner, dir, "cat-file", "-e", "HEAD:x.cls").ExitCode == 0 {
		t.Fatalf("DATA-INTEGRITY GUARD: interleaved unselected commit X (x.cls) was promoted by the range form <A>^..<B>")
	}
}

// TestHU006_CherryPick_RangeGuard_KeepsRangeFormWhenProvablySafe pins that the
// guard does not over-fire: a genuinely contiguous selection (no unselected
// commit between first and last) still promotes exactly its commits.
func TestHU006_CherryPick_RangeGuard_KeepsRangeFormWhenProvablySafe(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	writeFileHelper(t, dir, "uat-only.txt", "uat\n")
	runGit(t, runner, dir, "add", "uat-only.txt")
	runGit(t, runner, dir, "commit", "-m", "UAT-only change")
	runGit(t, runner, dir, "push", "origin", "UAT")

	runGit(t, runner, dir, "checkout", "-b", "feature", "main")
	a := writeAndCommit(t, runner, dir, "a.cls", "A\n", "PROJ-1: add A")
	b := writeAndCommit(t, runner, dir, "b.cls", "B\n", "PROJ-1: add B")
	runGit(t, runner, dir, "push", "origin", "feature")

	runGit(t, runner, dir, "checkout", "UAT")

	commits := []git.DiscoveredCommit{
		{Commit: git.Commit{SHA: a}},
		{Commit: git.Commit{SHA: b}},
	}
	outcome, err := svc.CherryPick(context.Background(), dir, commits, true)
	if err != nil {
		t.Fatalf("CherryPick error: %v", err)
	}
	if outcome.State.InProgress {
		t.Fatalf("expected the contiguous selection to complete cleanly, got %+v", outcome.State)
	}
	if runGitAllow(t, runner, dir, "cat-file", "-e", "HEAD:a.cls").ExitCode != 0 ||
		runGitAllow(t, runner, dir, "cat-file", "-e", "HEAD:b.cls").ExitCode != 0 {
		t.Errorf("expected both genuinely-contiguous commits promoted")
	}
}
