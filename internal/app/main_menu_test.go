package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// TestVisibleMenuEntries_FiltersUnimplemented is task 2.1 (RED): the
// hide-unimplemented mechanism (AC3 "Unimplemented Modes Hidden From The
// Menu") keeps only the entries flagged implemented:true, preserving their
// declared order. A future not-yet-built mode declared implemented:false is
// filtered out of the surface the menu iterates.
func TestVisibleMenuEntries_FiltersUnimplemented(t *testing.T) {
	entries := []menuEntry{
		{label: "Promocionar ticket", target: StateTicketInput, implemented: true},
		{label: "Modo futuro sin construir", target: StateMainMenu, implemented: false},
		{label: "Generar delta package", target: StateDeltaSourceSelect, implemented: true},
		{label: "Otro modo futuro", target: StateMainMenu, implemented: false},
		{label: "Validar package contra sandbox", target: StatePackageSelect, implemented: true},
	}

	got := visibleMenuEntries(entries)

	if len(got) != 3 {
		t.Fatalf("visibleMenuEntries should keep only the 3 implemented entries, got %d", len(got))
	}
	wantLabels := []string{"Promocionar ticket", "Generar delta package", "Validar package contra sandbox"}
	for i, want := range wantLabels {
		if got[i].label != want {
			t.Errorf("filtered entry %d = %q, want %q (order must be preserved)", i, got[i].label, want)
		}
		if !got[i].implemented {
			t.Errorf("filtered entry %d (%q) should be implemented", i, got[i].label)
		}
	}
}

// mainMenuModel parks a Model on StateMainMenu with the given cursor.
func mainMenuModel(cursor int) Model {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateMainMenu
	m.menuCursor = cursor
	return m
}

// TestModel_MainMenu_CursorAndEnterRouting is task 2.3 (RED): ↑/↓ (and k/j)
// clamp menuCursor into [0, len-1] over the FILTERED entries; Enter routes the
// selected entry — cursor 0 "Promocionar ticket" enters the UNCHANGED full flow
// (StateTicketInput, standaloneMode ""), cursor 1 "Generar delta package" the
// standalone delta flow (StateDeltaSourceSelect, standaloneMode "delta"), cursor
// 2 "Validar package contra sandbox" the standalone validation flow
// (StatePackageSelect, standaloneMode "validate"). AC: "Main Menu As Post-Prereq
// Landing", "Promote entry enters the unchanged full flow".
func TestModel_MainMenu_CursorAndEnterRouting(t *testing.T) {
	t.Run("down/up clamp the cursor within the filtered entries", func(t *testing.T) {
		m := mainMenuModel(0)
		// down walks to the last entry then clamps.
		m = advance(t, m, keyPress("down"))
		if m.menuCursor != 1 {
			t.Fatalf("down should move cursor to 1, got %d", m.menuCursor)
		}
		m = advance(t, m, keyPress("j"))
		if m.menuCursor != 2 {
			t.Fatalf("j should move cursor to 2, got %d", m.menuCursor)
		}
		if advance(t, m, keyPress("down")).menuCursor != 2 {
			t.Error("down at the last entry should clamp at 2")
		}
		// up walks back to the first entry then clamps.
		m = advance(t, m, keyPress("up"))
		if m.menuCursor != 1 {
			t.Fatalf("up should move cursor to 1, got %d", m.menuCursor)
		}
		m = advance(t, m, keyPress("k"))
		if m.menuCursor != 0 {
			t.Fatalf("k should move cursor to 0, got %d", m.menuCursor)
		}
		if advance(t, m, keyPress("up")).menuCursor != 0 {
			t.Error("up at the first entry should clamp at 0")
		}
	})

	t.Run("enter on cursor 0 enters the unchanged full flow", func(t *testing.T) {
		next := advance(t, mainMenuModel(0), keyPress("enter"))
		if next.State() != StateTicketInput {
			t.Fatalf("Promocionar should route to StateTicketInput, got %v", next.State())
		}
		if next.standaloneMode != "" {
			t.Errorf("the full flow must leave standaloneMode empty, got %q", next.standaloneMode)
		}
	})

	t.Run("enter on cursor 1 enters standalone delta", func(t *testing.T) {
		next := advance(t, mainMenuModel(1), keyPress("enter"))
		if next.State() != StateDeltaSourceSelect {
			t.Fatalf("Generar delta should route to StateDeltaSourceSelect, got %v", next.State())
		}
		if next.standaloneMode != "delta" {
			t.Errorf("delta entry should set standaloneMode=delta, got %q", next.standaloneMode)
		}
	})

	t.Run("enter on cursor 2 enters standalone validation", func(t *testing.T) {
		next := advance(t, mainMenuModel(2), keyPress("enter"))
		if next.State() != StatePackageSelect {
			t.Fatalf("Validar package should route to StatePackageSelect, got %v", next.State())
		}
		if next.standaloneMode != "validate" {
			t.Errorf("validate entry should set standaloneMode=validate, got %q", next.standaloneMode)
		}
	})
}

// TestViewMainMenu_RendersEntriesAndCursor is task 2.7 (RED): the menu screen
// renders all three implemented entry labels, marks the selected row with the
// cursor, and shows a quit footer.
func TestViewMainMenu_RendersEntriesAndCursor(t *testing.T) {
	m := mainMenuModel(1) // cursor on "Generar delta package"
	v := m.View()

	for _, want := range []string{"Promocionar ticket", "Generar delta package", "Validar package contra sandbox"} {
		if !strings.Contains(v, want) {
			t.Errorf("main menu view missing entry %q:\n%s", want, v)
		}
	}
	if !strings.Contains(v, "> Generar delta package") {
		t.Errorf("the cursor marker should point at the selected entry (cursor 1):\n%s", v)
	}
	if !strings.Contains(v, "q salir") {
		t.Errorf("main menu view should show a quit footer (q salir):\n%s", v)
	}
}

// TestViewMainMenu_EntriesShowDescriptions is task 3.7 (RED): each main-menu
// entry SHALL show a one-line description alongside its label (spec: "Menu
// Descriptions And Unified Terminology" — "Main menu entries show
// descriptions").
func TestViewMainMenu_EntriesShowDescriptions(t *testing.T) {
	m := mainMenuModel(0)
	v := m.viewMainMenu()

	for i, e := range visibleMenuEntries(menuEntries) {
		if e.description == "" {
			t.Fatalf("menuEntries[%d] (%q) has no description populated", i, e.label)
		}
		if !strings.Contains(v, e.description) {
			t.Errorf("main menu view missing description %q for entry %q:\n%s", e.description, e.label, v)
		}
	}
}

// TestOnResumeDetect_GuardUsesMainMenu is task 2.13's loud guard regression
// (BLOCKER-2 batch A): after the HU-018 landing moved onPrereqDone to
// StateMainMenu, the async resumeDetectMsg now lands while the model sits on
// StateMainMenu. If onResumeDetect's "did the user already advance?" guard
// still keyed off StateTicketInput, it would treat this legitimate landing as
// "advanced" and NO-OP — silently killing HU-013 resume-detection. Seeding
// StateMainMenu + a resumable record must still route to StateRunHistory
// pre-selected on the resumable run, proving the guard keys off StateMainMenu.
func TestOnResumeDetect_GuardUsesMainMenu(t *testing.T) {
	m := New(Deps{Dir: t.TempDir(), Config: validationConfig(), Runs: runs.NewWriter(t.TempDir())})
	m.state = StateMainMenu
	state := git.RepoState{InProgress: true, CurrentSHA: "sha-A", SequencerRemaining: 2}
	records := []runs.Record{
		{RunID: "old", Ticket: "PROJ-9", Status: "Succeeded", Phase: "done"},
		{RunID: "live", Ticket: "PROJ-1", Commits: []string{"sha-A", "sha-B"}, PickTotal: 2, Phase: "git-conflict"},
	}
	next, _ := m.Update(resumeDetectMsg{state: state, records: records})
	nm := next.(Model)
	if nm.State() != StateRunHistory {
		t.Fatalf("resume-detection landing on StateMainMenu must still offer resume via StateRunHistory (guard keys off StateMainMenu, HU-013 preserved), got %v", nm.State())
	}
	if len(nm.runs) != 2 {
		t.Fatalf("history should hold all runs, got %d", len(nm.runs))
	}
	if nm.runs[nm.runsCursor].RunID != "live" {
		t.Errorf("history should pre-select the resumable run, got cursor on %q", nm.runs[nm.runsCursor].RunID)
	}
}

// TestKeyMainMenu_BlocksStandaloneEntryWhileInProgress is tasks 2.1-2.3
// (RED): D1's in-progress entry guard (standalone-modes spec: "Standalone
// Entry Blocked While A Git Operation Is In Progress"). Both standalone
// entries (delta, validate) are blocked with an actionable notice and fire
// no command while `m.repoState.InProgress` is true; with no operation in
// progress, entry proceeds normally (regression); the Promote entry is
// unaffected either way.
func TestKeyMainMenu_BlocksStandaloneEntryWhileInProgress(t *testing.T) {
	t.Run("delta entry blocked while a git operation is in progress", func(t *testing.T) {
		m := mainMenuModel(1)
		m.repoState.InProgress = true

		next, cmd := m.keyMainMenu(keyPress("enter"))
		nm := next.(Model)

		if nm.State() != StateMainMenu {
			t.Fatalf("blocked delta entry should stay on StateMainMenu, got %v", nm.State())
		}
		if nm.standaloneMode != "" {
			t.Errorf("blocked delta entry must not set standaloneMode, got %q", nm.standaloneMode)
		}
		if nm.notice == "" {
			t.Error("blocked delta entry should surface an actionable notice")
		}
		if cmd != nil {
			t.Error("blocked delta entry must not fire any command")
		}
	})

	t.Run("validate entry blocked while a git operation is in progress", func(t *testing.T) {
		m := mainMenuModel(2)
		m.repoState.InProgress = true

		next, cmd := m.keyMainMenu(keyPress("enter"))
		nm := next.(Model)

		if nm.State() != StateMainMenu {
			t.Fatalf("blocked validate entry should stay on StateMainMenu, got %v", nm.State())
		}
		if nm.standaloneMode != "" {
			t.Errorf("blocked validate entry must not set standaloneMode, got %q", nm.standaloneMode)
		}
		if nm.notice == "" {
			t.Error("blocked validate entry should surface an actionable notice")
		}
		if cmd != nil {
			t.Error("blocked validate entry must not fire any command")
		}
	})

	t.Run("delta entry proceeds normally with no operation in progress", func(t *testing.T) {
		m := mainMenuModel(1)
		m.repoState.InProgress = false

		next, cmd := m.keyMainMenu(keyPress("enter"))
		nm := next.(Model)

		if nm.State() != StateDeltaSourceSelect {
			t.Fatalf("delta entry should reach StateDeltaSourceSelect, got %v", nm.State())
		}
		if nm.standaloneMode != "delta" {
			t.Errorf("delta entry should set standaloneMode=delta, got %q", nm.standaloneMode)
		}
		if cmd == nil {
			t.Error("delta entry should fire standaloneBranchesCmd")
		}
	})

	t.Run("validate entry proceeds normally with no operation in progress", func(t *testing.T) {
		m := mainMenuModel(2)
		m.repoState.InProgress = false

		next, cmd := m.keyMainMenu(keyPress("enter"))
		nm := next.(Model)

		if nm.State() != StatePackageSelect {
			t.Fatalf("validate entry should reach StatePackageSelect, got %v", nm.State())
		}
		if nm.standaloneMode != "validate" {
			t.Errorf("validate entry should set standaloneMode=validate, got %q", nm.standaloneMode)
		}
		_ = cmd
	})

	t.Run("Promote entry unaffected by in-progress state", func(t *testing.T) {
		m := mainMenuModel(0)
		m.repoState.InProgress = true

		next, _ := m.keyMainMenu(keyPress("enter"))
		nm := next.(Model)

		if nm.State() != StateTicketInput {
			t.Fatalf("Promote entry should still reach StateTicketInput while in-progress, got %v", nm.State())
		}
		if nm.standaloneMode != "" {
			t.Errorf("the full flow must leave standaloneMode empty, got %q", nm.standaloneMode)
		}
	})
}

// TestModel_MainMenu_QEscQuits is task 2.5 (RED): q and esc on StateMainMenu
// quit the app (the menu is a top-level landing, not a nested screen with a
// back target). Each yields a command that ultimately returns tea.QuitMsg.
func TestModel_MainMenu_QEscQuits(t *testing.T) {
	for _, key := range []string{"q", "esc"} {
		_, cmd := mainMenuModel(0).Update(keyPress(key))
		if cmd == nil {
			t.Fatalf("%q on StateMainMenu should return a quit command", key)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%q on StateMainMenu should ultimately quit (tea.QuitMsg), got %T", key, cmd())
		}
	}
}
