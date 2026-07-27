package git_test

import (
	"context"
	"strings"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

// TestService_Push_FirstPushSetsUpstreamAndRoundTrips is task 1.1 (RED):
// Push runs `git push -u origin <branch>` against a real bare local remote,
// round-tripping the pushed branch to origin and setting its upstream
// tracking ref on the FIRST push (HU-014 AC3, design.md's "Push state"
// threat-matrix row).
func TestService_Push_FirstPushSetsUpstreamAndRoundTrips(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)
	ctx := context.Background()

	branch := "deploy/PROJ-1-to-UAT"
	runGit(t, runner, dir, "checkout", "-b", branch)
	writeAndCommit(t, runner, dir, "feature.txt", "feature\n", "chore: add feature")

	if err := svc.Push(ctx, dir, branch); err != nil {
		t.Fatalf("Push: unexpected error: %v", err)
	}

	// The branch round-tripped to origin: origin/<branch> resolves to the
	// same SHA as the local branch.
	localHead := trimNewline(string(runGit(t, runner, dir, "rev-parse", branch).Stdout))
	runGit(t, runner, dir, "fetch", "origin")
	remoteHead := trimNewline(string(runGit(t, runner, dir, "rev-parse", "origin/"+branch).Stdout))
	if localHead != remoteHead {
		t.Fatalf("expected origin/%s to match local HEAD %q, got %q", branch, localHead, remoteHead)
	}

	// -u set upstream tracking on the FIRST push: `git status -sb` reports
	// the branch tracking origin/<branch>, not "no upstream".
	statusOut := string(runGit(t, runner, dir, "status", "--short", "--branch").Stdout)
	if !strings.Contains(statusOut, "origin/"+branch) {
		t.Fatalf("expected upstream tracking of origin/%s to be set after the first push, status: %q", branch, statusOut)
	}
}

// TestService_Push_UnknownBranchErrors is task 1.1 (RED)'s failure-path
// companion: pushing a branch that does not exist locally must return an
// error, not silently succeed.
func TestService_Push_UnknownBranchErrors(t *testing.T) {
	dir := newTempRepo(t)
	svc := git.New(exec.NewOSRunner())

	if err := svc.Push(context.Background(), dir, "does-not-exist"); err == nil {
		t.Fatal("expected Push to error for a branch that does not exist locally")
	}
}
