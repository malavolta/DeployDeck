package git_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"deploydeck/internal/config"
	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

// TestHU005_PromotionBranch_E2E is the consolidated end-to-end scenario
// from docs/HISTORIAS.md HU-005's "Test E2E" section, run through the real
// HU-005 entry points (RenderBranchName, Service.CreatePromotionBranch,
// RegisterPromotionBranch, IsProtectedBranch) against the documented
// two-repo bare-remote harness ("repo git temporal con remoto bare local,
// sin org"): origin/UAT starts with an initial HEAD; after the local
// clone, the remote advances UAT to a new HEAD that only a fetch
// retrieves; the created promotion branch must start exactly from the
// post-fetch origin/UAT HEAD, `git fetch origin` must run before creation,
// and the DeploymentPlan must record the final branch name. Variants
// (subtests): an existing temp branch locally and remotely (prompts for
// action), a user on a protected branch at flow start (not modified
// directly), and a fetch failure (no branch change).
func TestHU005_PromotionBranch_E2E(t *testing.T) {
	localDir, _, seedDir := newTempRepoWithRemote(t)
	runner := exec.NewOSRunner()
	ctx := context.Background()

	cfg := config.Config{
		Branches:     map[string]string{"integration": "main", "uat": "UAT"},
		BranchFormat: config.DefaultBranchFormat,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("cfg.Validate: unexpected error: %v", err)
	}

	ticket := "PROJ-1"
	target := "UAT"
	branchName := git.RenderBranchName(cfg.BranchFormat, ticket, target)
	if branchName != "deploy/PROJ-1-to-UAT" {
		t.Fatalf("expected the rendered branch name to follow the configured branchFormat, got %q", branchName)
	}

	// --- Seed: origin/UAT already has an initial HEAD (from
	// newTempRepoWithRemote); after the local clone, the remote advances
	// UAT to a new HEAD directly via seedDir — localDir is never touched —
	// that only a fetch retrieves. ---
	staleUATHead := trimNewline(string(runGit(t, runner, localDir, "rev-parse", "origin/UAT").Stdout))

	runGit(t, runner, seedDir, "checkout", "UAT")
	advancedUATHead := writeAndCommit(t, runner, seedDir, "uat-advance.txt", "advanced\n", "chore: advance UAT after local clone")
	runGit(t, runner, seedDir, "push", "origin", "UAT")
	runGit(t, runner, seedDir, "checkout", "main")

	if advancedUATHead == staleUATHead {
		t.Fatalf("setup bug: advancing UAT produced the same SHA as the stale clone-time HEAD")
	}

	// --- A promotion branch with the SAME name preexists in origin,
	// belonging to an UNRELATED run, so its own separate creation attempt
	// below (in the collision variant) hits a REMOTE-only collision. ---

	recorder := &callRecordingRunner{inner: exec.NewOSRunner()}
	svc := git.New(recorder)

	plan := git.DeploymentPlan{Ticket: ticket, TargetBranch: target}

	if err := svc.CreatePromotionBranch(ctx, localDir, target, branchName); err != nil {
		t.Fatalf("CreatePromotionBranch: unexpected error: %v", err)
	}
	plan = git.RegisterPromotionBranch(plan, branchName)

	// === Assert: git fetch origin runs before the branch is created. ===
	fetchIdx, checkoutIdx := -1, -1
	for i, call := range recorder.calls {
		if strings.Contains(call, "fetch origin") && fetchIdx == -1 {
			fetchIdx = i
		}
		if strings.Contains(call, "checkout -b "+branchName) && checkoutIdx == -1 {
			checkoutIdx = i
		}
	}
	if fetchIdx == -1 || checkoutIdx == -1 || fetchIdx >= checkoutIdx {
		t.Fatalf("expected fetch to run before checkout -b, got calls: %v", recorder.calls)
	}

	// === Assert: the branch starts EXACTLY from the post-fetch
	// origin/UAT HEAD, not the outdated local ref. ===
	gotHead := trimNewline(string(runGit(t, runner, localDir, "rev-parse", branchName).Stdout))
	if gotHead != advancedUATHead {
		t.Fatalf("expected %s to start from the advanced origin/UAT HEAD %q, got %q (stale ref was %q)", branchName, advancedUATHead, gotHead, staleUATHead)
	}

	// === Assert: the DeploymentPlan records the final branch name. ===
	if plan.PromotionBranch != branchName {
		t.Fatalf("expected the DeploymentPlan to record %q, got %q", branchName, plan.PromotionBranch)
	}

	// === Variant: a temp branch with the same name already exists,
	// locally and (separately) only on origin — both prompt for an
	// action instead of silently overwriting. ===
	t.Run("existing temp branch locally and remotely prompts for an action", func(t *testing.T) {
		localDir2, _, seedDir2 := newTempRepoWithRemote(t)
		runner2 := exec.NewOSRunner()
		svc2 := git.New(runner2)

		localCollision := "deploy/PROJ-2-to-UAT"
		runGit(t, runner2, localDir2, "checkout", "-b", localCollision, "main")
		runGit(t, runner2, localDir2, "checkout", "main")
		if err := svc2.CreatePromotionBranch(ctx, localDir2, "UAT", localCollision); !errors.Is(err, git.ErrPromotionBranchExists) {
			t.Fatalf("expected ErrPromotionBranchExists for a local collision, got: %v", err)
		}

		remoteCollision := "deploy/PROJ-3-to-UAT"
		runGit(t, runner2, seedDir2, "checkout", "-b", remoteCollision, "main")
		runGit(t, runner2, seedDir2, "push", "origin", remoteCollision)
		runGit(t, runner2, seedDir2, "checkout", "main")
		if err := svc2.CreatePromotionBranch(ctx, localDir2, "UAT", remoteCollision); !errors.Is(err, git.ErrPromotionBranchExists) {
			t.Fatalf("expected ErrPromotionBranchExists for a remote-only collision, got: %v", err)
		}
	})

	// === Variant: the user is on a protected branch when the flow
	// starts — it is not modified directly. ===
	t.Run("user on protected branch is not modified directly", func(t *testing.T) {
		localDir3, _, _ := newTempRepoWithRemote(t)
		runner3 := exec.NewOSRunner()
		svc3 := git.New(runner3)

		runGit(t, runner3, localDir3, "checkout", "main")
		if !git.IsProtectedBranch(cfg, "main") {
			t.Fatalf("expected main to be protected per config.Branches")
		}
		beforeHead := trimNewline(string(runGit(t, runner3, localDir3, "rev-parse", "main").Stdout))

		if err := svc3.CreatePromotionBranch(ctx, localDir3, "UAT", "deploy/PROJ-4-to-UAT"); err != nil {
			t.Fatalf("CreatePromotionBranch: unexpected error: %v", err)
		}

		afterHead := trimNewline(string(runGit(t, runner3, localDir3, "rev-parse", "main").Stdout))
		if afterHead != beforeHead {
			t.Fatalf("expected the protected branch main to remain unchanged: before=%q after=%q", beforeHead, afterHead)
		}
	})

	// === Variant: git fetch fails — the flow stops without changing the
	// current branch. ===
	t.Run("fetch failure stops without changing the current branch", func(t *testing.T) {
		localDir4, _, _ := newTempRepoWithRemote(t)
		runner4 := exec.NewOSRunner()
		svc4 := git.New(runner4)

		runGit(t, runner4, localDir4, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "does-not-exist"))
		beforeBranch := trimNewline(string(runGit(t, runner4, localDir4, "branch", "--show-current").Stdout))

		if err := svc4.CreatePromotionBranch(ctx, localDir4, "UAT", "deploy/PROJ-5-to-UAT"); err == nil {
			t.Fatalf("expected an error when git fetch origin fails")
		}

		afterBranch := trimNewline(string(runGit(t, runner4, localDir4, "branch", "--show-current").Stdout))
		if afterBranch != beforeBranch {
			t.Fatalf("expected the current branch to remain unchanged after a fetch failure: before=%q after=%q", beforeBranch, afterBranch)
		}
	})
}
