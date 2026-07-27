package app

import (
	"strings"
	"testing"
	"time"

	execpkg "deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
	"deploydeck/internal/runs"
)

// TestE2E_RunHistory_SeedsOfferDeclineResume is the consolidated HU-013 CI-safe
// E2E (task 7.1, harness fs+temp+sf-fake, NO org): a real clean temp git repo +
// a temp .deploydeck/runs/ seeded with every documented variant — a terminal
// (done, jobId) run, a stale git-conflict record the clean repo no longer
// matches (resync case), and a non-terminal jobId run (resumable) — driven from
// startup through offer, resync, list, decline, and resume-by-jobId.
func TestE2E_RunHistory_SeedsOfferDeclineResume(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local := setupCleanRepo(t)
	writer := runs.NewWriter(local)
	base := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)

	// A terminal, browsable-only run (jobId already Succeeded).
	seedRunAt(t, writer, runs.Record{
		RunID: "done", Ticket: "PROJ-2", Target: "UAT", Alias: "UAT_SBX",
		JobID: "0AfDONE", Status: "Succeeded", Phase: "done",
		CreatedAt: base, UpdatedAt: base,
	})
	// A stale git-conflict record: the repo is now clean (resolved/aborted
	// externally), so startup must resync it, never offer the conflict screen.
	seedRunAt(t, writer, runs.Record{
		RunID: "stale", Ticket: "PROJ-3", Target: "UAT",
		Commits: []string{"deadbeef"}, PickTotal: 2, Phase: "git-conflict",
		CreatedAt: base.Add(time.Hour), UpdatedAt: base.Add(time.Hour),
	})
	// A non-terminal jobId run: the newest resumable run.
	seedRunAt(t, writer, runs.Record{
		RunID: "job", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX",
		JobID: "JOB1", Status: "InProgress", Phase: "validating",
		CreatedAt: base.Add(2 * time.Hour), UpdatedAt: base.Add(2 * time.Hour),
	})

	clk := &fakeClock{t: base.Add(3 * time.Hour)}
	deps := Deps{
		Git:    git.New(execpkg.NewOSRunner()),
		SF:     reportSF(t, "JOB1", "UAT_SBX", "InProgress"),
		Config: validationConfig(),
		Dir:    local,
		Runs:   writer,
		Now:    clk.now,
	}

	// Startup: prereqs pass, then resume-detection lands.
	m := New(deps)
	m = advance(t, m, prereqDoneMsg{checks: []prereq.PrereqCheck{{Name: "git", Status: prereq.StatusOK}}})
	m = advance(t, m, run(t, m.resumeDetectCmd()))

	// Offered via history, pre-selected on the newest resumable (the jobId run).
	if m.State() != StateRunHistory {
		t.Fatalf("startup should offer resume via StateRunHistory, got %v", m.State())
	}
	if m.runs[m.runsCursor].RunID != "job" {
		t.Fatalf("the newest resumable run should be pre-selected, got %q", m.runs[m.runsCursor].RunID)
	}

	// The history lists every seeded run.
	v := m.View()
	for _, ticket := range []string{"PROJ-1", "PROJ-2", "PROJ-3"} {
		if !strings.Contains(v, ticket) {
			t.Errorf("history should list %q:\n%s", ticket, v)
		}
	}

	// The stale conflict record was resynced away from a cherry-pick phase.
	stale, err := writer.Load("stale")
	if err != nil {
		t.Fatalf("Load stale: %v", err)
	}
	if isCherryPickPhase(stale.Phase) {
		t.Errorf("the stale conflict record should be resynced, got phase %q", stale.Phase)
	}

	// Declining proceeds to the normal flow.
	if declined := advance(t, m, keyPress("q")); declined.State() != StateTicketInput {
		t.Fatalf("declining should proceed to StateTicketInput, got %v", declined.State())
	}

	// Accepting re-attaches polling to the persisted jobId.
	resumed := advance(t, m, keyPress("enter"))
	if resumed.State() != StateValidationPolling {
		t.Fatalf("accepting the jobId run should re-attach polling, got %v", resumed.State())
	}
	if resumed.jobID != "JOB1" || resumed.runID != "job" {
		t.Errorf("re-attach should re-arm the persisted job: jobID=%q runID=%q", resumed.jobID, resumed.runID)
	}
}
