package git_test

import (
	"context"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

// TestService_VerifyPromotedContent_DetectsSpuriousExtraFile closes the H3
// false-pass: an extra file changed on the promotion branch that the selected
// commit set never touched (e.g. a wrong pick dragging it in) must be flagged
// as a spurious promotion — even though a touched-files-only diff vs the
// selected tip would never inspect it.
func TestService_VerifyPromotedContent_DetectsSpuriousExtraFile(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	// UAT is the pre-pick base.
	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	writeFileHelper(t, dir, "uat-only.txt", "uat\n")
	runGit(t, runner, dir, "add", "uat-only.txt")
	runGit(t, runner, dir, "commit", "-m", "UAT-only change")
	runGit(t, runner, dir, "push", "origin", "UAT")

	// Promotion branch off origin/UAT: only a.cls was SUPPOSED to be promoted,
	// but an extra x.cls also landed on the branch.
	runGit(t, runner, dir, "checkout", "-b", "deploy/PROJ-1-to-UAT", "origin/UAT")
	selectedTip := writeAndCommit(t, runner, dir, "a.cls", "public class A {}\n", "PROJ-1: add A (selected)")
	writeAndCommit(t, runner, dir, "x.cls", "public class X {}\n", "PROJ-9: add X (spurious)")

	verify, err := svc.VerifyPromotedContent(context.Background(), dir, "origin/UAT", selectedTip, []string{"a.cls"})
	if err != nil {
		t.Fatalf("VerifyPromotedContent error: %v", err)
	}
	if verify.OK() {
		t.Fatalf("expected a spurious-promotion result, got OK")
	}
	if len(verify.SpuriousFiles) != 1 || verify.SpuriousFiles[0] != "x.cls" {
		t.Fatalf("expected x.cls flagged spurious, got %v", verify.SpuriousFiles)
	}
	// a.cls was faithfully promoted (matches the selected tip) — not partial.
	if len(verify.PartialFiles) != 0 {
		t.Errorf("expected no partial files, got %v", verify.PartialFiles)
	}
	if len(verify.Warnings()) == 0 {
		t.Errorf("expected a spurious-promotion warning, got none")
	}
}

// TestService_VerifyPromotedContent_IntentionalSubset_NoFalsePartial closes the
// H3 false-fail: promoting only an EARLIER selected commit (A) whose file is
// further modified by a LATER, deliberately-excluded commit (C) must NOT raise
// a partial-promotion warning. The promoted content faithfully equals the
// selected tip A, even though it differs from the source branch tip (which
// carries C's change).
func TestService_VerifyPromotedContent_IntentionalSubset_NoFalsePartial(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	// main seeds a.cls=v1; UAT is unrelated; feature edits a.cls twice.
	writeFileHelper(t, dir, "a.cls", "v1\n")
	runGit(t, runner, dir, "add", "a.cls")
	runGit(t, runner, dir, "commit", "-m", "seed a.cls")
	runGit(t, runner, dir, "push", "origin", "main")

	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	writeFileHelper(t, dir, "uat-only.txt", "uat\n")
	runGit(t, runner, dir, "add", "uat-only.txt")
	runGit(t, runner, dir, "commit", "-m", "UAT-only change")
	runGit(t, runner, dir, "push", "origin", "UAT")

	runGit(t, runner, dir, "checkout", "-b", "feature", "main")
	a := writeAndCommit(t, runner, dir, "a.cls", "v2\n", "PROJ-1: a.cls -> v2 (selected)")
	_ = writeAndCommit(t, runner, dir, "a.cls", "v3\n", "PROJ-1: a.cls -> v3 (excluded)")
	runGit(t, runner, dir, "push", "origin", "feature")

	runGit(t, runner, dir, "checkout", "UAT")

	// Promote ONLY A (the intentional subset). A applies cleanly onto UAT.
	outcome, err := svc.CherryPick(context.Background(), dir,
		[]git.DiscoveredCommit{{Commit: git.Commit{SHA: a}}}, true)
	if err != nil {
		t.Fatalf("CherryPick error: %v", err)
	}
	if outcome.State.InProgress {
		t.Fatalf("expected the subset promotion to complete cleanly, got %+v", outcome.State)
	}

	// Compared against the SELECTED tip A, a.cls is faithful (v2==v2), even
	// though it differs from the feature tip (v3), the excluded commit.
	verify, err := svc.VerifyPromotedContent(context.Background(), dir, "origin/UAT", a, []string{"a.cls"})
	if err != nil {
		t.Fatalf("VerifyPromotedContent error: %v", err)
	}
	if !verify.OK() {
		t.Fatalf("intentional subset promotion must not raise a partial/spurious warning; got partial=%v spurious=%v",
			verify.PartialFiles, verify.SpuriousFiles)
	}
}
