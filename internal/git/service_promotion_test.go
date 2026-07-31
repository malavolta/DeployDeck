package git_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestService_CreatePromotionBranch_FollowsAdvancedRemoteNotStaleLocalRef
// is the critical correctness property this HU exists for
// (Branch-Base-From-Fetched-Remote-Not-Stale-Local-Ref requirement:
// specs/promotion-branch/spec.md, docs/HISTORIAS.md HU-005 Test E2E): after
// localDir has already cloned origin/UAT at its initial HEAD, origin/UAT
// advances to a NEW HEAD directly on the bare remote (via seedDir, never
// touching localDir) — only a fetch retrieves it. The created promotion
// branch must start EXACTLY from the post-fetch origin/UAT HEAD, not the
// outdated ref localDir cloned with.
func TestService_CreatePromotionBranch_FollowsAdvancedRemoteNotStaleLocalRef(t *testing.T) {
	localDir, _, seedDir := newTempRepoWithRemote(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	// Sanity: localDir's clone-time origin/UAT is the initial seeded HEAD,
	// proving the advance below is a REAL change, not a no-op.
	staleUATHead := trimNewline(string(runGit(t, runner, localDir, "rev-parse", "origin/UAT").Stdout))

	// Advance origin/UAT directly on the remote, through seedDir — localDir
	// is never touched by this and does not (yet) know about it.
	runGit(t, runner, seedDir, "checkout", "UAT")
	advancedUATHead := writeAndCommit(t, runner, seedDir, "uat-advance.txt", "advanced\n", "chore: advance UAT after local clone")
	runGit(t, runner, seedDir, "push", "origin", "UAT")

	if advancedUATHead == staleUATHead {
		t.Fatalf("setup bug: advancing UAT produced the same SHA as the stale clone-time HEAD")
	}

	if err := svc.CreatePromotionBranch(context.Background(), localDir, "UAT", "deploy/PROJ-1-to-UAT"); err != nil {
		t.Fatalf("CreatePromotionBranch: unexpected error: %v", err)
	}

	gotHead := trimNewline(string(runGit(t, runner, localDir, "rev-parse", "deploy/PROJ-1-to-UAT").Stdout))
	if gotHead != advancedUATHead {
		t.Fatalf("expected the promotion branch to start from the advanced post-fetch origin/UAT HEAD %q, got %q (stale clone-time HEAD was %q)", advancedUATHead, gotHead, staleUATHead)
	}
}

// TestService_CreatePromotionBranch_ExistingBranchCollision_TableDriven
// proves a promotion branch name that already exists — locally, or ONLY as
// an origin remote-tracking branch discovered by the fetch this method
// always runs first — stops creation with ErrPromotionBranchExists instead
// of silently overwriting or reusing it (HU-005 Existing-Temp-Branch-
// Collision-Handling requirement: "se pide accion al usuario"), so the
// caller (internal/app, Phase 11) can prompt for an action. A name that
// collides nowhere creates normally.
func TestService_CreatePromotionBranch_ExistingBranchCollision_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		branchName  string
		seedLocally bool // create branchName in localDir itself, before creation is attempted
		seedRemote  bool // create branchName on origin (via seedDir), before creation is attempted
	}{
		{name: "collides locally", branchName: "deploy/PROJ-1-to-UAT-local", seedLocally: true},
		{name: "collides only on origin (discovered by fetch)", branchName: "deploy/PROJ-1-to-UAT-remote", seedRemote: true},
		{name: "no collision creates normally", branchName: "deploy/PROJ-1-to-UAT-clean"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localDir, _, seedDir := newTempRepoWithRemote(t)
			runner := exec.NewOSRunner()
			svc := git.New(runner)

			if tt.seedLocally {
				runGit(t, runner, localDir, "checkout", "-b", tt.branchName, "main")
				runGit(t, runner, localDir, "checkout", "main")
			}
			if tt.seedRemote {
				runGit(t, runner, seedDir, "checkout", "-b", tt.branchName, "main")
				runGit(t, runner, seedDir, "push", "origin", tt.branchName)
				runGit(t, runner, seedDir, "checkout", "main")
			}

			err := svc.CreatePromotionBranch(context.Background(), localDir, "UAT", tt.branchName)

			if tt.seedLocally || tt.seedRemote {
				if !errors.Is(err, git.ErrPromotionBranchExists) {
					t.Fatalf("expected ErrPromotionBranchExists for colliding branch %q, got: %v", tt.branchName, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("CreatePromotionBranch(%q): unexpected error: %v", tt.branchName, err)
			}
			result, _ := runner.Run(context.Background(), exec.CommandRequest{
				Name: "git", Args: []string{"rev-parse", "--verify", "--quiet", tt.branchName},
				Dir: localDir, Env: nonInteractiveGitEnv,
			})
			if result.ExitCode != 0 {
				t.Fatalf("expected branch %q to have been created", tt.branchName)
			}
		})
	}
}

// TestService_CreatePromotionBranch_ProtectedBranchNotModifiedDirectly
// proves that starting the promotion-branch flow while the user is
// currently ON a protected branch (config.Config.Branches — here "main")
// never modifies that branch directly: CreatePromotionBranch always bases
// the new branch on the explicit origin/<target> ref, never on the current
// HEAD, so the protected branch's own ref is untouched and HEAD ends up on
// the NEW promotion branch, not stacked on top of the protected one (HU-005
// Protected-Branch-Guard requirement: specs/promotion-branch/spec.md).
func TestService_CreatePromotionBranch_ProtectedBranchNotModifiedDirectly(t *testing.T) {
	localDir, _, _ := newTempRepoWithRemote(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	cfg := config.Config{Branches: map[string]string{"integration": "main", "uat": "UAT"}}

	runGit(t, runner, localDir, "checkout", "main")
	if !git.IsProtectedBranch(cfg, "main") {
		t.Fatalf("expected main to be a protected branch per config.Branches")
	}
	beforeHead := trimNewline(string(runGit(t, runner, localDir, "rev-parse", "main").Stdout))

	if err := svc.CreatePromotionBranch(context.Background(), localDir, "UAT", "deploy/PROJ-1-to-UAT"); err != nil {
		t.Fatalf("CreatePromotionBranch: unexpected error: %v", err)
	}

	afterHead := trimNewline(string(runGit(t, runner, localDir, "rev-parse", "main").Stdout))
	if afterHead != beforeHead {
		t.Fatalf("expected protected branch main to remain unchanged: before=%q after=%q", beforeHead, afterHead)
	}

	currentBranch := trimNewline(string(runGit(t, runner, localDir, "branch", "--show-current").Stdout))
	if currentBranch != "deploy/PROJ-1-to-UAT" {
		t.Fatalf("expected HEAD to move to the new promotion branch, not stay stacked on the protected one, got %q", currentBranch)
	}
}

// TestService_CreatePromotionBranch_FetchFailure_StopsWithoutBranchChange
// proves a real `git fetch origin` failure (origin pointed at a
// nonexistent path) stops the flow entirely: an error is returned, the
// current branch is unchanged, and the promotion branch is never created
// (HU-005 Fetch-Failure-Halts-Flow-Without-Branch-Change requirement:
// specs/promotion-branch/spec.md).
func TestService_CreatePromotionBranch_FetchFailure_StopsWithoutBranchChange(t *testing.T) {
	localDir, _, _ := newTempRepoWithRemote(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	runGit(t, runner, localDir, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "does-not-exist"))

	beforeBranch := trimNewline(string(runGit(t, runner, localDir, "branch", "--show-current").Stdout))

	err := svc.CreatePromotionBranch(context.Background(), localDir, "UAT", "deploy/PROJ-1-to-UAT")
	if err == nil {
		t.Fatalf("expected an error when git fetch origin fails")
	}

	afterBranch := trimNewline(string(runGit(t, runner, localDir, "branch", "--show-current").Stdout))
	if afterBranch != beforeBranch {
		t.Fatalf("expected the current branch to remain unchanged after a fetch failure: before=%q after=%q", beforeBranch, afterBranch)
	}

	result, runErr := runner.Run(context.Background(), exec.CommandRequest{
		Name: "git", Args: []string{"rev-parse", "--verify", "--quiet", "deploy/PROJ-1-to-UAT"},
		Dir: localDir, Env: nonInteractiveGitEnv,
	})
	if runErr != nil {
		t.Fatalf("failed to check for the promotion branch: %v", runErr)
	}
	if result.ExitCode == 0 {
		t.Fatalf("expected the promotion branch to NOT have been created after a fetch failure")
	}
}

// TestService_CreatePromotionBranch_FetchRunsBeforeCheckout proves
// `git fetch origin` runs before the promotion branch is created (HU-005
// Fetch-Before-Branch-Creation-Ordering requirement:
// specs/promotion-branch/spec.md). It exercises real git via a
// callRecordingRunner wrapping exec.NewOSRunner(), so the ordering
// assertion is against actual git behavior, not a canned response.
func TestService_CreatePromotionBranch_FetchRunsBeforeCheckout(t *testing.T) {
	localDir, _, _ := newTempRepoWithRemote(t)

	recorder := &callRecordingRunner{inner: exec.NewOSRunner()}
	svc := git.New(recorder)

	if err := svc.CreatePromotionBranch(context.Background(), localDir, "UAT", "deploy/PROJ-1-to-UAT"); err != nil {
		t.Fatalf("CreatePromotionBranch: unexpected error: %v", err)
	}

	fetchIdx, checkoutIdx := -1, -1
	for i, call := range recorder.calls {
		if strings.Contains(call, "fetch origin") && fetchIdx == -1 {
			fetchIdx = i
		}
		if strings.Contains(call, "checkout -b deploy/PROJ-1-to-UAT") && checkoutIdx == -1 {
			checkoutIdx = i
		}
	}

	if fetchIdx == -1 {
		t.Fatalf("expected a git fetch call, got calls: %v", recorder.calls)
	}
	if checkoutIdx == -1 {
		t.Fatalf("expected a git checkout -b call, got calls: %v", recorder.calls)
	}
	if fetchIdx >= checkoutIdx {
		t.Fatalf("expected fetch (call %d) to run before checkout -b (call %d): %v", fetchIdx, checkoutIdx, recorder.calls)
	}
}
