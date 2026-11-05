package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// This file covers the "Historial de runs" main-menu entry: StateRunHistory
// is the only screen where x (HU-015 quick deploy) and r (HU-016 re-promote)
// work, but before this entry it was reachable ONLY through startup resume
// detection, which fires only for a NON-terminal run. Once a validation
// finished Succeeded there was no way to open the history at all.

// runHistoryMenuIndex returns the visible main-menu index of the history
// entry, failing the test when the entry is missing.
func runHistoryMenuIndex(t *testing.T) int {
	t.Helper()
	for i, e := range visibleMenuEntries(menuEntries) {
		if e.target == StateRunHistory {
			return i
		}
	}
	t.Fatal("the main menu has no entry targeting StateRunHistory")
	return -1
}

// enterRunHistoryFromMenu moves the menu cursor onto the history entry with
// real key presses, presses Enter, then lands the load command's message —
// the full round trip a user drives.
func enterRunHistoryFromMenu(t *testing.T, m Model) Model {
	t.Helper()
	if m.State() != StateMainMenu {
		t.Fatalf("precondition: expected StateMainMenu, got %v", m.State())
	}
	m.menuCursor = 0
	for i := 0; i < runHistoryMenuIndex(t); i++ {
		m = advance(t, m, keyPress("down"))
	}
	next, cmd := m.Update(keyPress("enter"))
	nm := next.(Model)
	if nm.State() != StateRunHistory {
		t.Fatalf("Historial de runs should route to StateRunHistory, got %v (notice %q)", nm.State(), nm.notice)
	}
	if cmd == nil {
		t.Fatal("entering the history should fire a fresh run-list load")
	}
	return advance(t, nm, cmd())
}

// TestMainMenu_HasRunHistoryEntryLast pins the new entry's label and its
// position: LAST, so the three pre-existing menu indices never shift.
func TestMainMenu_HasRunHistoryEntryLast(t *testing.T) {
	entries := visibleMenuEntries(menuEntries)
	last := entries[len(entries)-1]
	if last.label != "Historial de runs" {
		t.Errorf("last menu entry label = %q, want %q", last.label, "Historial de runs")
	}
	if last.target != StateRunHistory {
		t.Errorf("last menu entry target = %v, want StateRunHistory", last.target)
	}
	if last.description == "" {
		t.Error("the history entry needs a one-line description")
	}
}

// TestRunHistoryMenu_SucceededRunReachesQuickDeploy is the bug repro: a single
// Succeeded RunLocalTests run with a JobID created now is quick-deploy
// eligible, but terminal, so resume detection leaves the user on the menu.
// The new entry must reach the history with that run listed, and x must
// open StateQuickDeploy.
func TestRunHistoryMenu_SucceededRunReachesQuickDeploy(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	clk := &fakeClock{t: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)}
	records := seedQuickDeployRuns(t, writer, runs.Record{
		RunID: "done-run", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX",
		JobID: "0AfDONE", Status: "Succeeded", TestLevel: "RunLocalTests",
		CreatedAt: clk.t,
	})
	if ok, _ := runs.QuickDeployEligible(records[0], clk.now()); !ok {
		t.Fatal("precondition: the seeded run must be quick-deploy eligible")
	}

	m := New(Deps{Dir: dir, Config: quickDeployConfig(false, false), Runs: writer, Now: clk.now})
	m.state = StateMainMenu
	m = advance(t, m, resumeDetectMsg{state: git.RepoState{Clean: true}, records: records})
	if m.State() != StateMainMenu {
		t.Fatalf("a terminal run is not resumable, detection should stay on StateMainMenu, got %v", m.State())
	}

	m = enterRunHistoryFromMenu(t, m)
	if len(m.runs) != 1 || m.runs[0].RunID != "done-run" {
		t.Fatalf("history should list the Succeeded run, got %+v", m.runs)
	}
	if m.runsCursor != 0 {
		t.Errorf("cursor should start on the newest run (0), got %d", m.runsCursor)
	}
	if !strings.Contains(m.View(), "PROJ-1") {
		t.Errorf("history view should list the run:\n%s", m.View())
	}

	m = advance(t, m, keyPress("x"))
	if m.State() != StateQuickDeploy {
		t.Fatalf("x on the eligible run should open StateQuickDeploy, got %v", m.State())
	}
}

// TestRunHistoryMenu_EmptyHistory reaches the history with zero runs, renders
// the empty state, and survives every history key without panicking.
func TestRunHistoryMenu_EmptyHistory(t *testing.T) {
	m := New(Deps{Dir: t.TempDir(), Config: validationConfig(), Runs: runs.NewWriter(t.TempDir())})
	m.state = StateMainMenu

	m = enterRunHistoryFromMenu(t, m)
	if len(m.runs) != 0 {
		t.Fatalf("an empty history should hold no runs, got %d", len(m.runs))
	}
	if v := m.View(); !strings.Contains(v, "sin runs registrados") {
		t.Errorf("empty history should render the empty state:\n%s", v)
	}

	for _, key := range []string{"up", "down", "k", "j", "d", "enter", "r", "x"} {
		m = advance(t, m, keyPress(key))
		if m.State() != StateRunHistory {
			t.Fatalf("%q on an empty history should stay on StateRunHistory, got %v", key, m.State())
		}
		_ = m.View()
	}

	m = advance(t, m, keyPress("q"))
	if m.State() != StateMainMenu {
		t.Errorf("q from the history should return to StateMainMenu, got %v", m.State())
	}
}

// TestRunHistoryMenu_FreshLoad proves the entry reloads from disk: a run
// persisted AFTER startup detection populated m.runs must appear, newest
// first, with the cursor reset to 0 even if it was left elsewhere.
func TestRunHistoryMenu_FreshLoad(t *testing.T) {
	dir := t.TempDir()
	writer := runs.NewWriter(dir)
	base := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	records := seedQuickDeployRuns(t, writer, runs.Record{
		RunID: "old-run", Ticket: "PROJ-1", Target: "UAT", Status: "Succeeded", CreatedAt: base,
	})

	m := New(Deps{Dir: dir, Config: validationConfig(), Runs: writer})
	m.state = StateMainMenu
	m = advance(t, m, resumeDetectMsg{state: git.RepoState{Clean: true}, records: records})
	m.runsCursor = 5 // stale cursor from an earlier visit

	seedQuickDeployRuns(t, writer, runs.Record{
		RunID: "new-run", Ticket: "PROJ-2", Target: "UAT", Status: "Succeeded", CreatedAt: base.Add(time.Hour),
	})

	m = enterRunHistoryFromMenu(t, m)
	if len(m.runs) != 2 {
		t.Fatalf("the fresh load should list both runs, got %d", len(m.runs))
	}
	if m.runs[0].RunID != "new-run" {
		t.Errorf("newest run should be first, got %q", m.runs[0].RunID)
	}
	if m.runsCursor != 0 {
		t.Errorf("cursor should reset to 0, got %d", m.runsCursor)
	}
}

// TestRunHistoryMenu_AllowedWhileInProgress: unlike the standalone entries
// (D1), the history must stay reachable during an in-progress cherry-pick,
// since that is exactly when resume is needed.
func TestRunHistoryMenu_AllowedWhileInProgress(t *testing.T) {
	m := New(Deps{Dir: t.TempDir(), Config: validationConfig(), Runs: runs.NewWriter(t.TempDir())})
	m.state = StateMainMenu
	m.repoState = git.RepoState{InProgress: true, CurrentSHA: "sha-A"}

	m = enterRunHistoryFromMenu(t, m)
	if m.notice != "" {
		t.Errorf("history entry must not be blocked while in progress, got notice %q", m.notice)
	}
	if !m.repoState.InProgress {
		t.Error("a load without a fresh repo state must keep the known in-progress state")
	}
}

// TestRunHistoryMenu_ClearsStandaloneMode: keyMainMenu re-stamps
// standaloneMode on every dispatch; the history entry is a non-standalone
// entry, so a stale "delta"/"validate" flag must never survive into the
// history's Enter-resume/r/x paths. It must also not seed the ticket buffer
// (that auto-suggest belongs to the promote entry only).
func TestRunHistoryMenu_ClearsStandaloneMode(t *testing.T) {
	m := New(Deps{Dir: t.TempDir(), Config: validationConfig(), Runs: runs.NewWriter(t.TempDir())})
	m.state = StateMainMenu
	m.standaloneMode = "validate"
	m.originalBranch = "feature/PROJ-7"

	m = enterRunHistoryFromMenu(t, m)
	if m.standaloneMode != "" {
		t.Errorf("history entry must clear standaloneMode, got %q", m.standaloneMode)
	}
	if m.ticket != "" {
		t.Errorf("history entry must not seed the ticket buffer, got %q", m.ticket)
	}
}

// TestRunHistoryLoaded_ReconcilesStaleRuns applies onResumeDetect's stale
// cherry-pick reconciliation when the load carries a live repo state that
// shows NO pick in progress.
func TestRunHistoryLoaded_ReconcilesStaleRuns(t *testing.T) {
	m := New(Deps{Dir: t.TempDir(), Config: validationConfig()})
	m.state = StateRunHistory
	msg := runHistoryLoadedMsg{
		state:      git.RepoState{Clean: true},
		stateKnown: true,
		records:    []runs.Record{{RunID: "stale", Ticket: "PROJ-1", Phase: "git-conflict"}},
	}
	m = advance(t, m, msg)
	if len(m.runs) != 1 || m.runs[0].Phase != "aborted" {
		t.Fatalf("a stale conflict record should be reconciled to aborted, got %+v", m.runs)
	}
}

// TestRunHistoryLoaded_IgnoredOffHistory: a load that lands after the user
// already left the history must not touch the model.
func TestRunHistoryLoaded_IgnoredOffHistory(t *testing.T) {
	m := New(Deps{Dir: t.TempDir(), Config: validationConfig()})
	m.state = StateMainMenu
	m = advance(t, m, runHistoryLoadedMsg{records: []runs.Record{{RunID: "late"}}})
	if len(m.runs) != 0 {
		t.Errorf("a late load off the history must be a no-op, got %d runs", len(m.runs))
	}
}

// TestRunHistoryLoaded_ErrorIsSurfaced: a failed listing keeps the screen
// usable and shows the error instead of silently rendering nothing.
func TestRunHistoryLoaded_ErrorIsSurfaced(t *testing.T) {
	m := New(Deps{Dir: t.TempDir(), Config: validationConfig()})
	m.state = StateRunHistory
	m = advance(t, m, runHistoryLoadedMsg{err: errors.New("disk on fire")})
	if m.State() != StateRunHistory {
		t.Fatalf("a load error should stay on StateRunHistory, got %v", m.State())
	}
	if v := m.View(); !strings.Contains(v, "disk on fire") {
		t.Errorf("history view should surface the load error:\n%s", v)
	}
}

// TestRunHistoryFooter_ReflectsSelectedRowActions pins the history footer:
// q/esc go BACK to the main menu (not quit), and x/r are advertised only when
// the selected row is actually eligible, so a silent no-op key is never shown.
func TestRunHistoryFooter_ReflectsSelectedRowActions(t *testing.T) {
	clk := &fakeClock{t: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)}
	eligible := runs.Record{
		RunID: "ok", Ticket: "PROJ-1", Target: "UAT", Alias: "UAT_SBX",
		JobID: "0AfOK", Status: "Succeeded", TestLevel: "RunLocalTests",
		Commits: []string{"abc123"}, CreatedAt: clk.t,
	}
	running := runs.Record{
		RunID: "busy", Ticket: "PROJ-2", Target: "UAT", Alias: "UAT_SBX",
		JobID: "0AfBUSY", Status: "InProgress", CreatedAt: clk.t,
	}

	m := New(Deps{Dir: t.TempDir(), Config: quickDeployConfig(false, false), Now: clk.now})
	m.state = StateRunHistory
	m.runs = []runs.Record{eligible, running}

	m.runsCursor = 0
	v := m.View()
	for _, want := range []string{"x quick deploy", "r re-promocionar", "q volver"} {
		if !strings.Contains(v, want) {
			t.Errorf("footer on an eligible row should contain %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "q salir") {
		t.Errorf("q returns to the menu, footer must not say \"q salir\":\n%s", v)
	}

	m.runsCursor = 1
	v = m.View()
	for _, unwanted := range []string{"x quick deploy", "r re-promocionar"} {
		if strings.Contains(v, unwanted) {
			t.Errorf("footer on an ineligible row must not advertise %q:\n%s", unwanted, v)
		}
	}

	m.runs = nil
	if v := m.View(); !strings.Contains(v, "q volver") || strings.Contains(v, "q salir") {
		t.Errorf("empty history footer should say \"q volver\":\n%s", v)
	}
}
