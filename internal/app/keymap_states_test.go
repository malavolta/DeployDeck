package app

import (
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/runs"
)

func footerHas(m Model, k string) bool {
	keys, _, _ := footerKeys(m.View())
	return keys[k]
}

func TestKeymap_ConflictContinueFollowsTheGate(t *testing.T) {
	m := fixtureModel(func(m *Model) { m.state, m.continuePending = StateCherryPickConflict, []string{"x"} })
	if footerHas(m, "c") {
		t.Error("footer must hide c while continue is disabled")
	}
	if nm, cmd := press(m, "c"); cmd != nil || nm.state != m.state {
		t.Errorf("c while disabled must not act: cmd=%v state=%v", cmd, nm.state)
	}
	m.continueEnabled, m.continuePending = true, nil
	if !footerHas(m, "c") || !acts(m, "c") {
		t.Error("c must be listed and act once continue is enabled")
	}
}

func TestKeymap_DisabledByModelStateIsNeitherShownNorActed(t *testing.T) {
	ready := New(Deps{Dir: "/repo", Config: testConfig()})
	ready.state, ready.pushPhase, ready.authState = StatePushPreparation, pushReady, github.AuthAuthenticated
	if nm, cmd := press(ready, "a"); footerHas(ready, "a") || cmd != nil || nm.aiPending {
		t.Error("push-ready without AI must neither list nor start a")
	}
	for _, s := range []State{StateSucceeded, StateAborted} {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.state = s
		if nm, cmd := press(m, "d"); footerHas(m, "d") || cmd != nil || nm.cleanupPhase != cleanupIdle {
			t.Errorf("%v without a branch must neither list nor start d", s)
		}
	}
}

func TestKeymap_InFlightFooters(t *testing.T) {
	for _, s := range []State{StateCommitDiscovery, StateBranchCreation, StateCherryPicking} {
		m := New(Deps{})
		m.state = s
		_, text, _ := footerKeys(m.View())
		if !strings.Contains(text, "ctrl+c interrumpe") || footerHas(m, "q") || footerHas(m, "esc") {
			t.Errorf("%v footer %q", s, text)
		}
		if _, cmd := press(m, "ctrl+c"); cmd == nil {
			t.Errorf("%v: ctrl+c must interrupt", s)
		}
	}
	m := fixtureModel(st(StateValidationPolling))
	if !footerHas(m, "c") || !acts(m, "c") {
		t.Error("polling footer must list c cancelar and c must act")
	}
}

// textStates are the states that consume typed text, with their buffers.
func textStates() []struct {
	name string
	set  func(*Model)
	buf  func(Model) string
} {
	del := func(s State) func(*Model) {
		return func(m *Model) {
			m.state, m.cleanupPhase, m.plan.PromotionBranch = s, cleanupStrongConfirm, "deploy/DD-1"
		}
	}
	return []struct {
		name string
		set  func(*Model)
		buf  func(Model) string
	}{
		{"ticket", st(StateTicketInput), func(m Model) string { return m.ticket }},
		{"package path", st(StatePackageSelect), func(m Model) string { return m.packagePath }},
		{"CANCELAR", st(StateCancelConfirm), func(m Model) string { return m.cancelInput }},
		{"DESPLEGAR", func(m *Model) { m.state, m.runs = StateQuickDeploy, []runs.Record{{RunID: "r"}} }, func(m Model) string { return m.quickConfirm }},
		{"BORRAR succeeded", del(StateSucceeded), func(m Model) string { return m.deleteConfirm }},
		{"BORRAR aborted", del(StateAborted), func(m Model) string { return m.deleteConfirm }},
		{"BORRAR cleanup list", func(m *Model) { *m = cleanupListModel(); m.cleanupPhase = cleanupStrongConfirm }, func(m Model) string { return m.deleteConfirm }},
	}
}

func TestKeymap_PrintableKeysReachTheBuffer(t *testing.T) {
	for _, c := range textStates() {
		t.Run(c.name, func(t *testing.T) {
			m := New(Deps{Dir: "/repo", Config: testConfig()})
			c.set(&m)
			m = typeString(m, "z")
			for _, k := range []string{"c", "d", "x", "p", "q"} {
				before := c.buf(m)
				nm, cmd := press(m, k)
				if c.buf(nm) != before+k || cmd != nil || nm.state != m.state {
					t.Errorf("%q: buffer=%q cmd=%v state=%v", k, c.buf(nm), cmd, nm.state)
				}
				m = nm
			}
			_, text, _ := footerKeys(m.View())
			for _, seg := range strings.Split(text, "   ") {
				if k := strings.Fields(seg)[0]; len(k) == 1 {
					t.Errorf("footer %q advertises printable %q", text, k)
				}
			}
		})
	}
}

func TestKeymap_QOnEmptyBufferActsAsEscNeverQuits(t *testing.T) {
	for _, c := range textStates() {
		t.Run(c.name, func(t *testing.T) {
			m := New(Deps{Dir: "/repo", Config: testConfig()})
			c.set(&m)
			q, qcmd := press(m, "q")
			e, _ := press(m, "esc")
			if isQuit(qcmd) || q.state != e.state || q.cleanupPhase != e.cleanupPhase {
				t.Errorf("q must act as esc: q=%v/%v esc=%v/%v quit=%v", q.state, q.cleanupPhase, e.state, e.cleanupPhase, isQuit(qcmd))
			}
		})
	}
}
