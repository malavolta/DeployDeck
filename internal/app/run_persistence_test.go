package app

import (
	"testing"
	"time"

	"deploydeck/internal/git"
	"deploydeck/internal/runs"
)

// --- 3.1/3.2: derivePickIndex (pure, design's locked formula) --------------

// TestDerivePickIndex is task 3.1 (RED): pins the two disambiguating cases
// from design.md's locked formula (pickTotal - SequencerRemaining + 1, the
// sequencer INCLUDES the current conflicting commit) plus its clamp edges.
func TestDerivePickIndex(t *testing.T) {
	tests := []struct {
		name      string
		pickTotal int
		state     git.RepoState
		want      int
	}{
		{
			name:      "2-commit selection, conflict on the first pick",
			pickTotal: 2,
			state:     git.RepoState{InProgress: true, SequencerRemaining: 2},
			want:      1,
		},
		{
			name:      "3-commit selection, conflict on the second pick",
			pickTotal: 3,
			state:     git.RepoState{InProgress: true, SequencerRemaining: 2},
			want:      2,
		},
		{
			name:      "conflict on the last pick",
			pickTotal: 2,
			state:     git.RepoState{InProgress: true, SequencerRemaining: 1},
			want:      2,
		},
		{
			name:      "conflict on the last pick, git drops the sequencer file entirely",
			pickTotal: 2,
			state:     git.RepoState{InProgress: true, SequencerRemaining: 0},
			want:      2, // clamp: 2-0+1=3 > pickTotal -> clamped to pickTotal
		},
		{
			name:      "single-commit pick, no sequencer file",
			pickTotal: 1,
			state:     git.RepoState{InProgress: true, SequencerRemaining: 0},
			want:      1, // clamp: 1-0+1=2 > pickTotal -> clamped to 1
		},
		{
			name:      "sequence complete (not in progress)",
			pickTotal: 3,
			state:     git.RepoState{InProgress: false},
			want:      3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := derivePickIndex(tt.pickTotal, tt.state)
			if got != tt.want {
				t.Errorf("derivePickIndex(%d, %+v) = %d, want %d", tt.pickTotal, tt.state, got, tt.want)
			}
		})
	}
}

// --- 3.3/3.4: onBranchCreated persists the initial run record --------------

// TestOnBranchCreated_PersistsInitialRunRecord is task 3.3 (RED): branch
// creation must persist run.json IMMEDIATELY — before any pick runs — with
// Ticket/Target/Alias/PickTotal/Commits/Phase="cherry-pick" (design.md: "Run
// creation point... Resume needs Ticket + PickTotal persisted DURING
// cherry-pick, before any jobId exists").
func TestOnBranchCreated_PersistsInitialRunRecord(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	deps := Deps{Dir: dir, Config: validationConfig(), Runs: writer, Now: func() time.Time { return now }}
	m := New(deps)
	m.state = StateBranchCreation
	m.branchName = "deploy/PROJ-1-to-UAT"
	m.plan = git.DeploymentPlan{
		Ticket:       "PROJ-1",
		TargetBranch: "UAT",
		SandboxAlias: "UAT_SBX",
		SelectedCommits: []git.DiscoveredCommit{
			discovered("aaa111", "aaa111", "PROJ-1: a", false),
			discovered("bbb222", "bbb222", "PROJ-1: b", false),
		},
	}

	next, _ := m.Update(branchCreatedMsg{})
	nm := next.(Model)
	if nm.State() != StateCherryPicking {
		t.Fatalf("expected CherryPicking, got %v (err=%v)", nm.State(), nm.Err())
	}
	if nm.runID == "" {
		t.Fatal("branch creation should generate a runID")
	}

	// The record must already be on disk (before any pick runs).
	rec, err := writer.Load(nm.runID)
	if err != nil {
		t.Fatalf("run.json should already be persisted at branch creation: %v", err)
	}
	if rec.Ticket != "PROJ-1" || rec.Target != "UAT" || rec.Alias != "UAT_SBX" {
		t.Errorf("identifying fields not persisted, got %+v", rec)
	}
	if rec.PickTotal != 2 {
		t.Errorf("PickTotal = %d, want 2", rec.PickTotal)
	}
	if len(rec.Commits) != 2 || rec.Commits[0] != "aaa111" || rec.Commits[1] != "bbb222" {
		t.Errorf("Commits = %v, want [aaa111 bbb222]", rec.Commits)
	}
	if rec.Phase != "cherry-pick" {
		t.Errorf("Phase = %q, want cherry-pick", rec.Phase)
	}
	if !rec.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", rec.CreatedAt, now)
	}
}

// TestOnBranchCreated_NilRunsWriterIsANoOp proves a nil Deps.Runs never
// blocks the flow (persistence stays best-effort/optional, matching every
// other Runs call site in this package).
func TestOnBranchCreated_NilRunsWriterIsANoOp(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: validationConfig()})
	m.state = StateBranchCreation
	m.branchName = "deploy/PROJ-1-to-UAT"
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}

	next, cmd := m.Update(branchCreatedMsg{})
	nm := next.(Model)
	if nm.State() != StateCherryPicking {
		t.Fatalf("expected CherryPicking, got %v", nm.State())
	}
	if cmd == nil {
		t.Error("entering CherryPicking should still fire the cherry-pick command")
	}
}

// --- 3.9/3.10: onVerifyDone/onAborted update the persisted Phase -----------

// seedRunAt seeds a minimal run record via Save, failing the test on error.
func seedRunAt(t *testing.T, w *runs.Writer, rec runs.Record) {
	t.Helper()
	if err := w.Save(rec); err != nil {
		t.Fatalf("seeding run %s: %v", rec.RunID, err)
	}
}

// TestOnVerifyDone_PersistsPhaseDone is task 3.9 (RED): a successful
// post-pick verification updates the persisted Phase to "done".
func TestOnVerifyDone_PersistsPhaseDone(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	seededAt := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	runID := "PROJ-1-to-UAT-20260727120000"
	seedRunAt(t, writer, runs.Record{
		RunID: runID, Ticket: "PROJ-1", Target: "UAT",
		PickTotal: 2, Phase: "cherry-pick", CreatedAt: seededAt, UpdatedAt: seededAt,
	})

	deps := Deps{Dir: dir, Config: validationConfig(), Runs: writer, Now: func() time.Time { return seededAt.Add(time.Minute) }}
	m := New(deps)
	m.state = StateCherryPicking
	m.runID = runID
	m.repoState = git.RepoState{Clean: true}

	next, _ := m.Update(verifyDoneMsg{verification: git.PickVerification{}})
	nm := next.(Model)
	if nm.State() != StatePickVerification {
		t.Fatalf("expected PickVerification, got %v", nm.State())
	}

	rec, err := writer.Load(runID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.Phase != "done" {
		t.Errorf("Phase = %q, want done", rec.Phase)
	}
	// Fields set at branch creation must be PRESERVED (merge, not replace).
	if rec.Ticket != "PROJ-1" || rec.PickTotal != 2 {
		t.Errorf("expected the seeded fields preserved, got %+v", rec)
	}
	if !rec.CreatedAt.Equal(seededAt) {
		t.Errorf("CreatedAt should be preserved (not reset), got %v, want %v", rec.CreatedAt, seededAt)
	}
}

// TestOnAborted_PersistsPhaseAborted is task 3.9 (RED): a confirmed abort
// updates the persisted Phase to "aborted".
func TestOnAborted_PersistsPhaseAborted(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	seededAt := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	runID := "PROJ-1-to-UAT-20260727120000"
	seedRunAt(t, writer, runs.Record{
		RunID: runID, Ticket: "PROJ-1", Target: "UAT",
		PickTotal: 2, Phase: "git-conflict", CreatedAt: seededAt, UpdatedAt: seededAt,
	})

	deps := Deps{Dir: dir, Config: validationConfig(), Runs: writer, Now: func() time.Time { return seededAt.Add(time.Minute) }}
	m := New(deps)
	m.state = StateCherryPickConflict
	m.runID = runID

	next, _ := m.Update(abortedMsg{})
	nm := next.(Model)
	if nm.State() != StateAborted {
		t.Fatalf("expected Aborted, got %v", nm.State())
	}

	rec, err := writer.Load(runID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.Phase != "aborted" {
		t.Errorf("Phase = %q, want aborted", rec.Phase)
	}
}

// TestOnPickDone_ConflictBranch_PersistsPickProgress is task 3.6 (unit-level
// coverage, complementing the real-sequencer 3.5 test): the conflict branch
// persists PickIndex/PickTotal/CurrentCommit/Phase via the derived index, from
// a fed-in RepoState (no real git needed to prove the wiring).
func TestOnPickDone_ConflictBranch_PersistsPickProgress(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	seededAt := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	runID := "PROJ-1-to-UAT-20260727120000"
	seedRunAt(t, writer, runs.Record{
		RunID: runID, Ticket: "PROJ-1", Target: "UAT",
		Commits: []string{"aaa111", "bbb222"}, PickTotal: 2, Phase: "cherry-pick",
		CreatedAt: seededAt, UpdatedAt: seededAt,
	})

	deps := Deps{Dir: dir, Config: validationConfig(), Runs: writer, Now: func() time.Time { return seededAt.Add(time.Minute) }}
	m := New(deps)
	m.state = StateCherryPicking
	m.runID = runID
	m.plan = git.DeploymentPlan{
		Ticket: "PROJ-1", TargetBranch: "UAT",
		SelectedCommits: []git.DiscoveredCommit{
			discovered("aaa111", "aaa111", "a", false),
			discovered("bbb222", "bbb222", "b", false),
		},
	}

	repoState := git.RepoState{InProgress: true, SequencerRemaining: 2, CurrentSHA: "aaa111"}
	next, _ := m.Update(pickDoneMsg{outcome: git.PickOutcome{State: repoState}})
	nm := next.(Model)
	if nm.State() != StateCherryPickConflict {
		t.Fatalf("expected CherryPickConflict, got %v", nm.State())
	}

	rec, err := writer.Load(runID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.PickIndex != 1 {
		t.Errorf("PickIndex = %d, want 1", rec.PickIndex)
	}
	if rec.PickTotal != 2 {
		t.Errorf("PickTotal = %d, want 2", rec.PickTotal)
	}
	if rec.CurrentCommit != "aaa111" {
		t.Errorf("CurrentCommit = %q, want aaa111", rec.CurrentCommit)
	}
	if rec.Phase != "git-conflict" {
		t.Errorf("Phase = %q, want git-conflict", rec.Phase)
	}
}

// --- commitSHAs (pure) -------------------------------------------------

// TestCommitSHAs proves the pure extraction of ordered SHAs from selected
// commits, and that an empty selection yields nil (not a non-nil empty
// slice, matching selectedCommits' convention).
func TestCommitSHAs(t *testing.T) {
	commits := []git.DiscoveredCommit{
		discovered("aaa111", "aaa111", "a", false),
		discovered("bbb222", "bbb222", "b", false),
	}
	got := commitSHAs(commits)
	if len(got) != 2 || got[0] != "aaa111" || got[1] != "bbb222" {
		t.Fatalf("commitSHAs = %v, want [aaa111 bbb222]", got)
	}

	if got := commitSHAs(nil); got != nil {
		t.Fatalf("commitSHAs(nil) = %v, want nil", got)
	}
}
