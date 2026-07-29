package app

import (
	"testing"
	"time"

	execpkg "deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
	"deploydeck/internal/runs"
	"deploydeck/internal/salesforce"
)

// setupCleanRepo initializes a real, clean temp git repo with one commit — a
// repo where RepoState reports InProgress=false (no cherry-pick), used by the
// jobId re-attach path which does not depend on any live cherry-pick.
func setupCleanRepo(t *testing.T) string {
	t.Helper()
	local := t.TempDir()
	gitRun(t, local, "init", "-b", "main", local)
	gitRun(t, local, "config", "user.name", "ana")
	gitRun(t, local, "config", "user.email", "ana@example.com")
	writeFile(t, local, "base.txt", "base\n")
	gitRun(t, local, "add", ".")
	gitRun(t, local, "commit", "-m", "base")
	return local
}

// reportSF builds a Salesforce client over a FakeRunner canned for a single
// `deploy report` poll of jobID against alias, returning the given status.
func reportSF(t *testing.T, jobID, alias, status string) salesforce.Client {
	t.Helper()
	fr := execpkg.NewFakeRunner()
	fr.When("sf", []string{
		"project", "deploy", "report",
		"--job-id", jobID,
		"--target-org", alias,
		"--json",
	}, execpkg.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"status":0,"result":{"status":"` + status + `","numberComponentsTotal":10,"numberComponentsDeployed":4}}`),
	})
	return salesforce.New(fr)
}

// --- 4.1: resumeDetectCmd composes RepoState + Runs.List --------------------

// TestResumeDetectCmd_RealInProgressCherryPick is task 4.1 (RED): after a real
// in-progress cherry-pick leaves CHERRY_PICK_HEAD + a matching run.json, a
// FRESH model's resumeDetectCmd returns a resumeDetectMsg reflecting the live
// RepoState (InProgress) and the persisted run list.
func TestResumeDetectCmd_RealInProgressCherryPick(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local := setupConflictRepo2(t)
	m, writer := driveToCherryPicking(t, local)
	m = advance(t, m, run(t, m.cherryPickCmd()))
	if m.State() != StateCherryPickConflict {
		t.Fatalf("expected a conflict, got %v (err=%v)", m.State(), m.Err())
	}
	runID := m.runID

	// Fresh model — nothing in memory, everything read from disk + repo.
	fresh := New(Deps{Git: git.New(execpkg.NewOSRunner()), Config: pickIndexConfig(), Dir: local, Runs: writer})
	msg := run(t, fresh.resumeDetectCmd())
	rmsg, ok := msg.(resumeDetectMsg)
	if !ok {
		t.Fatalf("expected resumeDetectMsg, got %T", msg)
	}
	if rmsg.err != nil {
		t.Fatalf("resumeDetectCmd errored: %v", rmsg.err)
	}
	if !rmsg.state.InProgress {
		t.Error("live RepoState should report an in-progress cherry-pick")
	}
	if len(rmsg.records) == 0 {
		t.Fatal("Runs.List should return the persisted run")
	}
	found := false
	for _, rec := range rmsg.records {
		if rec.RunID == runID {
			found = true
		}
	}
	if !found {
		t.Errorf("resumeDetectMsg.records should contain the persisted run %q", runID)
	}
}

// TestResumeDetectCmd_NilDepsIsNoOp proves resume-detection is best-effort: a
// nil Git or Runs writer yields no command (no panic, normal flow).
func TestResumeDetectCmd_NilDepsIsNoOp(t *testing.T) {
	if cmd := New(Deps{Dir: "/repo", Config: validationConfig()}).resumeDetectCmd(); cmd != nil {
		t.Error("resumeDetectCmd with nil Git/Runs should return nil (no detection)")
	}
}

// --- 4.3/4.4: onResumeDetect routing ---------------------------------------

// TestOnResumeDetect_RoutesToRunHistoryWhenResumable is tasks 4.3/4.4 (RED): a
// matching in-progress cherry-pick (or a non-terminal jobId) offers resume via
// StateRunHistory pre-selected; nothing resumable proceeds to the HU-018
// StateMainMenu landing.
func TestOnResumeDetect_RoutesToRunHistoryWhenResumable(t *testing.T) {
	t.Run("matching in-progress cherry-pick offers resume via history", func(t *testing.T) {
		m := New(Deps{Dir: t.TempDir(), Config: validationConfig(), Runs: runs.NewWriter(t.TempDir())})
		// HU-018: the pre-detect landing is now StateMainMenu (onPrereqDone),
		// not StateTicketInput; onResumeDetect's guard keys off it (task 2.14).
		m.state = StateMainMenu
		state := git.RepoState{InProgress: true, CurrentSHA: "sha-A", SequencerRemaining: 2}
		records := []runs.Record{
			{RunID: "old", Ticket: "PROJ-9", Status: "Succeeded", Phase: "done"},
			{RunID: "live", Ticket: "PROJ-1", Commits: []string{"sha-A", "sha-B"}, PickTotal: 2, Phase: "git-conflict"},
		}
		next, _ := m.Update(resumeDetectMsg{state: state, records: records})
		nm := next.(Model)
		if nm.State() != StateRunHistory {
			t.Fatalf("a resumable run should offer resume via StateRunHistory, got %v", nm.State())
		}
		if len(nm.runs) != 2 {
			t.Fatalf("history should hold all runs, got %d", len(nm.runs))
		}
		if nm.runs[nm.runsCursor].RunID != "live" {
			t.Errorf("history should pre-select the resumable run, got cursor on %q", nm.runs[nm.runsCursor].RunID)
		}
	})

	t.Run("non-terminal jobId offers resume via history", func(t *testing.T) {
		m := New(Deps{Dir: t.TempDir(), Config: validationConfig(), Runs: runs.NewWriter(t.TempDir())})
		// HU-018 landing (see above): guard keys off StateMainMenu.
		m.state = StateMainMenu
		records := []runs.Record{
			{RunID: "job", Ticket: "PROJ-1", JobID: "JOB1", Status: "InProgress", Phase: "validating"},
		}
		next, _ := m.Update(resumeDetectMsg{state: git.RepoState{Clean: true}, records: records})
		nm := next.(Model)
		if nm.State() != StateRunHistory {
			t.Fatalf("a non-terminal jobId should offer resume, got %v", nm.State())
		}
		if nm.runs[nm.runsCursor].RunID != "job" {
			t.Errorf("history should pre-select the resumable jobId run")
		}
	})

	t.Run("nothing resumable proceeds to the menu", func(t *testing.T) {
		m := New(Deps{Dir: t.TempDir(), Config: validationConfig(), Runs: runs.NewWriter(t.TempDir())})
		// HU-018 landing (task 2.14): guard keys off StateMainMenu.
		m.state = StateMainMenu
		records := []runs.Record{
			{RunID: "done", Ticket: "PROJ-1", JobID: "JOB1", Status: "Succeeded", Phase: "done"},
		}
		next, _ := m.Update(resumeDetectMsg{state: git.RepoState{Clean: true}, records: records})
		nm := next.(Model)
		if nm.State() != StateMainMenu {
			t.Fatalf("no resumable run should proceed to StateMainMenu, got %v", nm.State())
		}
		// Silent-pass repair (task 2.15): assert the run list was actually
		// reconciled and stored. onResumeDetect sets m.runs ONLY on the real
		// no-resume fallthrough (after reconcileStaleRuns runs) — a path
		// unreachable if the state guard no-ops early. Before task 2.14's guard
		// flip this subtest passed for the WRONG reason (a StateTicketInput
		// setup made the OLD guard no-op, leaving state coincidentally
		// unchanged). Requiring len(runs)==1 makes that coincidence impossible:
		// it can only hold when the fallthrough genuinely executed.
		if len(nm.runs) != 1 {
			t.Fatalf("the no-resume fallthrough must reconcile+store the run list (len 1), got %d — a guard no-op would leave it nil", len(nm.runs))
		}
	})

	t.Run("detection error falls back to the menu", func(t *testing.T) {
		m := New(Deps{Dir: t.TempDir(), Config: validationConfig()})
		// HU-018 landing (task 2.14): guard keys off StateMainMenu.
		m.state = StateMainMenu
		next, _ := m.Update(resumeDetectMsg{err: errStub})
		if next.(Model).State() != StateMainMenu {
			t.Fatalf("a detection error should fall back to StateMainMenu, got %v", next.(Model).State())
		}
		// HONEST LIMITATION (task 2.15): the error path early-returns BEFORE
		// touching any field, so its outcome (state unchanged = StateMainMenu)
		// is identical whether the guard admitted the message and then rejected
		// it on err, OR the guard no-op'd first. This subtest therefore cannot
		// on its own distinguish a correct guard from a stale one; it only pins
		// the best-effort "a detection error never blocks startup" contract. The
		// guard-correctness of the StateMainMenu landing is proven by the sibling
		// resumable / nothing-resumable subtests (whose len(runs) assertion fails
		// loudly under a stale guard) and by TestOnResumeDetect_GuardUsesMainMenu
		// (task 2.13).
	})
}

// --- 4.5/4.6: resync stale conflict records --------------------------------

// TestOnResumeDetect_ResyncsStaleConflictRecord is tasks 4.5/4.6 (RED): a
// record whose Phase is a cherry-pick phase but whose repo no longer shows an
// in-progress cherry-pick is reconciled via Save (marked aborted) and NOT
// offered — the flow proceeds normally.
func TestOnResumeDetect_ResyncsStaleConflictRecord(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	seededAt := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	rec := runs.Record{
		RunID: "stale", Ticket: "PROJ-1", Target: "UAT",
		Commits: []string{"sha-A"}, PickTotal: 2, Phase: "git-conflict",
		CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	seedRunAt(t, writer, rec)

	m := New(Deps{Dir: dir, Config: validationConfig(), Runs: writer, Now: func() time.Time { return seededAt.Add(time.Hour) }})
	// HU-018 landing (task 2.14): the guard keys off StateMainMenu, so the
	// stale-conflict resync path is only reached from the menu landing.
	m.state = StateMainMenu

	// Repo is CLEAN (no CHERRY_PICK_HEAD): the conflict record is stale.
	next, _ := m.Update(resumeDetectMsg{state: git.RepoState{Clean: true}, records: []runs.Record{rec}})
	nm := next.(Model)
	if nm.State() != StateMainMenu {
		t.Fatalf("a stale conflict record must not offer the conflict screen; want StateMainMenu (no-resume fallthrough), got %v", nm.State())
	}

	// The record must be reconciled on disk (no longer a cherry-pick phase).
	reloaded, err := writer.Load("stale")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.Phase == "git-conflict" || reloaded.Phase == "cherry-pick" {
		t.Errorf("stale conflict record should be resynced away from a cherry-pick phase, got %q", reloaded.Phase)
	}
}

// --- 4.7/4.8: resumeInto routing -------------------------------------------

// TestResumeInto_RoutesConflictAndPolling is tasks 4.7/4.8 (RED): resumeInto
// routes a conflict-phase run (matching the live in-progress cherry-pick) into
// StateCherryPickConflict rehydrated (ticket/pickIndex/pickTotal/runID), and a
// non-terminal jobId run into StateValidationPolling re-armed
// (jobID/runID/pollCtx/reportCmd). A terminal-jobId run is not resumed.
func TestResumeInto_RoutesConflictAndPolling(t *testing.T) {
	t.Run("conflict-phase run rehydrates the conflict screen", func(t *testing.T) {
		m := New(Deps{Dir: t.TempDir(), Config: validationConfig()})
		// Live in-progress cherry-pick whose HEAD is one of the run's commits.
		m.repoState = git.RepoState{InProgress: true, CurrentSHA: "sha-B", SequencerRemaining: 2}
		rec := runs.Record{
			RunID: "run-1", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX",
			Commits: []string{"sha-A", "sha-B", "sha-C"}, PickTotal: 3, PickIndex: 99, Phase: "git-conflict",
		}
		next, cmd := m.resumeInto(rec)
		nm := next.(Model)
		if nm.State() != StateCherryPickConflict {
			t.Fatalf("a conflict-phase resume should route to StateCherryPickConflict, got %v", nm.State())
		}
		if nm.runID != "run-1" {
			t.Errorf("runID = %q, want run-1", nm.runID)
		}
		if nm.plan.Ticket != "PROJ-1" {
			t.Errorf("ticket not rehydrated: %q", nm.plan.Ticket)
		}
		if nm.pickTotal != 3 {
			t.Errorf("pickTotal = %d, want 3", nm.pickTotal)
		}
		// LIVE recompute (3 - SeqRem 2 + 1 = 2), NOT the stale persisted 99.
		if nm.pickIndex != 2 {
			t.Errorf("pickIndex = %d, want 2 (recomputed live, not the persisted 99)", nm.pickIndex)
		}
		if cmd == nil {
			t.Error("entering the conflict screen should fire the repoState + tick commands")
		}
	})

	t.Run("non-terminal jobId run re-attaches polling", func(t *testing.T) {
		clk := &fakeClock{t: time.Unix(1000, 0)}
		m := New(Deps{Dir: t.TempDir(), Config: validationConfig(), SF: reportSF(t, "JOB1", "UAT_SBX", "InProgress"), Now: clk.now})
		m.repoState = git.RepoState{Clean: true}
		rec := runs.Record{RunID: "run-2", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX", JobID: "JOB1", Status: "InProgress", Phase: "validating"}

		next, cmd := m.resumeInto(rec)
		nm := next.(Model)
		if nm.State() != StateValidationPolling {
			t.Fatalf("a non-terminal jobId resume should route to StateValidationPolling, got %v", nm.State())
		}
		if nm.jobID != "JOB1" || nm.runID != "run-2" {
			t.Errorf("jobID/runID not re-armed: jobID=%q runID=%q", nm.jobID, nm.runID)
		}
		if nm.plan.SandboxAlias != "UAT_SBX" {
			t.Errorf("alias not rehydrated for polling: %q", nm.plan.SandboxAlias)
		}
		if nm.pollCtx == nil {
			t.Error("re-attaching polling should arm a cancelable poll context")
		}
		if !nm.pollInFlight {
			t.Error("re-attaching should fire the first report (pollInFlight)")
		}
		if cmd == nil {
			t.Fatal("re-attaching should fire the report command")
		}
		// The re-armed poll actually reaches the persisted jobId.
		if _, ok := run(t, cmd).(reportDoneMsg); !ok {
			t.Errorf("re-attached command should poll deploy report, got %T", run(t, cmd))
		}
	})

	t.Run("terminal jobId run is not resumed", func(t *testing.T) {
		m := New(Deps{Dir: t.TempDir(), Config: validationConfig()})
		m.state = StateRunHistory
		m.repoState = git.RepoState{Clean: true}
		rec := runs.Record{RunID: "run-3", JobID: "JOB1", Status: "Succeeded", Phase: "done"}
		next, cmd := m.resumeInto(rec)
		if next.(Model).State() != StateRunHistory {
			t.Fatalf("a terminal run must not be resumed (stays put), got %v", next.(Model).State())
		}
		if cmd != nil {
			t.Error("a terminal run resume must fire no command")
		}
	})
}

// TestResumeInto_RehydratesPromotionBranchForPush is the RED test for a
// resume/push interaction bug: resumeInto rehydrates Ticket/Target/Alias/
// SelectedCommits but historically left plan.PromotionBranch empty — it was
// set ONLY in onBranchCreated, which a resume never revisits. A run resumed
// via history (either resume branch) that later reaches StateSucceeded and
// enters push would then run `git push -u origin ""` (an invalid refspec).
// resumeInto must reconstruct PromotionBranch on BOTH resume branches,
// mirroring viewRunHistory's git.RenderBranchName(cfg.BranchFormat, ...) call,
// so a resumed run that succeeds can push a non-empty, correct branch.
func TestResumeInto_RehydratesPromotionBranchForPush(t *testing.T) {
	wantBranch := "deploy/PROJ-1-to-UAT" // config.DefaultBranchFormat rendered

	t.Run("conflict-phase resume rehydrates PromotionBranch", func(t *testing.T) {
		m := New(Deps{Dir: t.TempDir(), Config: validationConfig()})
		m.repoState = git.RepoState{InProgress: true, CurrentSHA: "sha-B", SequencerRemaining: 2}
		rec := runs.Record{
			RunID: "run-1", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX",
			Commits: []string{"sha-A", "sha-B", "sha-C"}, PickTotal: 3, Phase: "git-conflict",
		}
		next, _ := m.resumeInto(rec)
		nm := next.(Model)
		if nm.plan.PromotionBranch != wantBranch {
			t.Fatalf("plan.PromotionBranch = %q, want %q (resumeInto must reconstruct it like viewRunHistory does)", nm.plan.PromotionBranch, wantBranch)
		}
	})

	t.Run("jobId re-attach resume rehydrates PromotionBranch, and a later push uses it", func(t *testing.T) {
		clk := &fakeClock{t: time.Unix(1000, 0)}
		m := New(Deps{Dir: t.TempDir(), Config: validationConfig(), SF: reportSF(t, "JOB1", "UAT_SBX", "InProgress"), Now: clk.now})
		m.repoState = git.RepoState{Clean: true}
		rec := runs.Record{RunID: "run-2", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX", JobID: "JOB1", Status: "InProgress", Phase: "validating"}

		next, _ := m.resumeInto(rec)
		nm := next.(Model)
		if nm.plan.PromotionBranch != wantBranch {
			t.Fatalf("plan.PromotionBranch = %q, want %q (resumeInto must reconstruct it like viewRunHistory does)", nm.plan.PromotionBranch, wantBranch)
		}

		// Simulate the resumed run reaching StateSucceeded (as onReportDone/
		// onValidateDone would after a successful deploy) and entering push
		// preparation.
		nm.state = StateSucceeded
		next2, _ := nm.Update(keyPress("p"))
		nm2 := next2.(Model)
		if nm2.State() != StatePushPreparation {
			t.Fatalf("p on Succeeded should enter StatePushPreparation, got %v", nm2.State())
		}

		// Wire a Git service canned for the EXACT reconstructed branch — proves
		// pushCmd/git.Push receives the correct, non-empty refspec, not "".
		fr := execpkg.NewFakeRunner()
		fr.When("git", []string{"rev-parse", "--show-toplevel"}, execpkg.CommandResult{ExitCode: 0, Stdout: []byte("/repo")})
		fr.When("git", []string{"push", "-u", "origin", wantBranch}, execpkg.CommandResult{ExitCode: 0})
		nm2.deps.Git = git.New(fr)
		nm2.deps.Dir = "/repo"

		next3, cmd := nm2.Update(keyPress("p"))
		nm3 := next3.(Model)
		if nm3.pushPhase != pushPushing {
			t.Fatalf("confirming push should move to pushPushing, got %v", nm3.pushPhase)
		}
		if cmd == nil {
			t.Fatal("confirming push should return the push command")
		}
		pushMsg := cmd()
		pd, ok := pushMsg.(pushDoneMsg)
		if !ok || pd.err != nil {
			t.Fatalf("push should succeed against the reconstructed branch, got %#v", pushMsg)
		}
		if !calledWith(fr, "git", "push", "-u", "origin", wantBranch) {
			t.Fatalf("git.Push should run against the reconstructed branch %q; calls: %v", wantBranch, fr.Calls)
		}
	})
}

// --- Required integration deliverables --------------------------------------

// TestResume_RealInProgressCherryPick_RoutesToConflict is the story's headline
// integration test: a REAL in-progress multi-commit cherry-pick + a matching
// run.json, driven from startup, offers resume and (on Enter) routes to
// StateCherryPickConflict with the correct ticket and pick N of M recomputed
// LIVE from the repo's sequencer.
func TestResume_RealInProgressCherryPick_RoutesToConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local := setupConflictRepo3(t) // 3 commits, conflict on the SECOND pick
	seed, writer := driveToCherryPicking(t, local)
	seed = advance(t, seed, run(t, seed.cherryPickCmd()))
	if seed.State() != StateCherryPickConflict {
		t.Fatalf("expected a conflict, got %v (err=%v)", seed.State(), seed.Err())
	}
	runID := seed.runID

	// Restart: a brand-new model over the same repo + runs dir.
	fresh := New(Deps{Git: git.New(execpkg.NewOSRunner()), Config: pickIndexConfig(), Dir: local, Runs: writer})
	fresh = advance(t, fresh, prereqDoneMsg{checks: []prereq.PrereqCheck{{Name: "git", Status: prereq.StatusOK}}})
	if fresh.State() != StateMainMenu {
		t.Fatalf("prereq should default to StateMainMenu before detection lands, got %v", fresh.State())
	}
	// Startup resume-detection reads repo + disk and offers resume.
	fresh = advance(t, fresh, run(t, fresh.resumeDetectCmd()))
	if fresh.State() != StateRunHistory {
		t.Fatalf("startup detection should offer resume via StateRunHistory, got %v", fresh.State())
	}
	if fresh.runs[fresh.runsCursor].RunID != runID {
		t.Fatalf("the in-progress run should be pre-selected, got %q", fresh.runs[fresh.runsCursor].RunID)
	}

	// Accept the offer.
	fresh = advance(t, fresh, keyPress("enter"))
	if fresh.State() != StateCherryPickConflict {
		t.Fatalf("accepting resume should route to StateCherryPickConflict, got %v", fresh.State())
	}
	if fresh.plan.Ticket != "PROJ-1" {
		t.Errorf("resumed conflict should rehydrate the ticket, got %q", fresh.plan.Ticket)
	}
	if fresh.pickTotal != 3 {
		t.Errorf("pickTotal = %d, want 3", fresh.pickTotal)
	}
	if fresh.pickIndex != 2 {
		t.Errorf("pickIndex = %d, want 2 (3-commit, conflict on the second pick, recomputed live)", fresh.pickIndex)
	}
}

// TestResume_Resync_ExternallyResolved is the required resync integration test:
// the same conflict run.json, but the cherry-pick was aborted EXTERNALLY (no
// CHERRY_PICK_HEAD). Startup resync corrects the record and proceeds to the
// normal flow — never the stale conflict screen.
func TestResume_Resync_ExternallyResolved(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local := setupConflictRepo2(t)
	seed, writer := driveToCherryPicking(t, local)
	seed = advance(t, seed, run(t, seed.cherryPickCmd()))
	if seed.State() != StateCherryPickConflict {
		t.Fatalf("expected a conflict, got %v (err=%v)", seed.State(), seed.Err())
	}
	runID := seed.runID

	// Resolve the cherry-pick OUTSIDE DeployDeck.
	gitRun(t, local, "cherry-pick", "--abort")

	fresh := New(Deps{Git: git.New(execpkg.NewOSRunner()), Config: pickIndexConfig(), Dir: local, Runs: writer})
	fresh = advance(t, fresh, prereqDoneMsg{checks: []prereq.PrereqCheck{{Name: "git", Status: prereq.StatusOK}}})
	fresh = advance(t, fresh, run(t, fresh.resumeDetectCmd()))
	if fresh.State() != StateMainMenu {
		t.Fatalf("an externally-resolved conflict must resync and proceed to the menu (HU-018), got %v", fresh.State())
	}

	reloaded, err := writer.Load(runID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.Phase == "git-conflict" || reloaded.Phase == "cherry-pick" {
		t.Errorf("the record should be resynced away from a cherry-pick phase, got %q", reloaded.Phase)
	}
}

// TestResume_ByJobId_ReattachesPolling is the required jobId re-attach
// integration test: a clean repo + a run.json carrying a non-terminal jobId +
// a FakeRunner canned for `deploy report`. Startup offers resume; accepting
// re-attaches polling into StateValidationPolling against the persisted jobId.
func TestResume_ByJobId_ReattachesPolling(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local := setupCleanRepo(t)
	writer := runs.NewWriter(local)
	seededAt := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	runID := "PROJ-1-to-UAT-20260727120000"
	seedRunAt(t, writer, runs.Record{
		RunID: runID, Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX",
		JobID: "JOB1", Status: "InProgress", Phase: "validating",
		CreatedAt: seededAt, UpdatedAt: seededAt,
	})

	clk := &fakeClock{t: seededAt}
	deps := Deps{
		Git:    git.New(execpkg.NewOSRunner()),
		SF:     reportSF(t, "JOB1", "UAT_SBX", "InProgress"),
		Config: validationConfig(),
		Dir:    local,
		Runs:   writer,
		Now:    clk.now,
	}
	m := New(deps)
	m = advance(t, m, prereqDoneMsg{checks: []prereq.PrereqCheck{{Name: "git", Status: prereq.StatusOK}}})
	m = advance(t, m, run(t, m.resumeDetectCmd()))
	if m.State() != StateRunHistory {
		t.Fatalf("a persisted non-terminal jobId should offer resume, got %v", m.State())
	}
	if m.runs[m.runsCursor].RunID != runID {
		t.Fatalf("the jobId run should be pre-selected, got %q", m.runs[m.runsCursor].RunID)
	}

	m = advance(t, m, keyPress("enter"))
	if m.State() != StateValidationPolling {
		t.Fatalf("accepting a jobId resume should re-attach polling, got %v", m.State())
	}
	if m.jobID != "JOB1" || m.runID != runID {
		t.Errorf("re-attach should re-arm jobID/runID, got jobID=%q runID=%q", m.jobID, m.runID)
	}

	// Prove the re-armed poll actually reaches the persisted jobId.
	msg := run(t, m.reportCmd())
	rmsg, ok := msg.(reportDoneMsg)
	if !ok {
		t.Fatalf("re-attach should poll deploy report, got %T", msg)
	}
	if rmsg.err != nil {
		t.Fatalf("report poll errored: %v", rmsg.err)
	}
	if rmsg.report.Status != "InProgress" {
		t.Errorf("re-attached poll should read the persisted job status, got %q", rmsg.report.Status)
	}
}
