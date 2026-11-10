package app

import (
	"testing"

	"github.com/malavolta/DeployDeck/internal/runs"
)

type escQ struct {
	name string
	set  func(*Model)
	esc  any // "quit", "none" or a State
	q    any
}

func st(s State) func(*Model) { return func(m *Model) { m.state = s } }

func escQFixtures() []escQ {
	withBranch := func(s State) func(*Model) {
		return func(m *Model) { m.state, m.plan.PromotionBranch = s, "deploy/DD-1" }
	}
	push := func(p pushPhase) func(*Model) {
		return func(m *Model) { m.state, m.pushPhase = StatePushPreparation, p }
	}
	return []escQ{
		{"prereq", st(StatePrereqCheck), "quit", "quit"},
		{"main menu", st(StateMainMenu), "quit", "quit"},
		{"delta source", st(StateDeltaSourceSelect), StateMainMenu, "quit"},
		{"sandbox", st(StateSandboxSelect), StatePackageSelect, "quit"},
		{"run history", st(StateRunHistory), StateMainMenu, "quit"},
		{"gate blocked", st(StateDeployGateBlocked), StateRunHistory, "quit"},
		{"source confirm", st(StateSourceConfirm), StateTicketInput, "quit"},
		{"selection", st(StateCommitSelection), StateTicketInput, "quit"},
		{"target", st(StateTargetSelection), StateCommitSelection, "quit"},
		{"plan preview", st(StatePlanPreview), StateTargetSelection, "quit"},
		{"queue review", st(StateQueueReview), StatePackageReview, "quit"},
		{"branch collision", st(StateBranchCollision), StatePlanPreview, "quit"},
		{"conflict", st(StateCherryPickConflict), "none", "quit"},
		{"verification", st(StatePickVerification), "none", "quit"},
		{"delta generation", st(StateDeltaGeneration), "none", "quit"},
		{"package review", st(StatePackageReview), "none", "quit"},
		{"validation start", st(StateValidationStart), "none", "quit"},
		{"polling", st(StateValidationPolling), "none", "quit"},
		{"push confirm", push(pushConfirm), StateSucceeded, "quit"},
		{"push ready", push(pushReady), "none", "quit"},
		{"discovery", st(StateCommitDiscovery), "none", "none"},
		{"branch creation", st(StateBranchCreation), "none", "none"},
		{"cherry picking", st(StateCherryPicking), "none", "none"},
		{"succeeded", withBranch(StateSucceeded), "quit", "quit"},
		{"aborted", withBranch(StateAborted), "quit", "quit"},
		{"failed", st(StateFailed), "quit", "quit"},
		{"canceled", st(StateCanceled), "quit", "quit"},
		{"error", st(StateError), "quit", "quit"},
	}
}

func assertOutcome(t *testing.T, before Model, key string, want any) {
	t.Helper()
	nm, cmd := press(before, key)
	switch w := want.(type) {
	case string:
		if w == "quit" && !isQuit(cmd) {
			t.Errorf("%s: want quit, state=%v", key, nm.state)
		}
		if w == "none" && (isQuit(cmd) || nm.state != before.state || nm.pushPhase != before.pushPhase) {
			t.Errorf("%s: want no-op, state=%v quit=%v", key, nm.state, isQuit(cmd))
		}
	case State:
		if nm.state != w || isQuit(cmd) {
			t.Errorf("%s: want back to %v, got %v quit=%v", key, w, nm.state, isQuit(cmd))
		}
	}
}

func TestEscQ_Matrix(t *testing.T) {
	for _, f := range escQFixtures() {
		t.Run(f.name, func(t *testing.T) {
			m := New(Deps{Dir: "/repo", Config: testConfig()})
			f.set(&m)
			assertOutcome(t, m, "esc", f.esc)
			assertOutcome(t, m, "q", f.q)
		})
	}
	// Enter also quits on every terminal screen.
	for _, s := range []State{StateSucceeded, StateAborted, StateFailed, StateCanceled, StateError} {
		m := New(Deps{})
		m.state = s
		assertOutcome(t, m, "enter", "quit")
	}
}

func TestEscQ_TypedStatesNeverQuitOnQ(t *testing.T) {
	cases := []struct {
		name string
		set  func(*Model)
		buf  func(Model) string
		back State
	}{
		{"ticket", func(m *Model) { m.state = StateTicketInput }, func(m Model) string { return m.ticket }, StateMainMenu},
		{"package path", func(m *Model) { m.state = StatePackageSelect }, func(m Model) string { return m.packagePath }, StateMainMenu},
		{"CANCELAR", func(m *Model) { m.state = StateCancelConfirm }, func(m Model) string { return m.cancelInput }, StateValidationPolling},
		{"DESPLEGAR", func(m *Model) { m.state = StateQuickDeploy; m.runs = []runs.Record{{RunID: "r"}} }, func(m Model) string { return m.quickConfirm }, StateRunHistory},
		{"BORRAR", func(m *Model) {
			m.state, m.cleanupPhase = StateBranchCleanup, cleanupStrongConfirm
		}, func(m Model) string { return m.deleteConfirm }, StateBranchCleanup},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := New(Deps{Dir: "/repo", Config: testConfig()})
			c.set(&m)
			typed, cmd := press(typeString(m, "x"), "q")
			if isQuit(cmd) || c.buf(typed) != "xq" {
				t.Fatalf("q with text must append, buffer=%q quit=%v", c.buf(typed), isQuit(cmd))
			}
			empty, cmd := press(m, "q")
			if isQuit(cmd) || empty.state != c.back {
				t.Fatalf("q on empty buffer must go back to %v, got %v quit=%v", c.back, empty.state, isQuit(cmd))
			}
		})
	}
}

func TestEscQ_YesNoStatesCancelOnQ(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state, m.pushPhase = StatePushPreparation, pushPRConfirm
	for _, k := range []string{"esc", "q"} {
		nm, cmd := press(m, k)
		if nm.pushPhase != pushReady || isQuit(cmd) {
			t.Errorf("PR confirm %s: phase=%v quit=%v", k, nm.pushPhase, isQuit(cmd))
		}
	}
	d := New(Deps{Dir: "/repo"})
	d.state, d.cleanupPhase = StateSucceeded, cleanupConfirm
	for _, k := range []string{"esc", "q"} {
		nm, cmd := press(d, k)
		if nm.cleanupPhase != cleanupIdle || isQuit(cmd) {
			t.Errorf("delete confirm %s: phase=%v quit=%v", k, nm.cleanupPhase, isQuit(cmd))
		}
	}
}

func TestEscQ_CtrlCStaysHardInterrupt(t *testing.T) {
	for _, s := range []State{StateCherryPicking, StateTicketInput} {
		m := New(Deps{})
		m.state = s
		m.ticket = "WEB-1"
		nm, cmd := press(m, "ctrl+c")
		if cmd == nil || nm.ticket != "WEB-1" {
			t.Fatalf("ctrl+c in %v must return the interrupt cmd and keep the buffer", s)
		}
	}
}
