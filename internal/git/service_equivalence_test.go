package git_test

import (
	"context"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestService_Equivalence_Integration proves IsAncestor, Cherry and
// PatchID are correctly wired to real git: an ancestor commit, a
// cherry-picked-elsewhere commit (equivalent content, different SHA), and
// two genuinely different commits are each classified correctly using
// real command output (not canned data).
func TestService_Equivalence_Integration(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")

	// A commit applied to UAT directly: sameSHA is a real ancestor of UAT.
	sameSHA := writeAndCommit(t, runner, dir, "same.txt", "same content\n", "PROJ-1: applied directly to UAT")

	runGit(t, runner, dir, "checkout", "-b", "feature/PROJ-1", "main")

	// A commit with the SAME diff as one already on UAT, but a different
	// SHA (simulating a prior cherry-pick under a different message/date).
	equivSHA := writeAndCommit(t, runner, dir, "equiv.txt", "equivalent content\n", "PROJ-1: equivalent, different SHA")
	runGit(t, runner, dir, "checkout", "UAT")
	writeAndCommit(t, runner, dir, "equiv.txt", "equivalent content\n", "PROJ-1: equivalent content already on UAT")
	runGit(t, runner, dir, "checkout", "feature/PROJ-1")

	// A genuinely new, never-applied commit.
	newSHA := writeAndCommit(t, runner, dir, "new.txt", "brand new content\n", "PROJ-1: never applied")

	runGit(t, runner, dir, "push", "origin", "UAT")
	runGit(t, runner, dir, "push", "origin", "feature/PROJ-1")

	ctx := context.Background()

	// sameSHA is only reachable via UAT in this setup, so it must be
	// checked as an ancestor of UAT (not feature/PROJ-1, which never
	// contains it) — proving the "same SHA merged directly" case.
	isAncestor, err := svc.IsAncestor(ctx, dir, sameSHA, "origin/UAT")
	if err != nil {
		t.Fatalf("IsAncestor: unexpected error: %v", err)
	}
	if !isAncestor {
		t.Fatalf("expected sameSHA to be an ancestor of origin/UAT")
	}

	notAncestor, err := svc.IsAncestor(ctx, dir, newSHA, "origin/UAT")
	if err != nil {
		t.Fatalf("IsAncestor: unexpected error: %v", err)
	}
	if notAncestor {
		t.Fatalf("expected newSHA to NOT be an ancestor of origin/UAT")
	}

	markers, err := svc.Cherry(ctx, dir, "origin/UAT", "origin/feature/PROJ-1")
	if err != nil {
		t.Fatalf("Cherry: unexpected error: %v", err)
	}
	if marker := markers[equivSHA]; marker != '-' {
		t.Errorf("expected equivSHA cherry marker '-', got %q (markers=%v)", marker, markers)
	}
	if marker := markers[newSHA]; marker != '+' {
		t.Errorf("expected newSHA cherry marker '+', got %q (markers=%v)", marker, markers)
	}

	equivPatchID, err := svc.PatchID(ctx, dir, equivSHA)
	if err != nil {
		t.Fatalf("PatchID: unexpected error for equivSHA: %v", err)
	}
	if equivPatchID == "" {
		t.Fatalf("expected a non-empty patch-id for equivSHA")
	}
	newPatchID, err := svc.PatchID(ctx, dir, newSHA)
	if err != nil {
		t.Fatalf("PatchID: unexpected error for newSHA: %v", err)
	}
	if newPatchID == equivPatchID {
		t.Fatalf("expected different patch-ids for genuinely different diffs, got the same: %q", newPatchID)
	}
}
