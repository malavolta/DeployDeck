package app

import (
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/runs"
)

// historyModel parks a Model on StateRunHistory holding the given runs.
func historyModel(records []runs.Record) Model {
	m := New(Deps{Dir: "/repo", Config: validationConfig()})
	m.state = StateRunHistory
	m.runs = records
	return m
}

// --- 5.1/5.2: history lists runs newest-first -------------------------------

// TestViewRunHistory_ListsRunsNewestFirst is task 5.1 (RED): the history screen
// lists runs newest-first with ticket/target/status/date; an empty history
// renders no rows and no error.
func TestViewRunHistory_ListsRunsNewestFirst(t *testing.T) {
	newer := time.Date(2026, 7, 2, 10, 14, 0, 0, time.UTC)
	older := time.Date(2026, 7, 1, 9, 5, 0, 0, time.UTC)
	m := historyModel([]runs.Record{
		{RunID: "n", Ticket: "PROJ-9", Target: "UAT", JobID: "0AfNEW", Status: "InProgress", CreatedAt: newer},
		{RunID: "o", Ticket: "PROJ-1", Target: "INT", JobID: "0AfOLD", Status: "Failed", CreatedAt: older},
	})

	v := m.View()
	for _, want := range []string{"PROJ-9", "PROJ-1", "UAT", "INT", "InProgress", "Failed", "2026-07-02", "2026-07-01"} {
		if !strings.Contains(v, want) {
			t.Errorf("history view missing %q:\n%s", want, v)
		}
	}
	// Newest first: PROJ-9's row precedes PROJ-1's row.
	if strings.Index(v, "PROJ-9") > strings.Index(v, "PROJ-1") {
		t.Errorf("history should list newest-first (PROJ-9 before PROJ-1):\n%s", v)
	}

	// Empty history renders no rows and no error.
	empty := historyModel(nil)
	ev := empty.View()
	if ev == "" {
		t.Error("empty history should still render a screen")
	}
	if strings.Contains(ev, "PROJ") {
		t.Errorf("empty history should show no run rows:\n%s", ev)
	}
}

// --- 5.3/5.4: row shows progress reached ------------------------------------

// TestViewRunHistory_RowShowsProgressReached is task 5.3 (RED): a run WITH a
// jobId shows its validation status; a run WITHOUT a job shows the last flow
// step reached (its Phase).
func TestViewRunHistory_RowShowsProgressReached(t *testing.T) {
	t.Run("run without a job shows the last step reached", func(t *testing.T) {
		rec := runs.Record{RunID: "g", Ticket: "PROJ-1", Target: "UAT", Phase: "git-conflict", CreatedAt: time.Now()}
		if got := runProgressLabel(rec); got != "git-conflict" {
			t.Errorf("a jobless run's progress label = %q, want its phase git-conflict", got)
		}
		m := historyModel([]runs.Record{rec})
		if !strings.Contains(m.View(), "git-conflict") {
			t.Errorf("history row for a jobless run should show the last step reached:\n%s", m.View())
		}
	})

	t.Run("run with a job shows its validation status", func(t *testing.T) {
		rec := runs.Record{RunID: "j", Ticket: "PROJ-1", Target: "UAT", JobID: "0Af1", Status: "InProgress", Phase: "validating", CreatedAt: time.Now()}
		if got := runProgressLabel(rec); got != "InProgress" {
			t.Errorf("a jobId run's progress label = %q, want its validation status InProgress", got)
		}
	})
}

// --- 5.5/5.6: detail view on selection --------------------------------------

// TestViewRunHistory_DetailOnSelection is task 5.5 (RED): the selected run's
// detail panel shows its branch, commit count, and delta package path.
func TestViewRunHistory_DetailOnSelection(t *testing.T) {
	m := historyModel([]runs.Record{
		{RunID: "sel", Ticket: "PROJ-1", Target: "UAT", Commits: []string{"a", "b"}, CreatedAt: time.Now()},
	})
	m.runsCursor = 0

	v := m.View()
	if !strings.Contains(v, "deploy/PROJ-1-to-UAT") {
		t.Errorf("detail should show the run's branch:\n%s", v)
	}
	if !strings.Contains(v, "Commits: 2") {
		t.Errorf("detail should show the commit count (2):\n%s", v)
	}
	if !strings.Contains(v, "PROJ-1-to-UAT/package/package.xml") {
		t.Errorf("detail should show the delta package path:\n%s", v)
	}
}

// --- 3.3/3.4: D2 mode-aware identity and render -----------------------------

// TestRunPackagePath_DistinctPerBase is task 3.3 (RED): two standalone-delta
// runs from different bases produce distinct package.xml paths (D2:
// standalone-modes spec "Standalone Runs Are Distinguishable And
// Collision-Free" — the render half; the identity half is
// TestModel_DeltaSourceSelect_TicketIncludesSanitizedBase in
// standalone_delta_test.go).
func TestRunPackagePath_DistinctPerBase(t *testing.T) {
	cfg := validationConfig()
	main := runs.Record{Ticket: "standalone-main", Target: "main"}
	release := runs.Record{Ticket: "standalone-release-1.0", Target: "release/1.0"}

	pMain := runPackagePath(cfg, main)
	pRelease := runPackagePath(cfg, release)

	if pMain == pRelease {
		t.Fatalf("distinct bases must produce distinct package paths, both = %q", pMain)
	}
}

// TestViewRunHistory_ModeAwareRender is task 3.4 (RED): a standalone run's
// row and detail Package line render with a mode-distinct label instead of
// the meaningless blank ticket/target-derived placeholder a Mode=="validate"
// run (empty Ticket/Target) would otherwise produce via runPackagePath (D2:
// standalone-modes spec "Standalone runs render with a mode-distinct
// label"). A Mode=="" run (the historical full-promotion default — same
// fixture as TestViewRunHistory_DetailOnSelection) keeps rendering
// unchanged. (Scope: task 3.7 touches only the row loop and the detail's
// "Package:" line — the "Branch:" line is out of scope and untouched.)
func TestViewRunHistory_ModeAwareRender(t *testing.T) {
	t.Run("Mode=delta renders [delta] <base>", func(t *testing.T) {
		m := historyModel([]runs.Record{
			{RunID: "d", Mode: "delta", Ticket: "standalone-main", Target: "main", CreatedAt: time.Now()},
		})
		m.runsCursor = 0
		v := m.View()
		if !strings.Contains(v, "[delta] main") {
			t.Errorf("history should render a mode-distinct label for a delta run:\n%s", v)
		}
	})

	t.Run("Mode=validate renders [validate] <package basename>", func(t *testing.T) {
		m := historyModel([]runs.Record{
			{RunID: "v", Mode: "validate", ManifestPath: "/repo/.deploydeck/manifest/pkg/package.xml", CreatedAt: time.Now()},
		})
		m.runsCursor = 0
		v := m.View()
		if !strings.Contains(v, "[validate] package.xml") {
			t.Errorf("history should render a mode-distinct label for a validate run:\n%s", v)
		}
		if !strings.Contains(v, "Package: /repo/.deploydeck/manifest/pkg/package.xml") {
			t.Errorf("the detail Package line should show rec.ManifestPath directly, not the blank runPackagePath -to- placeholder:\n%s", v)
		}
	})

	t.Run("Mode=\"\" keeps the existing full-flow render unchanged", func(t *testing.T) {
		m := historyModel([]runs.Record{
			{RunID: "sel", Ticket: "PROJ-1", Target: "UAT", Commits: []string{"a", "b"}, CreatedAt: time.Now()},
		})
		m.runsCursor = 0
		v := m.View()
		if !strings.Contains(v, "PROJ-1") || !strings.Contains(v, "UAT") {
			t.Errorf("Mode=\"\" row should keep showing Ticket/Target unchanged, got:\n%s", v)
		}
		if !strings.Contains(v, "PROJ-1-to-UAT/package/package.xml") {
			t.Errorf("Mode=\"\" detail should keep showing runPackagePath, got:\n%s", v)
		}
	})
}

// --- 5.7/5.8: history keys --------------------------------------------------

// TestKeyRunHistory_Navigation is task 5.7 (RED): ↑/↓ move the cursor, `d`
// toggles the expanded detail, `Enter` on a resumable run resumes it, `Enter`
// on a terminal run is a no-op (stays on history), and `q`/`esc` declines to
// the normal ticket-input flow.
func TestKeyRunHistory_Navigation(t *testing.T) {
	records := []runs.Record{
		{RunID: "0", Ticket: "PROJ-9", Target: "UAT", JobID: "j0", Status: "Succeeded", Phase: "done", CreatedAt: time.Now()},
		{RunID: "1", Ticket: "PROJ-1", Target: "UAT", JobID: "j1", Status: "InProgress", Phase: "validating", CreatedAt: time.Now()},
	}

	t.Run("down then up moves the cursor", func(t *testing.T) {
		m := historyModel(records)
		down := advance(t, m, keyPress("down"))
		if down.runsCursor != 1 {
			t.Fatalf("down should move cursor to 1, got %d", down.runsCursor)
		}
		up := advance(t, down, keyPress("up"))
		if up.runsCursor != 0 {
			t.Fatalf("up should move cursor back to 0, got %d", up.runsCursor)
		}
		// Cursor clamps at the ends.
		if advance(t, m, keyPress("up")).runsCursor != 0 {
			t.Error("up at the top should stay at 0")
		}
		if advance(t, down, keyPress("down")).runsCursor != 1 {
			t.Error("down at the bottom should stay at the last row")
		}
	})

	t.Run("d toggles the expanded detail", func(t *testing.T) {
		m := historyModel(records)
		toggled := advance(t, m, keyPress("d"))
		if !toggled.runDetail {
			t.Fatal("d should turn the detail toggle on")
		}
		if !strings.Contains(toggled.View(), "Fase:") {
			t.Errorf("the expanded detail should surface extra fields (Fase:):\n%s", toggled.View())
		}
		if advance(t, toggled, keyPress("d")).runDetail {
			t.Error("d again should turn the detail toggle off")
		}
	})

	t.Run("enter on a resumable run resumes", func(t *testing.T) {
		clk := &fakeClock{t: time.Unix(1000, 0)}
		m := New(Deps{Dir: "/repo", Config: validationConfig(), SF: reportSF(t, "j1", "UAT_SBX", "InProgress"), Now: clk.now})
		m.state = StateRunHistory
		m.runs = records
		m.runsCursor = 1 // the non-terminal jobId run
		next, cmd := m.Update(keyPress("enter"))
		if next.(Model).State() != StateValidationPolling {
			t.Fatalf("Enter on a resumable jobId run should re-attach polling, got %v", next.(Model).State())
		}
		if cmd == nil {
			t.Error("resuming should fire a command")
		}
	})

	t.Run("enter on a terminal run is a no-op", func(t *testing.T) {
		m := historyModel(records)
		m.runsCursor = 0 // the terminal Succeeded run
		next, cmd := m.Update(keyPress("enter"))
		if next.(Model).State() != StateRunHistory {
			t.Fatalf("Enter on a terminal run must stay on history, got %v", next.(Model).State())
		}
		if cmd != nil {
			t.Error("a terminal run must not fire a resume command")
		}
	})

	t.Run("q and esc decline to the main menu", func(t *testing.T) {
		// HU-018 (design ADR-1 refinement): declining the resume offer returns to
		// the main menu — not StateTicketInput — so the hub stays reachable after
		// a declined resume.
		for _, key := range []string{"q", "esc"} {
			m := historyModel(records)
			if advance(t, m, keyPress(key)).State() != StateMainMenu {
				t.Errorf("%q should decline the offer and proceed to StateMainMenu", key)
			}
		}
	})
}
