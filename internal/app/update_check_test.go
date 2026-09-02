package app

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/malavolta/DeployDeck/internal/prereq"
)

// TestCheckUpdateCmd_NilDeps_ReturnsNil proves the nil-degrades convention
// (design ADR-2): with no Deps.CheckUpdate injected, checkUpdateCmd is a
// no-op, mirroring runPrereqCmd's nil-NewChecker degrade.
func TestCheckUpdateCmd_NilDeps_ReturnsNil(t *testing.T) {
	m := New(Deps{})
	if cmd := m.checkUpdateCmd(); cmd != nil {
		t.Fatalf("checkUpdateCmd() with nil Deps.CheckUpdate should return nil, got non-nil cmd")
	}
}

// TestCheckUpdateCmd_FedFake_YieldsDoneMsg feeds a fake Deps.CheckUpdate and
// invokes the returned tea.Cmd synchronously (design's test approach, per
// go-testing skill: drive Bubble Tea commands directly), proving the cmd
// wraps the injected func and forwards its result as updateCheckDoneMsg.
func TestCheckUpdateCmd_FedFake_YieldsDoneMsg(t *testing.T) {
	m := New(Deps{CheckUpdate: func(ctx context.Context) (bool, string, error) {
		return true, "9.9.9", nil
	}})

	cmd := m.checkUpdateCmd()
	if cmd == nil {
		t.Fatalf("checkUpdateCmd() with a non-nil Deps.CheckUpdate should return a non-nil cmd")
	}

	msg, ok := cmd().(updateCheckDoneMsg)
	if !ok {
		t.Fatalf("checkUpdateCmd()() = %T, want updateCheckDoneMsg", msg)
	}
	want := updateCheckDoneMsg{hasUpdate: true, latest: "9.9.9", err: nil}
	if msg != want {
		t.Errorf("checkUpdateCmd()() = %+v, want %+v", msg, want)
	}
}

// TestInit_Batches_PrereqAndCheckUpdate proves Init() fires the update check
// concurrently with the existing prereq flow (design's data flow: "Init() =
// tea.Batch(runPrereqCmd(), checkUpdateCmd())"), and that a nil
// Deps.CheckUpdate leaves the pre-existing prereq-only startup unaffected
// (tea.Batch drops nil cmds — bubbletea's compactCmds returns the single
// remaining cmd directly rather than a BatchMsg).
func TestInit_Batches_PrereqAndCheckUpdate(t *testing.T) {
	// fakeNewChecker errors immediately (never reaching a real
	// prereq.Checker.Check call, which would need a wired *git.Service).
	// This keeps the assertions isolated to Init()'s batching shape, not
	// prereq internals — already covered by prereq_test.go. Zero-arg
	// (design.md ADR-5): the composition root resolves the roots, not this
	// closure.
	fakeNewChecker := func() (*prereq.Checker, error) {
		return nil, errStub
	}

	t.Run("both NewChecker and CheckUpdate set batches both commands", func(t *testing.T) {
		m := New(Deps{
			NewChecker: fakeNewChecker,
			CheckUpdate: func(ctx context.Context) (bool, string, error) {
				return false, "", nil
			},
		})

		cmd := m.Init()
		if cmd == nil {
			t.Fatalf("Init() should be non-nil when NewChecker and CheckUpdate are both set")
		}

		msg := cmd()
		batch, ok := msg.(tea.BatchMsg)
		if !ok {
			t.Fatalf("Init()() = %T, want tea.BatchMsg (both prereq and update-check cmds)", msg)
		}
		if len(batch) != 2 {
			t.Errorf("tea.BatchMsg has %d cmds, want 2 (prereq + update-check)", len(batch))
		}
	})

	t.Run("nil CheckUpdate leaves the existing prereq-only flow unaffected", func(t *testing.T) {
		m := New(Deps{NewChecker: fakeNewChecker})

		cmd := m.Init()
		if cmd == nil {
			t.Fatalf("Init() should still be non-nil with only NewChecker set (existing prereq flow unaffected)")
		}
	})
}

// TestOnUpdateCheckDone_HasUpdate_SetsFields proves a genuine hasUpdate
// result records the notice fields the banner (task 3.9/3.10) will read.
func TestOnUpdateCheckDone_HasUpdate_SetsFields(t *testing.T) {
	m := New(Deps{})

	next, cmd := m.Update(updateCheckDoneMsg{hasUpdate: true, latest: "9.9.9"})
	if cmd != nil {
		t.Errorf("onUpdateCheckDone should return a nil cmd, got non-nil")
	}
	nm := next.(Model)
	if !nm.updateAvailable {
		t.Errorf("updateAvailable = false, want true")
	}
	if nm.updateLatest != "9.9.9" {
		t.Errorf("updateLatest = %q, want %q", nm.updateLatest, "9.9.9")
	}
}

// TestOnUpdateCheckDone_ErrOrNoUpdate_NoOp proves ADR-3's silent skip: an
// errored check and a "no newer version" result are treated identically —
// no fields set, no state change, startup never gated by this check.
func TestOnUpdateCheckDone_ErrOrNoUpdate_NoOp(t *testing.T) {
	tests := []struct {
		name string
		msg  updateCheckDoneMsg
	}{
		{name: "check failed", msg: updateCheckDoneMsg{err: errStub}},
		{name: "no newer version available", msg: updateCheckDoneMsg{hasUpdate: false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(Deps{})
			wantState := m.State()

			next, cmd := m.Update(tt.msg)
			if cmd != nil {
				t.Errorf("onUpdateCheckDone should return a nil cmd, got non-nil")
			}
			nm := next.(Model)
			if nm.updateAvailable {
				t.Errorf("updateAvailable = true, want false (silent skip)")
			}
			if nm.State() != wantState {
				t.Errorf("state = %v, want %v (startup flow unaffected)", nm.State(), wantState)
			}
		})
	}
}

// TestView_UpdateBanner_ShownWhenAvailable proves View() prepends the
// notice without displacing the underlying screen (design's data flow:
// "View() = m.updateBanner() + m.viewBody()").
func TestView_UpdateBanner_ShownWhenAvailable(t *testing.T) {
	m := Model{state: StateTicketInput, updateAvailable: true, updateLatest: "9.9.9"}

	out := m.View()
	if !strings.Contains(out, "9.9.9") {
		t.Errorf("View() should contain the latest version %q when updateAvailable, got: %q", "9.9.9", out)
	}
	if !strings.Contains(out, "Ticket o incidencia") {
		t.Errorf("View() should still contain the underlying screen content, got: %q", out)
	}
}

// TestView_NoBanner_WhenUnavailable proves the default (no update known)
// case renders no banner at all.
func TestView_NoBanner_WhenUnavailable(t *testing.T) {
	m := Model{state: StateTicketInput, updateAvailable: false, updateLatest: "9.9.9"}

	out := m.View()
	if strings.Contains(out, "9.9.9") {
		t.Errorf("View() should NOT contain the latest version when !updateAvailable, got: %q", out)
	}
}
