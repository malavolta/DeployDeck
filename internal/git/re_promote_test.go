package git_test

import (
	"context"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// seedRePromoteRange builds two branches on top of newTempRepo's base repo:
// nextTarget (the destination environment for a re-promotion, e.g. "UAT")
// and recTarget (the prior run's own environment branch, e.g. "INT", which
// now holds ONE commit — the prior run's cherry-picked change — under a
// SHA that is genuinely different from wherever that change originated,
// exactly as a real cherry-pick always produces a new commit object). It
// leaves the repo checked out on "main" and returns the cherry-picked
// commit's NEW SHA on recTarget.
func seedRePromoteRange(t *testing.T, runner exec.Runner, dir, nextTarget, recTarget string) (newSHA string) {
	t.Helper()

	runGit(t, runner, dir, "checkout", "-b", nextTarget, "main")
	runGit(t, runner, dir, "push", "origin", nextTarget)

	runGit(t, runner, dir, "checkout", "-b", recTarget, "main")
	newSHA = writeAndCommit(t, runner, dir, "a.cls", "public class A {}\n", "PROJ-1: add A (cherry-picked)")
	runGit(t, runner, dir, "push", "origin", recTarget)

	runGit(t, runner, dir, "checkout", "main")
	return newSHA
}

// TestService_RemapCommitsByPatchID_MapsEquivalentSHAAcrossRange is task 3.1
// (RED): a prior-run commit's content exists in the new
// origin/<nextTarget>..origin/<recTarget> range under a DIFFERENT SHA (the
// real cherry-pick shape) — RemapCommitsByPatchID identifies the new-range
// equivalent and pre-checks it (re-promotion spec: "Equivalent SHA is
// mapped and pre-checked").
func TestService_RemapCommitsByPatchID_MapsEquivalentSHAAcrossRange(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	const nextTarget = "UAT"
	const recTarget = "INT"

	// The ORIGINAL commit as it existed on the prior run's discovery
	// source (e.g. a feature branch) — this is the SHA recorded in
	// rec.Commits. It is deliberately never pushed to recTarget itself:
	// recTarget's real cherry-pick below creates a genuinely different SHA
	// for the same content.
	runGit(t, runner, dir, "checkout", "-b", "feature/PROJ-1", "main")
	oldSHA := writeAndCommit(t, runner, dir, "a.cls", "public class A {}\n", "PROJ-1: add A")
	runGit(t, runner, dir, "checkout", "main")

	newSHA := seedRePromoteRange(t, runner, dir, nextTarget, recTarget)

	ctx := context.Background()
	result, err := svc.RemapCommitsByPatchID(ctx, dir, []string{oldSHA}, nextTarget, recTarget)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Unmatched) != 0 {
		t.Fatalf("expected no unmatched SHAs, got %+v", result.Unmatched)
	}
	if len(result.Matched) != 1 {
		t.Fatalf("expected exactly 1 matched commit, got %d: %+v", len(result.Matched), result.Matched)
	}

	got := result.Matched[0]
	if got.SHA != newSHA {
		t.Fatalf("expected the matched commit's SHA to be the NEW-range SHA %q, got %q (oldSHA=%q must NOT be returned)", newSHA, got.SHA, oldSHA)
	}
	if got.Merge {
		t.Fatalf("expected Merge=false for a single-parent commit, got true")
	}
	if got.Equivalence != git.NotApplied {
		t.Fatalf("expected Equivalence=NotApplied per design, got %v", got.Equivalence)
	}
}

// TestService_RemapCommitsByPatchID_UnmatchedPriorSHAIsWarnedNeverDropped is
// task 3.2 (RED): a prior-run commit with no patch-id equivalent anywhere in
// the new origin/<nextTarget>..origin/<recTarget> range lands in Unmatched
// (exact SHA string), never silently omitted (re-promotion spec: "Commit
// absent from the new origin is warned, not dropped").
func TestService_RemapCommitsByPatchID_UnmatchedPriorSHAIsWarnedNeverDropped(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	const nextTarget = "UAT"
	const recTarget = "INT"

	seedRePromoteRange(t, runner, dir, nextTarget, recTarget)

	// A prior-run commit whose content never made it into recTarget at all
	// (e.g. a diverging/abandoned follow-up commit) — no patch-id in the
	// new range matches it.
	runGit(t, runner, dir, "checkout", "-b", "feature/PROJ-2", "main")
	priorSHA := writeAndCommit(t, runner, dir, "b.cls", "public class B {}\n", "PROJ-2: never promoted")
	runGit(t, runner, dir, "checkout", "main")

	ctx := context.Background()
	result, err := svc.RemapCommitsByPatchID(ctx, dir, []string{priorSHA}, nextTarget, recTarget)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Matched) != 0 {
		t.Fatalf("expected Matched to not grow, got %+v", result.Matched)
	}
	if len(result.Unmatched) != 1 || result.Unmatched[0] != priorSHA {
		t.Fatalf("expected priorSHA %q in Unmatched, got %+v", priorSHA, result.Unmatched)
	}
}

// TestService_RemapCommitsByPatchID_MalformedLeadingDashSHADegradesToUnmatched
// is task 3.3 (RED), the design threat-matrix's planned RED test for
// "Subject/process args (`git show <priorSHA>`)": a crafted leading-`-` SHA
// must never be read as a `git show` option — it degrades to Unmatched with
// no crash and no error return.
func TestService_RemapCommitsByPatchID_MalformedLeadingDashSHADegradesToUnmatched(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	const nextTarget = "UAT"
	const recTarget = "INT"

	seedRePromoteRange(t, runner, dir, nextTarget, recTarget)

	ctx := context.Background()
	result, err := svc.RemapCommitsByPatchID(ctx, dir, []string{"-oops"}, nextTarget, recTarget)
	if err != nil {
		t.Fatalf("expected no hard error for a malformed leading-dash SHA, got: %v", err)
	}
	if len(result.Matched) != 0 {
		t.Fatalf("expected no matches, got %+v", result.Matched)
	}
	if len(result.Unmatched) != 1 || result.Unmatched[0] != "-oops" {
		t.Fatalf("expected the malformed SHA in Unmatched (never parsed as a git show flag), got %+v", result.Unmatched)
	}
}
