package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	execpkg "github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// TestIncrementalPromotion_E2E_ReuseAppendsOnlyNewCommits_SkipsCreateWhenPROpen
// is task 2.13 (RED/E2E): the full incremental-promotion reuse flow against a
// REAL temp git repo (fake gh only). It proves the three load-bearing
// properties in one continuous drive:
//
//  1. the reused branch's cherry-pick sequence appends ONLY the not-yet-
//     present commit (the prior session's commit A stays untouched, only B
//     is picked) — incremental-promotion spec: "Only new commits are
//     cherry-picked".
//  2. the SAME prior RunID's record is mutated in place (Commits extended,
//     PickTotal bumped, CreatedAt/RunID preserved) — never a new run record
//     — run-persistence spec: "Reuse appends to the existing run record".
//  3. once pushed, an already-OPEN PR for the branch skips `gh pr create`
//     entirely and shows the existing URL — push-pr-preparation spec:
//     "Open PR already exists for the branch".
func TestIncrementalPromotion_E2E_ReuseAppendsOnlyNewCommits_SkipsCreateWhenPROpen(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}

	local := setupFlowRepo(t)
	branch := "deploy/PROJ-1-to-UAT"

	// The two ticket commits feature/PROJ-1 carries (earliest first).
	shaLines := strings.Fields(gitOut(t, local, "log", "--format=%H", "--reverse", "origin/UAT..origin/feature/PROJ-1"))
	if len(shaLines) != 2 {
		t.Fatalf("expected 2 commits on feature/PROJ-1, got %d: %v", len(shaLines), shaLines)
	}
	aSHA, bSHA := shaLines[0], shaLines[1]

	// Simulate an EARLIER session: deploy/PROJ-1-to-UAT already exists
	// (locally here — the "collides locally" collision shape), carrying
	// ONLY commit A, cherry-picked with -x so the reused-branch trailer
	// layer can detect it.
	gitRun(t, local, "checkout", "-b", branch, "origin/UAT")
	gitRun(t, local, "cherry-pick", "-x", aSHA)
	gitRun(t, local, "push", "origin", branch)
	gitRun(t, local, "checkout", "main")

	writer := runs.NewWriter(local)
	priorCreated := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	priorRunID := "PROJ-1-to-UAT-priorsession"
	prior := runs.Record{
		RunID:     priorRunID,
		Ticket:    "PROJ-1",
		Target:    "UAT",
		Commits:   []string{aSHA},
		PickTotal: 1,
		Phase:     "done",
		CreatedAt: priorCreated,
		UpdatedAt: priorCreated,
	}
	if err := writer.Save(prior); err != nil {
		t.Fatalf("seeding prior run: %v", err)
	}

	prURL := "https://github.com/org/repo/pull/5"
	ghFake := ghRunner("authed")
	cannPRForBranch(ghFake, branch, execpkg.CommandResult{ExitCode: 0, Stdout: []byte(`{"url":"` + prURL + `","state":"OPEN"}`)})

	m := New(Deps{
		Dir:    local,
		Config: flowConfig(),
		Git:    git.New(execpkg.NewOSRunner()),
		GH:     github.New(ghFake),
		Runs:   writer,
	})
	m.state = StateBranchCreation
	m.plan = git.DeploymentPlan{
		Ticket:       "PROJ-1",
		TargetBranch: "UAT",
		SelectedCommits: []git.DiscoveredCommit{
			{Commit: git.Commit{SHA: aSHA}},
			{Commit: git.Commit{SHA: bSHA}},
		},
	}
	m.branchName = branch
	m.source = git.Branch{Name: "origin/feature/PROJ-1"}
	// m.runs mirrors resumeDetectCmd's startup load (onResumeDetect's
	// m.runs = records) — the prior session's run is already known.
	m.runs = []runs.Record{prior}

	// BranchCreation hits the real collision (git.ErrPromotionBranchExists).
	m = advance(t, m, run(t, m.branchCreateCmd()))
	if m.State() != StateBranchCollision {
		t.Fatalf("expected StateBranchCollision, got %v (err=%v)", m.State(), m.Err())
	}

	// r -> reuseBranchCmd (Checkout -> FastForwardBranch -> FilterNotOnBranch).
	next, cmd := m.Update(keyPress("r"))
	m = next.(Model)
	if cmd == nil {
		t.Fatal("expected the reuse command to fire")
	}
	reuseMsg := run(t, cmd)
	m = advance(t, m, reuseMsg)
	if m.State() != StateCherryPicking {
		t.Fatalf("expected StateCherryPicking after reuse, got %v (err=%v)", m.State(), m.Err())
	}
	if !m.reusing {
		t.Fatal("expected m.reusing to be true")
	}
	if m.runID != priorRunID {
		t.Fatalf("expected the SAME prior RunID to be targeted, got %q want %q", m.runID, priorRunID)
	}
	if len(m.plan.SelectedCommits) != 1 || m.plan.SelectedCommits[0].SHA != bSHA {
		t.Fatalf("expected the pick sequence to be narrowed to ONLY the new commit B, got %v", m.plan.SelectedCommits)
	}

	// Property 2: the SAME run record was mutated in place.
	rec, err := writer.Load(priorRunID)
	if err != nil {
		t.Fatalf("loading mutated run: %v", err)
	}
	if rec.RunID != priorRunID {
		t.Fatalf("RunID must be preserved, got %q", rec.RunID)
	}
	if !rec.CreatedAt.Equal(priorCreated) {
		t.Fatalf("CreatedAt must be preserved, got %v want %v", rec.CreatedAt, priorCreated)
	}
	if len(rec.Commits) != 2 || rec.Commits[0] != aSHA || rec.Commits[1] != bSHA {
		t.Fatalf("expected Commits extended to [A, B], got %v", rec.Commits)
	}
	if rec.PickTotal != 2 {
		t.Fatalf("expected PickTotal bumped to 2, got %d", rec.PickTotal)
	}

	// Property 1: only the new commit is actually cherry-picked onto the
	// reused branch — run the REAL pick.
	m = advance(t, m, run(t, m.cherryPickCmd()))
	if m.State() != StateCherryPicking {
		t.Fatalf("expected to stay in CherryPicking pending verify, got %v (err=%v)", m.State(), m.Err())
	}
	m = advance(t, m, run(t, m.verifyCmd()))
	if m.State() != StatePickVerification {
		t.Fatalf("expected StatePickVerification after a clean pick, got %v (err=%v)", m.State(), m.Err())
	}
	// The reused branch's HEAD now carries exactly base+A+B: b.cls exists
	// (freshly picked) and a.cls exists (from the prior session) — proving
	// A was never re-picked (a re-pick of an already-applied commit onto
	// itself would either conflict or no-op; either way this asserts the
	// pick sequence only ran B).
	if _, statErr := os.Stat(filepath.Join(local, "b.cls")); statErr != nil {
		t.Fatalf("expected b.cls to exist after picking the new commit: %v", statErr)
	}
	if got := gitOut(t, local, "log", "--oneline", "-1"); !strings.Contains(got, "PROJ-1 add B") {
		t.Fatalf("expected the reused branch's tip to be the newly picked commit B, got %q", got)
	}

	// Property 3: once pushed, an already-OPEN PR skips gh pr create.
	m.state = StateSucceeded

	next, _ = m.Update(keyPress("p"))
	m = next.(Model)
	if m.State() != StatePushPreparation || m.pushPhase != pushConfirm {
		t.Fatalf("p should enter push preparation, got state=%v phase=%v", m.State(), m.pushPhase)
	}

	next, cmd = m.Update(keyPress("p"))
	m = next.(Model)
	pushMsg := run(t, cmd)
	if pd, ok := pushMsg.(pushDoneMsg); !ok || pd.err != nil {
		t.Fatalf("real git push should succeed, got %#v", pushMsg)
	}
	m = advance(t, m, pushMsg)

	m = advance(t, m, run(t, m.preparePRCmd()))
	if m.pushPhase != pushReady {
		t.Fatalf("expected pushReady after prep, got %v", m.pushPhase)
	}
	if m.prURL != prURL {
		t.Fatalf("expected the ALREADY-OPEN PR's URL to be shown, got %q want %q", m.prURL, prURL)
	}
	if m.prErr != nil {
		t.Fatalf("expected no prErr, got %v", m.prErr)
	}
	if n := prCreateCalls(ghFake); n != 0 {
		t.Fatalf("gh pr create must never run when a PR is already open, got %d call(s)", n)
	}
	if v := m.View(); !strings.Contains(v, prURL) {
		t.Errorf("view should show the existing PR URL\n%s", v)
	}
}
