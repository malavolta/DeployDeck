package app

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/malavolta/DeployDeck/internal/git"
)

// cmdName is the symbol of cmd's closure. Ticks and batches are told apart by
// it without running them (a tick would block for its interval; closure code
// pointers are not stable across inlined tea.Batch call sites).
func cmdName(cmd tea.Cmd) string {
	if cmd == nil {
		return ""
	}
	return runtime.FuncForPC(reflect.ValueOf(cmd).Pointer()).Name()
}

func isTick(cmd tea.Cmd) bool  { return strings.HasSuffix(cmdName(cmd), "bubbletea.Tick.func1") }
func isBatch(cmd tea.Cmd) bool { return strings.Contains(cmdName(cmd), "Batch.compactCmds") }

// isQuit reports whether cmd is a quit command (tea.Quit, or the guarded
// quitCmd closure that restores the branch before quitting). It identifies the
// command by code identity and never runs it: running a poll tick or a git
// restore just to look at its message is what made the suite slow.
func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	return reflect.ValueOf(cmd).Pointer() == reflect.ValueOf(tea.Quit).Pointer() || strings.Contains(cmdName(cmd), "quitCmd.func")
}

// hasTick reports whether cmd is or batches a tick.
func hasTick(cmd tea.Cmd) bool {
	if isBatch(cmd) {
		for _, c := range cmd().(tea.BatchMsg) {
			if hasTick(c) {
				return true
			}
		}
	}
	return isTick(cmd)
}

// withoutTicks strips ticks from cmd so entry-command assertions keep checking
// the real command when a phase also seeds the spinner.
func withoutTicks(cmd tea.Cmd) tea.Cmd {
	if isTick(cmd) {
		return nil
	}
	if !isBatch(cmd) {
		return cmd
	}
	var rest []tea.Cmd
	for _, c := range cmd().(tea.BatchMsg) {
		if c = withoutTicks(c); c != nil {
			rest = append(rest, c)
		}
	}
	return tea.Batch(rest...)
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

func TestIsQuit_IdentifiesWithoutRunning(t *testing.T) {
	g := New(Deps{Dir: "/repo", Git: pushGit("/repo", "b", "u", true)})
	g.originalBranch = "main"
	if !isQuit(tea.Quit) || !isQuit(g.quitCmd()) || isQuit(nil) {
		t.Error("tea.Quit and the quitCmd closure must count as quit")
	}
	if isQuit(func() tea.Msg { t.Fatal("isQuit must never run the command"); return nil }) || isQuit(spinnerCmd()) {
		t.Error("other commands are not quit")
	}
	if !isTick(spinnerCmd()) || !isTick(tickCmd()) || isTick(tea.Quit) || !isBatch(tea.Batch(tea.Quit, spinnerCmd())) {
		t.Error("tick and batch detection are off")
	}
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
