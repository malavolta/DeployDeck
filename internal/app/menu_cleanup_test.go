package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/malavolta/DeployDeck/internal/git"
)

// isQuit reports whether cmd is the quit command (Deps.Git is nil in these
// models, so quitCmd degrades to tea.Quit).
func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func press(m Model, k string) (Model, tea.Cmd) {
	next, cmd := m.Update(keyPress(k))
	return next.(Model), cmd
}

func cleanupListModel() Model {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateBranchCleanup
	m.cleanupPhase = cleanupBrowsing
	m.cleanupBranches = []cleanupRow{{DeployBranch: git.DeployBranch{Name: "deploy/DD-7"}}}
	return m
}

func TestMenuCleanup_EntryOrderAndDescription(t *testing.T) {
	entries := visibleMenuEntries(menuEntries)
	n := len(entries)
	if entries[n-2].label != "Historial de runs" || entries[n-1].label != "Limpieza de ramas" {
		t.Fatalf("want ... Historial de runs, Limpieza de ramas; got %q, %q", entries[n-2].label, entries[n-1].label)
	}
	if entries[n-1].target != StateBranchCleanup || entries[n-1].description == "" {
		t.Fatalf("cleanup entry needs target and description: %+v", entries[n-1])
	}
	if !strings.Contains(mainMenuModel(0).View(), "Limpieza de ramas") {
		t.Fatal("menu view must render the cleanup entry")
	}
}

func TestMenuCleanup_EnterOpensCleanupEvenWithTicket(t *testing.T) {
	m := mainMenuModel(len(visibleMenuEntries(menuEntries)) - 1)
	m.ticket = "DD-7"
	nm, cmd := press(m, "enter")
	if nm.state != StateBranchCleanup || nm.cleanupPhase != cleanupLoading || cmd == nil {
		t.Fatalf("state=%v phase=%v cmd=%v", nm.state, nm.cleanupPhase, cmd)
	}
	tm := New(Deps{Dir: "/repo", Config: testConfig()})
	tm.state = StateTicketInput
	nm, cmd = press(tm, "b")
	if nm.state != StateBranchCleanup || cmd == nil {
		t.Fatalf("ticket b must still open cleanup, state=%v", nm.state)
	}
}

func TestMenuCleanup_TicketEscGoesToMenu(t *testing.T) {
	for _, typed := range []string{"", "WEB-1"} {
		m := New(Deps{})
		m.state, m.ticket = StateTicketInput, typed
		for _, k := range []string{"esc", "q"} {
			if typed != "" && k == "q" {
				continue
			}
			nm, cmd := press(m, k)
			if nm.state != StateMainMenu || isQuit(cmd) {
				t.Fatalf("ticket %q + %s: state=%v quit=%v", typed, k, nm.state, isQuit(cmd))
			}
		}
	}
}

func TestMenuCleanup_ListAndResultNavigation(t *testing.T) {
	nm, _ := press(cleanupListModel(), "esc")
	if nm.state != StateMainMenu {
		t.Fatalf("list esc -> %v, want menu", nm.state)
	}
	if _, cmd := press(cleanupListModel(), "q"); !isQuit(cmd) {
		t.Fatal("list q must quit")
	}
}

func TestMenuCleanup_ConfirmViewsBackToList(t *testing.T) {
	cases := []struct {
		phase cleanupPhase
		keys  []string
	}{
		{cleanupConfirm, []string{"esc", "q"}},
		{cleanupPruneConfirm, []string{"esc", "q"}},
		{cleanupStrongConfirm, []string{"esc"}},
	}
	for _, c := range cases {
		for _, k := range c.keys {
			m := cleanupListModel()
			m.cleanupPhase, m.cleanupDeleteTarget = c.phase, "deploy/DD-7"
			nm, cmd := press(m, k)
			if nm.state != StateBranchCleanup || nm.cleanupPhase != cleanupBrowsing || isQuit(cmd) {
				t.Errorf("phase %v + %s: state=%v phase=%v quit=%v", c.phase, k, nm.state, nm.cleanupPhase, isQuit(cmd))
			}
		}
	}
}

func TestMenuCleanup_BorrarPromptAndGate(t *testing.T) {
	m := cleanupListModel()
	m.cleanupPhase, m.cleanupDeleteTarget = cleanupStrongConfirm, "deploy/DD-7"
	if v := m.View(); !strings.Contains(lineWith(v, "BORRAR"), "deploy/DD-7") {
		t.Fatalf("prompt must name branch and word:\n%s", v)
	}
	nm := typeString(m, "nope")
	nm, cmd := press(nm, "enter")
	if cmd != nil || nm.cleanupPhase != cleanupStrongConfirm {
		t.Fatal("wrong word must not delete")
	}
	q, cmd := press(typeString(m, "x"), "q")
	if q.deleteConfirm != "xq" || isQuit(cmd) {
		t.Fatalf("q in a non-empty buffer must append, buffer=%q", q.deleteConfirm)
	}
	ok := typeString(m, "BORRAR")
	if _, cmd := press(ok, "enter"); cmd == nil {
		t.Fatal("exact BORRAR must delete")
	}
}

func lineWith(view, sub string) string {
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, sub) {
			return l
		}
	}
	return ""
}
