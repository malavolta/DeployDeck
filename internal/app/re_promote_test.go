package app

import (
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// rePromoteEnvConfig mirrors internal/git/source_suggestion_test.go's
// envConfig(): three pipeline environments (INT -> UAT -> main), so
// NextEnvironmentBranch has a real forward mirror to resolve against.
func rePromoteEnvConfig() config.Config {
	return config.Config{
		Branches: map[string]string{
			"integration": "INT",
			"uat":         "UAT",
			"production":  "main",
		},
		TicketPatterns: []string{"PROJ-[0-9]+"},
		BranchFormat:   config.DefaultBranchFormat,
	}
}

// --- 4.1: startRePromoteInto seeds synchronous fields and fires the remap --

// TestStartRePromoteInto_EligibleRow_SeedsSynchronousFieldsAndFiresRemap is
// task 4.1 (RED): starting a re-promotion from an eligible prior run seeds
// m.ticket (NOT m.plan.Ticket — confirmSelection reads the model field
// directly), m.sourceRunID, and the NextEnvironmentBranch-derived m.prelim,
// lands on the interim StateCommitDiscovery (reused, no new state), and
// fires a non-nil command (the async patch-id remap).
func TestStartRePromoteInto_EligibleRow_SeedsSynchronousFieldsAndFiresRemap(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: rePromoteEnvConfig()})
	rec := runs.Record{
		RunID: "prior", Ticket: "PROJ-1", Target: "INT", Status: "Succeeded",
		Commits: []string{"shaA", "shaB"},
	}

	next, cmd := m.startRePromoteInto(rec)
	nm := next.(Model)

	if nm.ticket != "PROJ-1" {
		t.Errorf("ticket = %q, want PROJ-1 (must seed m.ticket, not m.plan.Ticket)", nm.ticket)
	}
	if nm.sourceRunID != "prior" {
		t.Errorf("sourceRunID = %q, want prior", nm.sourceRunID)
	}
	if nm.prelim != "UAT" {
		t.Errorf("prelim = %q, want UAT (the configured environment after INT)", nm.prelim)
	}
	if nm.State() != StateCommitDiscovery {
		t.Fatalf("state = %v, want the interim StateCommitDiscovery", nm.State())
	}
	if cmd == nil {
		t.Fatal("startRePromoteInto should fire the patch-id remap command")
	}
}

// --- 4.2: onRePromoteSeeded applies Matched/Unmatched, lands on selection --

// TestOnRePromoteSeeded_AppliesMatchedAndUnmatched_LandsOnCommitSelection is
// task 4.2 (RED): once the remap result lands, the Matched set becomes
// m.discovery.OrderedCommits (so confirmSelection's
// git.IsContiguousSelection check runs against the REAL reused set, not an
// empty one) and drives pre-checked, still-toggleable selection items;
// Unmatched is held as m.rePromoteMissing for the view's explicit warning.
func TestOnRePromoteSeeded_AppliesMatchedAndUnmatched_LandsOnCommitSelection(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: rePromoteEnvConfig()})
	rec := runs.Record{
		RunID: "prior", Ticket: "PROJ-1", Target: "INT", Status: "Succeeded",
		Commits: []string{"shaA", "shaB", "shaC"},
	}
	seededNext, _ := m.startRePromoteInto(rec)
	seeded := seededNext.(Model)

	matched := []git.DiscoveredCommit{
		discovered("newA", "newA", "PROJ-1: add A", false),
		discovered("newB", "newB", "PROJ-1: add B", false),
	}
	next, _ := seeded.onRePromoteSeeded(rePromoteSeededMsg{Matched: matched, Unmatched: []string{"shaC"}})
	nm := next.(Model)

	if nm.State() != StateCommitSelection {
		t.Fatalf("state = %v, want StateCommitSelection", nm.State())
	}
	if len(nm.discovery.OrderedCommits) != 2 || nm.discovery.OrderedCommits[0].SHA != "newA" || nm.discovery.OrderedCommits[1].SHA != "newB" {
		t.Fatalf("discovery.OrderedCommits = %+v, want the Matched set (confirmSelection's contiguity check needs the real set)", nm.discovery.OrderedCommits)
	}
	if len(nm.items) != 2 {
		t.Fatalf("items = %d rows, want 2 pre-loaded selection rows", len(nm.items))
	}
	for _, it := range nm.items {
		if !it.Selected {
			t.Errorf("matched commit %s should start pre-checked, got Selected=false", it.SHA)
		}
		if it.Disabled {
			t.Errorf("matched commit %s should remain editable, got Disabled=true", it.SHA)
		}
	}
	// Prove the pre-loaded selection is genuinely editable via the real
	// ToggleSelection path (re-promotion spec: "remain editable").
	toggled := git.ToggleSelection(nm.items, 0)
	if toggled[0].Selected {
		t.Error("ToggleSelection should still flip a pre-checked re-promote item")
	}
	if len(nm.rePromoteMissing) != 1 || nm.rePromoteMissing[0] != "shaC" {
		t.Fatalf("rePromoteMissing = %v, want [shaC]", nm.rePromoteMissing)
	}
}

// --- 4.3/4.4: keyRunHistory's r eligibility gate ----------------------------

// TestKeyRunHistory_R_OnSucceededPartialRow_StartsRePromote is task 4.3
// (RED): r on a SucceededPartial row starts re-promotion, mirroring 4.1's
// seeding.
func TestKeyRunHistory_R_OnSucceededPartialRow_StartsRePromote(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: rePromoteEnvConfig()})
	m.state = StateRunHistory
	m.runs = []runs.Record{
		{RunID: "prior", Ticket: "PROJ-1", Target: "INT", Status: "SucceededPartial", Commits: []string{"shaA"}},
	}
	m.runsCursor = 0

	next, cmd := m.Update(keyPress("r"))
	nm := next.(Model)
	if nm.State() != StateCommitDiscovery {
		t.Fatalf("r on a SucceededPartial row should start re-promote (interim StateCommitDiscovery), got %v", nm.State())
	}
	if nm.ticket != "PROJ-1" || nm.sourceRunID != "prior" {
		t.Errorf("re-promote seed not applied: ticket=%q sourceRunID=%q", nm.ticket, nm.sourceRunID)
	}
	if cmd == nil {
		t.Error("r should fire the remap command")
	}
}

// TestKeyRunHistory_R_OnIneligibleRow_IsNoOp is task 4.4 (RED): r on a
// Failed/Canceled/Aborted row is a strict no-op — the history view stays put
// and no commits are pre-loaded (re-promotion spec: "Failed prior run
// offers no pre-load").
func TestKeyRunHistory_R_OnIneligibleRow_IsNoOp(t *testing.T) {
	for _, status := range []string{"Failed", "Canceled", "Aborted"} {
		t.Run(status, func(t *testing.T) {
			m := New(Deps{Dir: "/repo", Config: rePromoteEnvConfig()})
			m.state = StateRunHistory
			m.runs = []runs.Record{
				{RunID: "prior", Ticket: "PROJ-1", Target: "INT", Status: status, Commits: []string{"shaA"}},
			}
			m.runsCursor = 0

			next, cmd := m.Update(keyPress("r"))
			nm := next.(Model)
			if nm.State() != StateRunHistory {
				t.Fatalf("r on a %s row should be a no-op, got state %v", status, nm.State())
			}
			if nm.ticket != "" || nm.sourceRunID != "" {
				t.Errorf("no re-promote seed should apply on a %s row, got ticket=%q sourceRunID=%q", status, nm.ticket, nm.sourceRunID)
			}
			if cmd != nil {
				t.Errorf("r on a %s row must fire no command", status)
			}
		})
	}
}

// TestKeyRunHistory_Enter_StillResumesIndependentlyOfR is task 4.5 (RED):
// Enter on a resumable row still calls resumeInto exactly as before,
// unaffected by the new r branch (run-history spec: "Enter still resumes
// independently of r").
func TestKeyRunHistory_Enter_StillResumesIndependentlyOfR(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	m := New(Deps{Dir: "/repo", Config: validationConfig(), SF: reportSF(t, "JOB1", "UAT_SBX", "InProgress"), Now: clk.now})
	m.state = StateRunHistory
	m.runs = []runs.Record{
		{RunID: "run-2", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX", JobID: "JOB1", Status: "InProgress", Phase: "validating"},
	}
	m.runsCursor = 0

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StateValidationPolling {
		t.Fatalf("Enter should still resume a non-terminal jobId run unaffected by r, got %v", nm.State())
	}
	if cmd == nil {
		t.Error("resuming should fire a command")
	}
}

// --- 4.6: no next environment degrades to manual, ticket pre-filled --------

// TestStartRePromoteInto_NoNextEnvironment_DegradesToManualFlowWithTicketPrefilled
// is task 4.6, UPDATED by the review remediation for Finding 1 (HIGH): a
// prior run whose Target is the LAST pipeline stage (NextEnvironmentBranch
// returns ok=false) still pre-fills the ticket and still proceeds through the
// interim StateCommitDiscovery, but now degrades to the NORMAL manual flow —
// mirroring keyTicket's Enter branch exactly — rather than firing the remap
// over an ill-defined empty range (commit-discovery spec: "No next
// environment falls back to manual target choice"; design decision #4: "No
// next env -> degrade to manual flow, ticket pre-filled, notice shown").
// sourceRunID is left empty: this degrade has no well-defined provenance
// link to the prior run (see
// TestStartRePromoteInto_NoNextEnvironment_DegradesToNormalDiscovery in
// re_promote_e2e_test.go for the real-git proof that the returned command
// never fires the remap and never reaches StateError). prelim mirrors
// exactly what a normal manual ticket entry would compute
// (preliminaryTarget: the first configured destination, sorted by
// environment key) — NOT forced empty, since this is genuinely the manual
// flow, not a special re-promote case.
func TestStartRePromoteInto_NoNextEnvironment_DegradesToManualFlowWithTicketPrefilled(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: rePromoteEnvConfig()})
	rec := runs.Record{
		RunID: "prior", Ticket: "PROJ-1", Target: "main", Status: "Succeeded",
		Commits: []string{"shaA"},
	}

	next, cmd := m.startRePromoteInto(rec)
	nm := next.(Model)

	if nm.ticket != "PROJ-1" {
		t.Errorf("ticket = %q, want PROJ-1 pre-filled even with no next environment", nm.ticket)
	}
	if nm.sourceRunID != "" {
		t.Errorf("sourceRunID = %q, want empty (no next-env degrade has no well-defined provenance link)", nm.sourceRunID)
	}
	if nm.prelim != preliminaryTarget(rePromoteEnvConfig()) {
		t.Errorf("prelim = %q, want %q (preliminaryTarget, exactly what keyTicket's Enter branch computes for the manual flow)", nm.prelim, preliminaryTarget(rePromoteEnvConfig()))
	}
	if nm.State() != StateCommitDiscovery {
		t.Fatalf("state = %v, want the flow to still proceed through the interim discovery state", nm.State())
	}
	if cmd == nil {
		t.Error("the manual discovery command should still fire even with no next-environment default")
	}
	if nm.notice == "" {
		t.Error("a notice should explain the no-next-environment degrade (design decision #4: \"notice shown\")")
	}
}

// --- Findings 2/3 (review remediation): keyTicket's Enter resets stale
// re-promote state -----------------------------------------------------

// TestKeyTicket_Enter_ResetsStaleRePromoteState is the review remediation for
// Finding 2 (MED) and Finding 3 (LOW): m.sourceRunID and m.rePromoteMissing
// are seeded by startRePromoteInto but never cleared, so an ABANDONED
// re-promotion (r on an eligible row -> esc back to StateTicketInput -> a
// DIFFERENT ticket -> Enter) would otherwise mislink the unrelated normal run
// to the abandoned prior run's RunID (onBranchCreated persists SourceRunID
// verbatim from m.sourceRunID) and leak the abandoned missing-commit warning
// onto the new selection screen. keyTicket's Enter is the single choke point
// every non-re-promote run passes through, so it must reset both fields.
func TestKeyTicket_Enter_ResetsStaleRePromoteState(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: rePromoteEnvConfig()})
	m.state = StateTicketInput
	m.sourceRunID = "prior-run-xyz"
	m.rePromoteMissing = []string{"deadbeef"}
	m.ticket = "PROJ-2"

	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)

	if nm.sourceRunID != "" {
		t.Errorf("sourceRunID = %q, want empty after a normal-flow ticket entry (an abandoned re-promote link must not bleed onto an unrelated run)", nm.sourceRunID)
	}
	if nm.rePromoteMissing != nil {
		t.Errorf("rePromoteMissing = %v, want nil after a normal-flow ticket entry (an abandoned warning must not bleed onto the new selection screen)", nm.rePromoteMissing)
	}
	if cmd == nil {
		t.Error("normal ticket entry should still fire the discovery command")
	}
}

// --- 5.1/5.2: onBranchCreated threads SourceRunID onto the new run.Record --

// TestOnBranchCreated_RePromoteRun_PersistsSourceRunID is task 5.1 (RED): a
// re-promotion run (m.sourceRunID set by startRePromoteInto) persists that
// link on the NEW run's record at creation time (re-promotion spec:
// "Completed re-promotion records SourceRunID"; run-persistence spec:
// "SourceRunID persisted at creation for a re-promotion").
func TestOnBranchCreated_RePromoteRun_PersistsSourceRunID(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	deps := Deps{Dir: dir, Config: rePromoteEnvConfig(), Runs: writer, Now: func() time.Time { return now }}
	m := New(deps)
	m.state = StateBranchCreation
	m.branchName = "deploy/PROJ-1-to-UAT"
	m.sourceRunID = "prior-run-id"
	m.plan = git.DeploymentPlan{
		Ticket:       "PROJ-1",
		TargetBranch: "UAT",
		SandboxAlias: "UAT_SBX",
		SelectedCommits: []git.DiscoveredCommit{
			discovered("newA", "newA", "PROJ-1: add A", false),
		},
	}

	next, _ := m.Update(branchCreatedMsg{})
	nm := next.(Model)
	if nm.State() != StateCherryPicking {
		t.Fatalf("expected CherryPicking, got %v (err=%v)", nm.State(), nm.Err())
	}

	gotRec, err := writer.Load(nm.runID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if gotRec.SourceRunID != "prior-run-id" {
		t.Errorf("SourceRunID = %q, want prior-run-id", gotRec.SourceRunID)
	}
}

// TestOnBranchCreated_NormalRun_PersistsEmptySourceRunID is task 5.2 (RED):
// a NORMAL (non-re-promotion) run leaves m.sourceRunID empty, and the
// persisted record's SourceRunID stays empty too — the existing flow is
// unaffected (regression guard).
func TestOnBranchCreated_NormalRun_PersistsEmptySourceRunID(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	deps := Deps{Dir: dir, Config: rePromoteEnvConfig(), Runs: writer, Now: func() time.Time { return now }}
	m := New(deps)
	m.state = StateBranchCreation
	m.branchName = "deploy/PROJ-1-to-UAT"
	m.plan = git.DeploymentPlan{
		Ticket:       "PROJ-1",
		TargetBranch: "UAT",
		SandboxAlias: "UAT_SBX",
		SelectedCommits: []git.DiscoveredCommit{
			discovered("newA", "newA", "PROJ-1: add A", false),
		},
	}

	next, _ := m.Update(branchCreatedMsg{})
	nm := next.(Model)

	gotRec, err := writer.Load(nm.runID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if gotRec.SourceRunID != "" {
		t.Errorf("SourceRunID = %q, want empty for a normal (non-re-promote) run", gotRec.SourceRunID)
	}
}

// --- Finding 4a (review remediation): a zero-commit run has nothing to
// remap, so it is not re-promote-eligible -------------------------------

// TestIsRePromoteEligible_ZeroCommits_IsIneligible is the review remediation
// for Finding 4a (LOW): a Succeeded/SucceededPartial run with ZERO Commits
// has nothing to remap, so it must be treated as ineligible — pressing r on
// such a row is a strict no-op, exactly like an ineligible-status row.
func TestIsRePromoteEligible_ZeroCommits_IsIneligible(t *testing.T) {
	cases := []struct {
		name string
		rec  runs.Record
		want bool
	}{
		{"succeeded with commits", runs.Record{Status: "Succeeded", Commits: []string{"shaA"}}, true},
		{"succeededPartial with commits", runs.Record{Status: "SucceededPartial", Commits: []string{"shaA"}}, true},
		{"succeeded with zero commits", runs.Record{Status: "Succeeded", Commits: nil}, false},
		{"succeededPartial with zero commits", runs.Record{Status: "SucceededPartial", Commits: []string{}}, false},
		{"failed with commits", runs.Record{Status: "Failed", Commits: []string{"shaA"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRePromoteEligible(tc.rec); got != tc.want {
				t.Errorf("isRePromoteEligible(%+v) = %v, want %v", tc.rec, got, tc.want)
			}
		})
	}
}

// --- Finding 4b (review remediation): onRePromoteSeeded must not leak stale
// discovery sibling fields -------------------------------------------------

// TestOnRePromoteSeeded_ReplacesDiscoveryFresh_NoStaleSiblingFields is the
// review remediation for Finding 4b (LOW): onRePromoteSeeded must assign a
// FRESH git.DiscoverResult carrying only OrderedCommits, rather than mutating
// a single field on whatever m.discovery was left holding from a prior
// in-session discovery (e.g. stale Alternatives/CandidateBranches from an
// earlier no-results search) — stale sibling fields viewSelection may still
// render.
func TestOnRePromoteSeeded_ReplacesDiscoveryFresh_NoStaleSiblingFields(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: rePromoteEnvConfig()})
	m.discovery = git.DiscoverResult{
		Ticket:            "OLD-1",
		Alternatives:      []string{"stale-alt-branch"},
		CandidateBranches: []git.Branch{{Name: "stale/branch"}},
	}
	matched := []git.DiscoveredCommit{discovered("newA", "newA", "PROJ-1: add A", false)}

	next, _ := m.onRePromoteSeeded(rePromoteSeededMsg{Matched: matched})
	nm := next.(Model)

	if nm.discovery.Alternatives != nil {
		t.Errorf("discovery.Alternatives = %v, want nil (fresh discovery, no stale sibling fields)", nm.discovery.Alternatives)
	}
	if nm.discovery.CandidateBranches != nil {
		t.Errorf("discovery.CandidateBranches = %v, want nil (fresh discovery, no stale sibling fields)", nm.discovery.CandidateBranches)
	}
	if nm.discovery.Ticket != "" {
		t.Errorf("discovery.Ticket = %q, want empty (fresh discovery, no stale sibling fields)", nm.discovery.Ticket)
	}
	if len(nm.discovery.OrderedCommits) != 1 || nm.discovery.OrderedCommits[0].SHA != "newA" {
		t.Fatalf("discovery.OrderedCommits = %+v, want the Matched set", nm.discovery.OrderedCommits)
	}
}
