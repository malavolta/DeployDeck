package git_test

import (
	"context"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestService_FastForwardBranch_TableDriven is task 1.3 (RED):
// FastForwardBranch classifies the relationship between a LOCAL branch and
// its origin/<branch> counterpart into FFResult DATA, never a force-push
// (incremental-promotion spec: "Diverged Remote Deploy Branch Blocks With A
// Clear Error"). The diverged case is the highest-value safety property:
// it asserts the branch's own tip (both local AND remote) is COMPLETELY
// UNCHANGED by the call — nothing was force-pushed or rewritten.
func TestService_FastForwardBranch_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		seed    func(t *testing.T, runner exec.Runner, localDir, seedDir, branch string)
		want    git.FFResult
		wantErr bool
	}{
		{
			name: "remote strictly ahead fast-forwards",
			seed: func(t *testing.T, runner exec.Runner, localDir, seedDir, branch string) {
				// localDir already has `branch` at the seeded tip; advance
				// origin/<branch> further via seedDir (never touching localDir).
				runGit(t, runner, seedDir, "checkout", branch)
				writeAndCommit(t, runner, seedDir, "advance.txt", "advance\n", "chore: advance branch")
				runGit(t, runner, seedDir, "push", "origin", branch)
			},
			want: git.FFFastForwarded,
		},
		{
			name: "local strictly ahead is a no-op",
			seed: func(t *testing.T, runner exec.Runner, localDir, seedDir, branch string) {
				runGit(t, runner, localDir, "checkout", branch)
				writeAndCommit(t, runner, localDir, "local-advance.txt", "local\n", "chore: local-only advance")
			},
			want: git.FFLocalAhead,
		},
		{
			name: "up to date is a no-op",
			seed: func(t *testing.T, runner exec.Runner, localDir, seedDir, branch string) {
				// No changes on either side.
			},
			want: git.FFUpToDate,
		},
		{
			name: "diverged blocks with data, never a force-push",
			seed: func(t *testing.T, runner exec.Runner, localDir, seedDir, branch string) {
				runGit(t, runner, seedDir, "checkout", branch)
				writeAndCommit(t, runner, seedDir, "remote-only.txt", "remote\n", "chore: remote-only change")
				runGit(t, runner, seedDir, "push", "origin", branch)

				runGit(t, runner, localDir, "checkout", branch)
				writeAndCommit(t, runner, localDir, "local-only.txt", "local\n", "chore: local-only change")
			},
			want: git.FFDiverged,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localDir, _, seedDir := newTempRepoWithRemote(t)
			runner := exec.NewOSRunner()
			svc := git.New(runner)
			branch := "deploy/PROJ-1-to-UAT"

			// Seed the reused branch identically on both sides (local checkout +
			// push), so every scenario starts from a genuinely shared ancestor.
			runGit(t, runner, seedDir, "checkout", "-b", branch, "UAT")
			runGit(t, runner, seedDir, "push", "origin", branch)
			runGit(t, runner, seedDir, "checkout", "UAT")
			runGit(t, runner, localDir, "fetch", "origin")
			runGit(t, runner, localDir, "checkout", "-b", branch, "origin/"+branch)

			beforeLocal := trimNewline(string(runGit(t, runner, localDir, "rev-parse", branch).Stdout))
			beforeRemoteRes, _ := runner.Run(context.Background(), exec.CommandRequest{
				Name: "git", Args: []string{"rev-parse", "--verify", "--quiet", "origin/" + branch},
				Dir: localDir, Env: nonInteractiveGitEnv,
			})
			beforeRemote := trimNewline(string(beforeRemoteRes.Stdout))

			tt.seed(t, runner, localDir, seedDir, branch)

			// Re-fetch so origin/<branch> reflects any remote-side advance the
			// seed step pushed — FastForwardBranch itself never fetches (the
			// design's collision flow already fetched at branch-creation time).
			runGit(t, runner, localDir, "fetch", "origin")

			got, err := svc.FastForwardBranch(context.Background(), localDir, branch)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got FFResult %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("FastForwardBranch: unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("FastForwardBranch() = %v, want %v", got, tt.want)
			}

			if tt.want == git.FFDiverged {
				afterLocal := trimNewline(string(runGit(t, runner, localDir, "rev-parse", branch).Stdout))
				afterRemoteRes := runGit(t, runner, localDir, "rev-parse", "origin/"+branch)
				afterRemote := trimNewline(string(afterRemoteRes.Stdout))
				if afterLocal == beforeLocal {
					t.Fatalf("setup bug: local tip did not actually change during divergence seeding")
				}
				if afterRemote == beforeRemote {
					t.Fatalf("setup bug: remote tip did not actually change during divergence seeding")
				}
				// The load-bearing safety assertion: FastForwardBranch must not
				// have touched either tip — no force-push, no rewrite.
				stillLocal := trimNewline(string(runGit(t, runner, localDir, "rev-parse", branch).Stdout))
				stillRemote := trimNewline(string(runGit(t, runner, localDir, "rev-parse", "origin/"+branch).Stdout))
				if stillLocal != afterLocal {
					t.Fatalf("FastForwardBranch must never rewrite the local diverged branch: before-call=%q after-call=%q", afterLocal, stillLocal)
				}
				if stillRemote != afterRemote {
					t.Fatalf("FastForwardBranch must never force-push the diverged remote branch: before-call=%q after-call=%q", afterRemote, stillRemote)
				}
			}
		})
	}
}

// TestService_FastForwardBranch_RemoteAbsent is task 1.3 (RED): a local-only
// branch with no origin/<branch> counterpart at all (never pushed) is
// reported as FFRemoteAbsent — a separate test from the table above since
// this scenario must never push the branch to origin in the first place
// (a shared "push then fetch" setup would recreate the very ref this case
// tests the absence of).
func TestService_FastForwardBranch_RemoteAbsent(t *testing.T) {
	localDir, _, _ := newTempRepoWithRemote(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)
	branch := "deploy/PROJ-1-to-UAT"

	// Created locally, never pushed: origin/<branch> never existed.
	runGit(t, runner, localDir, "checkout", "-b", branch, "origin/UAT")

	got, err := svc.FastForwardBranch(context.Background(), localDir, branch)
	if err != nil {
		t.Fatalf("FastForwardBranch: unexpected error: %v", err)
	}
	if got != git.FFRemoteAbsent {
		t.Fatalf("FastForwardBranch() = %v, want FFRemoteAbsent", got)
	}
}

// TestService_FilterNotOnBranch_LayeredClassification is task 1.5 (RED):
// FilterNotOnBranch classifies each candidate commit via the layered
// ancestor -> -x trailer (primary) -> cherry-equivalence (fallback) order
// (incremental-promotion spec: "Layered Already-On-Branch Detection"),
// excluding anything already present and keeping only the genuinely new
// commits. Here the ancestor commit is caught by ancestry and the
// differently-SHA'd cherry-picked commit by its `-x` trailer, leaving only
// the not-yet-applied commit.
func TestService_FilterNotOnBranch_LayeredClassification(t *testing.T) {
	localDir, _, seedDir := newTempRepoWithRemote(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	// deploy/PROJ-1-to-UAT already carries a commit whose ORIGINAL SHA (on
	// feature/PROJ-1) differs from its cherry-picked SHA on the deploy
	// branch — the trailer-detection case — plus another commit that is a
	// direct ancestor (merged as-is) — the ancestor-detection case.
	runGit(t, runner, seedDir, "checkout", "-b", "feature/PROJ-1", "UAT")
	ancestorSHA := writeAndCommit(t, runner, seedDir, "ancestor.txt", "ancestor\n", "PROJ-1 ancestor commit")
	trailerSourceSHA := writeAndCommit(t, runner, seedDir, "trailer.txt", "trailer\n", "PROJ-1 trailer-detected commit")
	newSHA := writeAndCommit(t, runner, seedDir, "new.txt", "new\n", "PROJ-1 not yet applied commit")
	runGit(t, runner, seedDir, "push", "origin", "feature/PROJ-1")

	runGit(t, runner, seedDir, "checkout", "-b", "deploy/PROJ-1-to-UAT", "UAT")
	// Ancestor case: merge the ancestor commit directly (fast-forward it in).
	runGit(t, runner, seedDir, "merge", "--ff-only", ancestorSHA)
	// Trailer case: cherry-pick with -x so the deploy branch's own commit
	// carries a DIFFERENT SHA but the "(cherry picked from commit
	// <trailerSourceSHA>)" trailer.
	runGit(t, runner, seedDir, "cherry-pick", "-x", trailerSourceSHA)
	runGit(t, runner, seedDir, "push", "origin", "deploy/PROJ-1-to-UAT")
	runGit(t, runner, seedDir, "checkout", "UAT")

	runGit(t, runner, localDir, "fetch", "origin")
	runGit(t, runner, localDir, "checkout", "-b", "deploy/PROJ-1-to-UAT", "origin/deploy/PROJ-1-to-UAT")

	commits := []git.DiscoveredCommit{
		{Commit: git.Commit{SHA: ancestorSHA}},
		{Commit: git.Commit{SHA: trailerSourceSHA}},
		{Commit: git.Commit{SHA: newSHA}},
	}

	remaining, err := svc.FilterNotOnBranch(context.Background(), localDir, "deploy/PROJ-1-to-UAT", "origin/feature/PROJ-1", commits)
	if err != nil {
		t.Fatalf("FilterNotOnBranch: unexpected error: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected exactly 1 remaining commit, got %d: %v", len(remaining), remaining)
	}
	if remaining[0].SHA != newSHA {
		t.Fatalf("expected the remaining commit to be the not-yet-applied one %q, got %q", newSHA, remaining[0].SHA)
	}
}

// TestService_FilterNotOnBranch_EmptySourceRefDegradesSkipsCherry is task
// 1.5 (RED): an empty sourceRef skips the Cherry fallback entirely (no
// error) — the ancestor and trailer layers still run and still filter what
// they can detect.
func TestService_FilterNotOnBranch_EmptySourceRefDegradesSkipsCherry(t *testing.T) {
	localDir, _, seedDir := newTempRepoWithRemote(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	runGit(t, runner, seedDir, "checkout", "-b", "deploy/PROJ-1-to-UAT", "UAT")
	ancestorSHA := writeAndCommit(t, runner, seedDir, "ancestor.txt", "ancestor\n", "PROJ-1 ancestor commit")
	runGit(t, runner, seedDir, "push", "origin", "deploy/PROJ-1-to-UAT")
	runGit(t, runner, seedDir, "checkout", "UAT")

	runGit(t, runner, localDir, "fetch", "origin")
	runGit(t, runner, localDir, "checkout", "-b", "deploy/PROJ-1-to-UAT", "origin/deploy/PROJ-1-to-UAT")

	// A genuinely new commit, created on a SEPARATE local branch never merged
	// into deploy/PROJ-1-to-UAT: neither the ancestor nor trailer layers can
	// detect it, and Cherry is skipped entirely (empty sourceRef) — it must
	// survive the filter.
	runGit(t, runner, localDir, "checkout", "-b", "feature/PROJ-1-local", "deploy/PROJ-1-to-UAT")
	newSHA := writeAndCommit(t, runner, localDir, "new.txt", "new\n", "PROJ-1 not yet applied commit")
	runGit(t, runner, localDir, "checkout", "deploy/PROJ-1-to-UAT")
	commits := []git.DiscoveredCommit{
		{Commit: git.Commit{SHA: ancestorSHA}},
		{Commit: git.Commit{SHA: newSHA}},
	}

	remaining, err := svc.FilterNotOnBranch(context.Background(), localDir, "deploy/PROJ-1-to-UAT", "", commits)
	if err != nil {
		t.Fatalf("FilterNotOnBranch with empty sourceRef: unexpected error: %v", err)
	}
	if len(remaining) != 1 || remaining[0].SHA != newSHA {
		t.Fatalf("expected only the new commit %q to remain (ancestor excluded, Cherry skipped), got %v", newSHA, remaining)
	}
}

// TestService_FilterNotOnBranch_SkipsEquivalentContentViaCherryFallback
// covers the cherry-equivalence fallback layer: a commit whose content was
// reproduced DIRECTLY on the deploy branch WITHOUT `-x` (so it carries NO
// trailer at all) is still correctly skipped via git-cherry's own
// content-equivalence — the layered check's third layer backstops content
// the ancestry and trailer layers cannot see.
func TestService_FilterNotOnBranch_SkipsEquivalentContentViaCherryFallback(t *testing.T) {
	localDir, _, seedDir := newTempRepoWithRemote(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	runGit(t, runner, seedDir, "checkout", "-b", "feature/PROJ-1", "UAT")
	sourceSHA := writeAndCommit(t, runner, seedDir, "y.txt", "y\n", "PROJ-1 normally-applied commit")
	runGit(t, runner, seedDir, "push", "origin", "feature/PROJ-1")

	// Manually reproduce the IDENTICAL diff directly on the deploy branch,
	// WITHOUT `-x` — no trailer is ever written, so only cherry-equivalence
	// can classify this as already-present.
	runGit(t, runner, seedDir, "checkout", "-b", "deploy/PROJ-1-to-UAT", "UAT")
	writeAndCommit(t, runner, seedDir, "y.txt", "y\n", "manually reproduced identical change")
	runGit(t, runner, seedDir, "push", "origin", "deploy/PROJ-1-to-UAT")
	runGit(t, runner, seedDir, "checkout", "UAT")

	runGit(t, runner, localDir, "fetch", "origin")
	runGit(t, runner, localDir, "checkout", "-b", "deploy/PROJ-1-to-UAT", "origin/deploy/PROJ-1-to-UAT")

	commits := []git.DiscoveredCommit{{Commit: git.Commit{SHA: sourceSHA}}}

	remaining, err := svc.FilterNotOnBranch(context.Background(), localDir, "deploy/PROJ-1-to-UAT", "origin/feature/PROJ-1", commits)
	if err != nil {
		t.Fatalf("FilterNotOnBranch: unexpected error: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected the content-equivalent commit to be skipped via cherry alone (no trailer), got %v", remaining)
	}
}

// TestService_FilterNotOnBranch_EmptySourceRef_TrailerFallbackStillSkips is
// the RED fix's companion (c): when sourceRef == "" (git-cherry
// unavailable), the trailer remains the best available signal and still
// correctly classifies a trailer-detected (differently-SHA'd) commit as
// already-present — the fallback path the empty-sourceRef degrade preserves.
func TestService_FilterNotOnBranch_EmptySourceRef_TrailerFallbackStillSkips(t *testing.T) {
	localDir, _, seedDir := newTempRepoWithRemote(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	runGit(t, runner, seedDir, "checkout", "-b", "feature/PROJ-1", "UAT")
	sourceSHA := writeAndCommit(t, runner, seedDir, "x.txt", "x\n", "PROJ-1 trailer-only fallback commit")
	runGit(t, runner, seedDir, "push", "origin", "feature/PROJ-1")

	runGit(t, runner, seedDir, "checkout", "-b", "deploy/PROJ-1-to-UAT", "UAT")
	runGit(t, runner, seedDir, "cherry-pick", "-x", sourceSHA)
	runGit(t, runner, seedDir, "push", "origin", "deploy/PROJ-1-to-UAT")
	runGit(t, runner, seedDir, "checkout", "UAT")

	runGit(t, runner, localDir, "fetch", "origin")
	runGit(t, runner, localDir, "checkout", "-b", "deploy/PROJ-1-to-UAT", "origin/deploy/PROJ-1-to-UAT")

	commits := []git.DiscoveredCommit{{Commit: git.Commit{SHA: sourceSHA}}}

	remaining, err := svc.FilterNotOnBranch(context.Background(), localDir, "deploy/PROJ-1-to-UAT", "", commits)
	if err != nil {
		t.Fatalf("FilterNotOnBranch with empty sourceRef: unexpected error: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected the trailer-detected commit to be skipped via the fallback trailer signal, got %v", remaining)
	}
}
