package app

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func TestHelpOverlay_ListsEveryBindingIncludingDisabledAndUnlabelled(t *testing.T) {
	m := historyModel(nil)
	m.runs = richModel().runs
	m, _ = press(m, "?")
	v := m.View()
	if !m.helpOpen || !strings.Contains(v, "Atajos") || !strings.HasSuffix(v, footer("?/Esc cerrar")) {
		t.Fatalf("overlay must open with title and closing footer:\n%s", v)
	}
	for _, b := range m.screenBindings() {
		if !strings.Contains(v, b.Help().Desc) {
			t.Errorf("overlay lacks binding %q", b.Help().Desc)
		}
	}
	if !regexp.MustCompile(`x\s+quick deploy \(no disponible\)`).MatchString(v) {
		t.Errorf("a disabled binding must be listed as unavailable:\n%s", v)
	}
	bare := key.NewBinding(key.WithKeys("z", "y"))
	if got := overlayEntries([]key.Binding{bare}); got[0].Help().Key != "z/y" || !got[0].Enabled() {
		t.Errorf("an unlabelled binding must fall back to its key list, got %q", got[0].Help().Key)
	}
}

func TestHelpOverlay_ClosesAndIsModal(t *testing.T) {
	for _, closer := range []string{"?", "esc"} {
		m := richModel()
		m.state, m.cursor = StateTargetSelection, 1
		base := m.View()
		o, _ := press(m, "?")
		c, cmd := press(o, closer)
		if c.helpOpen || cmd != nil || c.state != StateTargetSelection || c.cursor != 1 || c.View() != base {
			t.Errorf("%s must close the overlay leaving state and cursor unchanged", closer)
		}
	}
	m := fixtureModel(func(m *Model) { m.state = StateCommitSelection })
	o, _ := press(m, "?")
	for _, k := range []string{" ", "enter", "down", "q"} {
		if nm, cmd := press(o, k); cmd != nil || nm.state != o.state || nm.cursor != o.cursor || nm.items[0].Selected || !nm.helpOpen {
			t.Errorf("%q must be inert while the overlay is open", k)
		}
	}
	if _, cmd := o.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil {
		t.Error("ctrl+c must still interrupt")
	}
}

func TestHelpOverlay_QuestionMarkIsTypedInTextStates(t *testing.T) {
	buffers := map[string]func(*Model) *string{
		"ticket":  func(m *Model) *string { m.state = StateTicketInput; return &m.ticket },
		"package": func(m *Model) *string { m.state = StatePackageSelect; return &m.packagePath },
		"cancel":  func(m *Model) *string { m.state = StateCancelConfirm; return &m.cancelInput },
		"quick":   func(m *Model) *string { m.state = StateQuickDeploy; return &m.quickConfirm },
		"delete": func(m *Model) *string {
			m.state, m.cleanupPhase = StateSucceeded, cleanupStrongConfirm
			return &m.deleteConfirm
		},
		"cleanup": func(m *Model) *string {
			if m.state != StateBranchCleanup {
				*m = cleanupListModel()
			}
			m.cleanupPhase = cleanupStrongConfirm
			return &m.deleteConfirm
		},
	}
	for name, pick := range buffers {
		for _, seed := range []string{"", "AB"} {
			m := richModel()
			*pick(&m) = seed
			nm, _ := press(m, "?")
			if nm.helpOpen || *pick(&nm) != seed+"?" {
				t.Errorf("%s with %q: ? must be typed, open=%v", name, seed, nm.helpOpen)
			}
			if strings.Contains(m.View(), "? ayuda") {
				t.Errorf("%s: footer must not advertise ?", name)
			}
		}
	}
}
