package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// TestOnBranchCreated_ErrPromotionBranchExists_RoutesToStateBranchCollision
// is task 2.1 (RED): a branchCreatedMsg carrying git.ErrPromotionBranchExists
// routes to the dedicated StateBranchCollision decision, never the terminal
// StateError dead-end (incremental-promotion spec: "Branch Collision Offers
// Reuse, Recreate, Or Cancel" — "SHALL NOT dead-end into an unrecoverable
// error state on this collision").
func TestOnBranchCreated_ErrPromotionBranchExists_RoutesToStateBranchCollision(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
	m.branchName = "deploy/PROJ-1-to-UAT"
	m.state = StateBranchCreation

	next, cmd := m.Update(branchCreatedMsg{err: git.ErrPromotionBranchExists})
	nm := next.(Model)

	if nm.State() != StateBranchCollision {
		t.Fatalf("expected StateBranchCollision, got %v", nm.State())
	}
	if nm.Err() != nil {
		t.Fatalf("a collision must not set a terminal err, got %v", nm.Err())
	}
	if cmd != nil {
		t.Fatalf("entering the collision decision must not fire a command yet, got %v", cmd)
	}
}

// TestOnBranchCreated_OtherError_StaysStateError is task 2.1 (RED)'s
// companion: every OTHER branchCreatedMsg error (not
// git.ErrPromotionBranchExists) still lands on StateError, unchanged.
func TestOnBranchCreated_OtherError_StaysStateError(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
	m.branchName = "deploy/PROJ-1-to-UAT"
	m.state = StateBranchCreation

	unrelated := errors.New("git: fetching origin in /repo: network unreachable")
	next, _ := m.Update(branchCreatedMsg{err: unrelated})
	nm := next.(Model)

	if nm.State() != StateError {
		t.Fatalf("expected an unrelated error to still land StateError, got %v", nm.State())
	}
	if nm.Err() != unrelated {
		t.Fatalf("expected Err() to carry the unrelated error, got %v", nm.Err())
	}
}

// TestKeyBranchCollision_R_D_C is task 2.3 (RED): `r` fires reuseBranchCmd
// (state stays StateBranchCollision until reuseReadyMsg lands); `d` fires a
// command that deletes the local branch then re-creates it fresh; `c`/`esc`
// return to StatePlanPreview untouched (incremental-promotion spec:
// "Collision presents all three choices" / "Cancel leaves the existing
// branch and PR untouched" / "Delete & recreate replaces the branch fresh").
func TestKeyBranchCollision_R_D_C(t *testing.T) {
	base := func() Model {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
		m.branchName = "deploy/PROJ-1-to-UAT"
		m.state = StateBranchCollision
		return m
	}

	t.Run("r fires reuseBranchCmd, state stays put until reuseReadyMsg", func(t *testing.T) {
		m := base()
		next, cmd := m.Update(keyPress("r"))
		nm := next.(Model)
		if nm.State() != StateBranchCollision {
			t.Fatalf("r must not change state synchronously (a command runs first), got %v", nm.State())
		}
		if cmd == nil {
			t.Fatal("r should fire the reuse command")
		}
	})

	t.Run("d fires delete-and-recreate, entering StateBranchCreation", func(t *testing.T) {
		m := base()
		next, cmd := m.Update(keyPress("d"))
		nm := next.(Model)
		if nm.State() != StateBranchCreation {
			t.Fatalf("d should enter StateBranchCreation while delete+recreate runs, got %v", nm.State())
		}
		if cmd == nil {
			t.Fatal("d should fire a command")
		}
	})

	t.Run("c cancels back to StatePlanPreview untouched", func(t *testing.T) {
		m := base()
		next, cmd := m.Update(keyPress("c"))
		nm := next.(Model)
		if nm.State() != StatePlanPreview {
			t.Fatalf("c should return to StatePlanPreview, got %v", nm.State())
		}
		if cmd != nil {
			t.Fatalf("cancel must not fire any command (no git call), got %v", cmd)
		}
	})

	t.Run("esc cancels back to StatePlanPreview untouched", func(t *testing.T) {
		m := base()
		next, cmd := m.Update(keyPress("esc"))
		nm := next.(Model)
		if nm.State() != StatePlanPreview {
			t.Fatalf("esc should return to StatePlanPreview, got %v", nm.State())
		}
		if cmd != nil {
			t.Fatalf("cancel must not fire any command (no git call), got %v", cmd)
		}
	})
}

// callIndex returns the index of the first FakeRunner call whose Args
// contain token, or -1 when no call matches.
func callIndex(fr *execpkg.FakeRunner, token string) int {
	for i, c := range fr.Calls {
		for _, a := range c.Args {
			if a == token {
				return i
			}
		}
	}
	return -1
}

// TestReuseBranchCmd_ChecksOutFastForwardsFilters is task 2.5 (RED):
// reuseBranchCmd runs, in order, Checkout -> FastForwardBranch ->
// FilterNotOnBranch(deployBranch, sourceRef, m.plan.SelectedCommits) — never
// reordered, never skipping the ff-guard before filtering (design.md's
// Data Flow: "r -> reuseBranchCmd: Checkout -> FastForwardBranch ->
// FilterNotOnBranch").
func TestReuseBranchCmd_ChecksOutFastForwardsFilters(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	commitSHA := "cccccccccccccccccccccccccccccccccccccccc"

	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"checkout", branch, "--"}, execpkg.CommandResult{ExitCode: 0})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "origin/" + branch}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("deadbeef")})
	fr.When("git", []string{"merge-base", "--is-ancestor", branch, "origin/" + branch}, execpkg.CommandResult{ExitCode: 0})
	fr.When("git", []string{"merge-base", "--is-ancestor", "origin/" + branch, branch}, execpkg.CommandResult{ExitCode: 0})
	fr.When("git", []string{"log", branch}, execpkg.CommandResult{ExitCode: 0})
	fr.When("git", []string{"merge-base", "--is-ancestor", commitSHA, branch}, execpkg.CommandResult{ExitCode: 1})

	m := New(Deps{Dir: "/repo", Config: testConfig(), Git: git.New(fr)})
	m.branchName = branch
	m.plan = git.DeploymentPlan{
		Ticket:       "PROJ-1",
		TargetBranch: "UAT",
		SelectedCommits: []git.DiscoveredCommit{
			{Commit: git.Commit{SHA: commitSHA}},
		},
	}

	msg := run(t, m.reuseBranchCmd())
	got, ok := msg.(reuseReadyMsg)
	if !ok {
		t.Fatalf("expected a reuseReadyMsg, got %#v", msg)
	}
	if got.err != nil {
		t.Fatalf("reuseBranchCmd: unexpected error: %v", got.err)
	}
	if got.ffResult != git.FFUpToDate {
		t.Fatalf("ffResult = %v, want FFUpToDate", got.ffResult)
	}
	if len(got.remaining) != 1 || got.remaining[0].SHA != commitSHA {
		t.Fatalf("expected the not-yet-ancestor commit to remain, got %v", got.remaining)
	}

	checkoutIdx := callIndex(fr, branch)
	ffIdx := callIndex(fr, "origin/"+branch)
	filterIdx := -1
	for i, c := range fr.Calls {
		if len(c.Args) == 2 && c.Args[0] == "log" && c.Args[1] == branch {
			filterIdx = i
			break
		}
	}
	if checkoutIdx == -1 || ffIdx == -1 || filterIdx == -1 {
		t.Fatalf("expected all three phases to have run; calls: %v", fr.Calls)
	}
	if !(checkoutIdx < ffIdx && ffIdx < filterIdx) {
		t.Fatalf("expected order Checkout(%d) -> FastForwardBranch(%d) -> FilterNotOnBranch(%d)", checkoutIdx, ffIdx, filterIdx)
	}
}

// TestOnReuseReady is task 2.7 (RED): onReuseReady's full decision table
// (incremental-promotion spec + design.md's Data Flow):
//   - FFDiverged or a genuine err -> StateError, a clear message, no
//     push/overwrite attempted.
//   - len(remaining)==0 -> an explicit "nothing new" notice, then
//     StatePushPreparation (idempotent push + PR check), never a raw
//     cherry-pick error.
//   - a non-empty remainder with a matching prior run (runs.FindRunForBranch)
//     -> that SAME record is mutated in place (Commits extended, PickTotal/
//     UpdatedAt bumped, CreatedAt/RunID preserved) and saved, then
//     StateCherryPicking.
//   - a non-empty remainder with NO matching prior run -> proceeds as a
//     fresh increment target, no error.
func TestOnReuseReady(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	newCommit := git.DiscoveredCommit{Commit: git.Commit{SHA: "cccccccccccccccccccccccccccccccccccccccc"}}

	t.Run("FFDiverged lands StateError with no push/overwrite", func(t *testing.T) {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.branchName = branch
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}

		next, cmd := m.Update(reuseReadyMsg{ffResult: git.FFDiverged})
		nm := next.(Model)
		if nm.State() != StateError {
			t.Fatalf("expected StateError on divergence, got %v", nm.State())
		}
		if nm.Err() == nil {
			t.Fatal("expected a clear divergence error message")
		}
		if cmd != nil {
			t.Fatalf("divergence must never fire a follow-up command (no push/overwrite), got %v", cmd)
		}
	})

	t.Run("a genuine error lands StateError", func(t *testing.T) {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.branchName = branch
		wantErr := errors.New("git: checking out deploy/PROJ-1-to-UAT: boom")

		next, _ := m.Update(reuseReadyMsg{err: wantErr})
		nm := next.(Model)
		if nm.State() != StateError {
			t.Fatalf("expected StateError, got %v", nm.State())
		}
		if nm.Err() != wantErr {
			t.Fatalf("expected Err() to carry the original error, got %v", nm.Err())
		}
	})

	t.Run("empty remainder shows a notice and enters StatePushPreparation", func(t *testing.T) {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.branchName = branch
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}

		next, cmd := m.Update(reuseReadyMsg{ffResult: git.FFUpToDate, remaining: nil})
		nm := next.(Model)
		if nm.State() != StatePushPreparation {
			t.Fatalf("expected StatePushPreparation, got %v", nm.State())
		}
		if nm.notice == "" {
			t.Fatal("expected an explicit 'nothing new to apply' notice")
		}
		if cmd != nil {
			t.Fatalf("entering push preparation must not fire a command yet, got %v", cmd)
		}
	})

	t.Run("remainder with a matching prior run mutates it in place", func(t *testing.T) {
		writer := runs.NewWriter(t.TempDir())
		created := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		prior := runs.Record{
			RunID:     "PROJ-1-to-UAT-20240101000000",
			Ticket:    "PROJ-1",
			Target:    "UAT",
			Commits:   []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			PickTotal: 1,
			CreatedAt: created,
			UpdatedAt: created,
		}
		if err := writer.Save(prior); err != nil {
			t.Fatalf("seeding prior run: %v", err)
		}

		clock := &fakeClock{t: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)}
		m := New(Deps{Dir: "/repo", Config: testConfig(), Runs: writer, Now: clock.now})
		m.branchName = branch
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
		// m.runs mirrors the in-memory run history resumeDetectCmd already
		// loaded at startup (onResumeDetect's m.runs = records) — the SAME
		// source every other FindRunForBranch-style correlation in this
		// codebase (selectOrphans, resolveMergeTarget) reads from.
		m.runs = []runs.Record{prior}

		next, cmd := m.Update(reuseReadyMsg{ffResult: git.FFUpToDate, remaining: []git.DiscoveredCommit{newCommit}})
		nm := next.(Model)

		if nm.State() != StateCherryPicking {
			t.Fatalf("expected StateCherryPicking, got %v", nm.State())
		}
		if !nm.reusing {
			t.Fatal("expected m.reusing to be set true")
		}
		if cmd == nil {
			t.Fatal("expected the cherry-pick command to fire")
		}
		if nm.runID != prior.RunID {
			t.Fatalf("expected the SAME RunID to be targeted, got %q want %q", nm.runID, prior.RunID)
		}

		rec, err := writer.Load(prior.RunID)
		if err != nil {
			t.Fatalf("loading mutated run: %v", err)
		}
		if rec.RunID != prior.RunID {
			t.Fatalf("RunID must be preserved, got %q", rec.RunID)
		}
		if !rec.CreatedAt.Equal(created) {
			t.Fatalf("CreatedAt must be preserved, got %v want %v", rec.CreatedAt, created)
		}
		if len(rec.Commits) != 2 {
			t.Fatalf("expected Commits extended to 2 entries, got %v", rec.Commits)
		}
		if rec.Commits[0] != prior.Commits[0] || rec.Commits[1] != newCommit.SHA {
			t.Fatalf("expected the prior commit preserved and the new one appended, got %v", rec.Commits)
		}
		if rec.PickTotal != 2 {
			t.Fatalf("expected PickTotal bumped to 2, got %d", rec.PickTotal)
		}
		if !rec.UpdatedAt.Equal(clock.t) {
			t.Fatalf("expected UpdatedAt bumped to the injected clock, got %v want %v", rec.UpdatedAt, clock.t)
		}
		if rec.SourceRunID != "" {
			t.Fatalf("increment must never set SourceRunID, got %q", rec.SourceRunID)
		}
	})

	t.Run("remainder with no matching prior run proceeds as a fresh increment target", func(t *testing.T) {
		writer := runs.NewWriter(t.TempDir())
		m := New(Deps{Dir: "/repo", Config: testConfig(), Runs: writer})
		m.branchName = branch
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}

		next, cmd := m.Update(reuseReadyMsg{ffResult: git.FFUpToDate, remaining: []git.DiscoveredCommit{newCommit}})
		nm := next.(Model)

		if nm.State() != StateCherryPicking {
			t.Fatalf("expected StateCherryPicking (no error on a no-match), got %v", nm.State())
		}
		if nm.runID == "" {
			t.Fatal("expected a fresh runID to be seeded")
		}
		if cmd == nil {
			t.Fatal("expected the cherry-pick command to fire")
		}
	})
}

// cannPRForBranch registers `gh pr view <branch> --json url,state`'s canned
// response, mirroring push_preparation_test.go's cannPRCreate helper.
func cannPRForBranch(fr *execpkg.FakeRunner, branch string, result execpkg.CommandResult) {
	fr.When("gh", []string{"pr", "view", branch, "--json", "url,state"}, result)
}

// TestPreparePRCmd_Reusing_SkipsCreateWhenPROpen is task 2.9 (RED):
// preparePRCmd, on the reuse path, ALSO checks PRForBranch when
// authenticated; an OPEN PR is reflected on prepDoneMsg so onPrepDone can
// skip gh pr create and show the existing URL directly (incremental-
// promotion design: "PR detection ... OPEN -> skip create (set prURL)").
func TestPreparePRCmd_Reusing_SkipsCreateWhenPROpen(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	prURL := "https://github.com/org/repo/pull/9"

	fr := ghRunner("authed")
	cannPRForBranch(fr, branch, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(`{"url":"` + prURL + `","state":"OPEN"}`)})

	gitFR := execpkg.NewFakeRunner()
	gitFR.When("git", []string{"remote", "get-url", "origin"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("git@github.com:org/repo.git")})
	gitFR.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})

	m := New(Deps{Dir: "/repo", Config: testConfig(), Git: git.New(gitFR), GH: github.New(fr)})
	m.reusing = true
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}

	msg := run(t, m.preparePRCmd())
	pm, ok := msg.(prepDoneMsg)
	if !ok {
		t.Fatalf("expected a prepDoneMsg, got %#v", msg)
	}
	if !pm.prOpen {
		t.Fatal("expected prepDoneMsg.prOpen to be true for an OPEN PR")
	}
	if pm.prURL != prURL {
		t.Fatalf("prepDoneMsg.prURL = %q, want %q", pm.prURL, prURL)
	}

	next, cmd := m.Update(pm)
	nm := next.(Model)
	if nm.prURL != prURL {
		t.Fatalf("model prURL = %q, want %q", nm.prURL, prURL)
	}
	if nm.prErr != nil {
		t.Fatalf("expected no prErr, got %v", nm.prErr)
	}
	if cmd != nil {
		// No AI suggestion or any other follow-up should fire when the PR
		// is already open — nothing left to prepare.
		t.Fatalf("expected no follow-up command when the PR is already open, got %v", cmd)
	}

	if n := prCreateCalls(fr); n != 0 {
		t.Fatalf("gh pr create must never run when an open PR already exists, got %d call(s)", n)
	}
}

// TestPreparePRCmd_Reusing_ClosedFallsThroughToCreateWithNotice is task 2.9
// (RED)'s companion: a CLOSED/MERGED PR still reports its URL (open=false)
// but onPrepDone falls through to the normal create path with a notice,
// rather than silently reusing the stale closed PR.
func TestPreparePRCmd_Reusing_ClosedFallsThroughToCreateWithNotice(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	prURL := "https://github.com/org/repo/pull/3"

	fr := ghRunner("authed")
	cannPRForBranch(fr, branch, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(`{"url":"` + prURL + `","state":"CLOSED"}`)})

	gitFR := execpkg.NewFakeRunner()
	gitFR.When("git", []string{"remote", "get-url", "origin"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("git@github.com:org/repo.git")})
	gitFR.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})

	m := New(Deps{Dir: "/repo", Config: testConfig(), Git: git.New(gitFR), GH: github.New(fr)})
	m.reusing = true
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}

	msg := run(t, m.preparePRCmd())
	pm := msg.(prepDoneMsg)
	if pm.prOpen {
		t.Fatal("expected prepDoneMsg.prOpen to be false for a CLOSED PR")
	}
	if pm.prURL != prURL {
		t.Fatalf("prepDoneMsg.prURL = %q, want %q (closed/merged still carries its URL)", pm.prURL, prURL)
	}

	next, _ := m.Update(pm)
	nm := next.(Model)
	if nm.prURL != "" {
		t.Fatalf("model prURL should stay empty (no PR created yet), got %q", nm.prURL)
	}
	if nm.pushPhase != pushReady {
		t.Fatalf("expected the normal pushReady screen (create still offered), got %v", nm.pushPhase)
	}
	if nm.notice == "" {
		t.Fatal("expected a notice explaining the prior PR is closed/merged")
	}
}

// TestPreparePRCmd_NotReusing_NeverCallsPRForBranch proves PRForBranch is
// consulted ONLY on the reuse path — an ordinary first-time promotion
// (m.reusing == false) never pays for the extra gh pr view lookup.
func TestPreparePRCmd_NotReusing_NeverCallsPRForBranch(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"
	fr := ghRunner("authed")

	gitFR := execpkg.NewFakeRunner()
	gitFR.When("git", []string{"remote", "get-url", "origin"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("git@github.com:org/repo.git")})
	gitFR.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})

	m := New(Deps{Dir: "/repo", Config: testConfig(), Git: git.New(gitFR), GH: github.New(fr)})
	m.reusing = false
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: branch}

	run(t, m.preparePRCmd())

	for _, c := range fr.Calls {
		if c.Name == "gh" && len(c.Args) >= 2 && c.Args[0] == "pr" && c.Args[1] == "view" {
			t.Fatalf("PRForBranch must not be called on a non-reuse promotion, calls: %v", fr.Calls)
		}
	}
}

// TestViewBranchCollision_ShowsThreeChoices is task 2.11 (RED):
// StateBranchCollision's screen renders all three choices (incremental-
// promotion spec: "Collision presents all three choices").
func TestViewBranchCollision_ShowsThreeChoices(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateBranchCollision
	m.branchName = "deploy/PROJ-1-to-UAT"

	v := m.View()
	for _, want := range []string{"reus", "borr", "cancel"} {
		if !strings.Contains(strings.ToLower(v), want) {
			t.Errorf("expected the collision screen to mention %q, got:\n%s", want, v)
		}
	}
}

// hasCall reports whether fr recorded a call matching name and args EXACTLY
// (unlike callIndex's loose single-token search), so the delete-and-recreate
// tests below can assert precisely which git subprocess did or did not run.
func hasCall(fr *execpkg.FakeRunner, name string, args ...string) bool {
	for _, c := range fr.Calls {
		if c.Name != name || len(c.Args) != len(args) {
			continue
		}
		match := true
		for i := range args {
			if c.Args[i] != args[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// TestDeleteExistingDeployBranch is the verify-review RED fix ("delete &
// recreate must handle the remote branch and origin-only collisions"): the
// OLD deleteAndRecreateBranchCmd called DeleteLocalBranch unconditionally,
// which ERRORS on a pure origin-only collision (no local branch at all) —
// dead-ending to StateError — and it never deleted the REMOTE (origin)
// deploy branch, so CreatePromotionBranch's own BranchExists guard (which
// checks origin too) would still collide even after the local delete
// succeeded. deleteExistingDeployBranch fixes both: it exists-guards each
// side independently (tolerating either being absent, no error) and deletes
// only what is actually there.
func TestDeleteExistingDeployBranch(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"

	t.Run("origin-only collision deletes only the remote branch, no error", func(t *testing.T) {
		fr := execpkg.NewFakeRunner()
		fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
		fr.When("git", []string{"rev-parse", "--verify", "--quiet", branch}, execpkg.CommandResult{ExitCode: 1})
		fr.When("git", []string{"rev-parse", "--verify", "--quiet", "origin/" + branch}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("deadbeef")})
		fr.When("git", []string{"push", "origin", "--delete", "--", branch}, execpkg.CommandResult{ExitCode: 0})

		g := git.New(fr)
		if err := deleteExistingDeployBranch(context.Background(), g, "/repo", branch); err != nil {
			t.Fatalf("deleteExistingDeployBranch: unexpected error: %v", err)
		}
		if hasCall(fr, "git", "branch", "-D", "--", branch) {
			t.Fatalf("must never call DeleteLocalBranch when the local branch does not exist (it would error), calls: %v", fr.Calls)
		}
		if !hasCall(fr, "git", "push", "origin", "--delete", "--", branch) {
			t.Fatalf("expected the remote deploy branch to be deleted, calls: %v", fr.Calls)
		}
	})

	t.Run("local+remote collision deletes both", func(t *testing.T) {
		fr := execpkg.NewFakeRunner()
		fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
		fr.When("git", []string{"rev-parse", "--verify", "--quiet", branch}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("deadbeef")})
		fr.When("git", []string{"rev-parse", "--verify", "--quiet", "origin/" + branch}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("deadbeef")})
		fr.When("git", []string{"branch", "-D", "--", branch}, execpkg.CommandResult{ExitCode: 0})
		fr.When("git", []string{"push", "origin", "--delete", "--", branch}, execpkg.CommandResult{ExitCode: 0})

		g := git.New(fr)
		if err := deleteExistingDeployBranch(context.Background(), g, "/repo", branch); err != nil {
			t.Fatalf("deleteExistingDeployBranch: unexpected error: %v", err)
		}
		if !hasCall(fr, "git", "branch", "-D", "--", branch) {
			t.Fatalf("expected the local deploy branch to be deleted, calls: %v", fr.Calls)
		}
		if !hasCall(fr, "git", "push", "origin", "--delete", "--", branch) {
			t.Fatalf("expected the remote deploy branch to be deleted, calls: %v", fr.Calls)
		}
	})

	t.Run("delete of an absent local branch does not error (neither side exists)", func(t *testing.T) {
		fr := execpkg.NewFakeRunner()
		fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
		fr.When("git", []string{"rev-parse", "--verify", "--quiet", branch}, execpkg.CommandResult{ExitCode: 1})
		fr.When("git", []string{"rev-parse", "--verify", "--quiet", "origin/" + branch}, execpkg.CommandResult{ExitCode: 1})

		g := git.New(fr)
		if err := deleteExistingDeployBranch(context.Background(), g, "/repo", branch); err != nil {
			t.Fatalf("deleteExistingDeployBranch: unexpected error for an absent local branch, got: %v", err)
		}
		if hasCall(fr, "git", "branch", "-D", "--", branch) || hasCall(fr, "git", "push", "origin", "--delete", "--", branch) {
			t.Fatalf("expected no delete calls when neither side exists, calls: %v", fr.Calls)
		}
	})
}

// TestDeleteAndRecreateBranchCmd_OriginOnlyCollision_NeverCallsDeleteLocalBranch
// is the same fix's integration proof at the actual command entrypoint
// (deleteAndRecreateBranchCmd), rather than the extracted helper directly:
// on an origin-only collision, the OLD code's unconditional DeleteLocalBranch
// call would have failed outright (a raw, non-ErrPromotionBranchExists error)
// and dead-ended onBranchCreated straight to StateError. This proves that
// specific failure mode is gone, and that the remote branch delete is
// actually attempted before recreate.
func TestDeleteAndRecreateBranchCmd_OriginOnlyCollision_NeverCallsDeleteLocalBranch(t *testing.T) {
	branch := "deploy/PROJ-1-to-UAT"

	fr := execpkg.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", branch}, execpkg.CommandResult{ExitCode: 1})
	fr.When("git", []string{"rev-parse", "--verify", "--quiet", "origin/" + branch}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("deadbeef")})
	fr.When("git", []string{"push", "origin", "--delete", "--", branch}, execpkg.CommandResult{ExitCode: 0})
	// CreatePromotionBranch's own re-check runs next (fetch + BranchExists);
	// canned to keep failing on the SAME static "exists" response a
	// deterministic FakeRunner necessarily still returns post-delete (it has
	// no notion of state change) — this re-collision is a fake-fidelity
	// artifact, not a production behavior, and is exactly why
	// TestDeleteExistingDeployBranch above asserts the actual delete calls
	// directly instead of relying on full recreate success here.
	fr.When("git", []string{"fetch", "origin"}, execpkg.CommandResult{ExitCode: 0})

	m := New(Deps{Dir: "/repo", Config: testConfig(), Git: git.New(fr)})
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
	m.branchName = branch

	msg := run(t, m.deleteAndRecreateBranchCmd())
	bm, ok := msg.(branchCreatedMsg)
	if !ok {
		t.Fatalf("expected a branchCreatedMsg, got %#v", msg)
	}

	if hasCall(fr, "git", "branch", "-D", "--", branch) {
		t.Fatalf("must never call DeleteLocalBranch on an origin-only collision, calls: %v", fr.Calls)
	}
	if !hasCall(fr, "git", "push", "origin", "--delete", "--", branch) {
		t.Fatalf("expected the remote deploy branch to be deleted, calls: %v", fr.Calls)
	}

	// The load-bearing regression check: routing the resulting message must
	// NEVER dead-end to StateError via a raw (non-collision) delete error —
	// the exact bug this fix removes. Re-entering StateBranchCollision (the
	// re-check's ErrPromotionBranchExists path, per the FakeRunner caveat
	// above) is an acceptable outcome here; StateError is not.
	next, _ := m.Update(bm)
	nm := next.(Model)
	if nm.State() == StateError {
		t.Fatalf("delete & recreate must never dead-end to StateError on an origin-only collision, err: %v", nm.Err())
	}
}
