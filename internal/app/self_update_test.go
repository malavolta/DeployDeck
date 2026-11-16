package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

const testReleasesURL = "https://github.com/malavolta/DeployDeck/releases/latest"

// updModel parks a menu-state model that already received an update result.
func updModel(t *testing.T, canUpgrade, hasUpdate bool) Model {
	t.Helper()
	m := New(Deps{Dir: "/repo", Config: testConfig(), CanUpgrade: canUpgrade, ReleasesURL: testReleasesURL})
	m.state = StateMainMenu
	next, _ := m.Update(updateCheckDoneMsg{hasUpdate: hasUpdate, latest: "v1.3.0"})
	return next.(Model)
}

func pressU(m Model) (Model, tea.Cmd) {
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	return next.(Model), cmd
}

func TestSelfUpdate_UOnMainMenuRequestsUpgradeAndQuits(t *testing.T) {
	m, cmd := pressU(updModel(t, true, true))
	if !m.UpgradeRequested() || m.UpdateLatest() != "v1.3.0" || cmd == nil {
		t.Fatalf("requested=%v latest=%q cmd=%v", m.UpgradeRequested(), m.UpdateLatest(), cmd != nil)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("cmd() = %T, want tea.QuitMsg", cmd())
	}
}

func TestSelfUpdate_UOnOtherStateDoesNotRequest(t *testing.T) {
	m := updModel(t, true, true)
	m.state = StateTicketInput
	if m, _ = pressU(m); m.UpgradeRequested() {
		t.Fatal("UpgradeRequested() = true outside the main menu")
	}
}

func TestSelfUpdate_BlockedWhileInProgressThenAllowed(t *testing.T) {
	m := updModel(t, true, true)
	m.repoState.InProgress = true
	m, cmd := pressU(m)
	if m.UpgradeRequested() || cmd != nil {
		t.Fatalf("blocked press: requested=%v cmd=%v", m.UpgradeRequested(), cmd != nil)
	}
	if !strings.Contains(m.View(), "termina la operación en curso antes de actualizar DeployDeck") {
		t.Fatalf("missing in-progress notice in %q", m.View())
	}
	m.repoState.InProgress = false
	if m, cmd = pressU(m); !m.UpgradeRequested() || cmd == nil {
		t.Fatalf("after unblock: requested=%v cmd=%v", m.UpgradeRequested(), cmd != nil)
	}
}

func TestSelfUpdate_UNoOps(t *testing.T) {
	for name, m := range map[string]Model{
		"no update available": updModel(t, true, false),
		"nothing wired":       updModel(t, false, true),
	} {
		t.Run(name, func(t *testing.T) {
			if m, cmd := pressU(m); m.UpgradeRequested() || cmd != nil {
				t.Fatalf("requested=%v cmd=%v, want false/nil", m.UpgradeRequested(), cmd != nil)
			}
		})
	}
}

func TestSelfUpdate_BannerCopy(t *testing.T) {
	other := updModel(t, true, true)
	other.state = StateTicketInput
	tests := []struct {
		name      string
		m         Model
		want, not []string
	}{
		{"main menu upgradable", updModel(t, true, true),
			[]string{"v1.3.0", "Nueva versión de DeployDeck disponible", "pulsa u para actualizar", "u actualizar"}, []string{"A newer DeployDeck"}},
		{"main menu manual", updModel(t, false, true),
			[]string{"v1.3.0", testReleasesURL}, []string{"pulsa u", "u actualizar"}},
		{"other state", other, []string{"v1.3.0"}, []string{"pulsa u"}},
		{"no update", updModel(t, true, false), nil, []string{"Nueva versión", "u actualizar"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := tt.m.View()
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in %q", w, out)
				}
			}
			for _, w := range tt.not {
				if strings.Contains(out, w) {
					t.Errorf("unexpected %q in %q", w, out)
				}
			}
		})
	}
}
