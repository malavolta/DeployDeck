package app

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/malavolta/DeployDeck/internal/git"
)

// TestOnSpinnerTick_BumpsFrameWhileInSpinnerState is task 5.2 (RED):
// onSpinnerTick bumps m.spinnerFrame and reschedules (non-nil tea.Cmd) while
// m.state is one of the 5 long-running states; outside those states it is a
// strict no-op (nil cmd, frame unchanged) — mirrors onTick's own
// state-scoped guard (update.go).
func TestOnSpinnerTick_BumpsFrameWhileInSpinnerState(t *testing.T) {
	t.Run("bumps frame and reschedules in a spinner state", func(t *testing.T) {
		m := New(Deps{})
		m.state = StateDeltaGeneration
		m.spinnerFrame = 0

		next, cmd := m.onSpinnerTick()
		nm := next.(Model)
		if nm.spinnerFrame != 1 {
			t.Fatalf("spinnerFrame = %d, want 1", nm.spinnerFrame)
		}
		if cmd == nil {
			t.Fatal("onSpinnerTick should reschedule (non-nil cmd) while still in a spinner state")
		}
	})

	t.Run("no-op outside a spinner state", func(t *testing.T) {
		m := New(Deps{})
		m.state = StateMainMenu
		m.spinnerFrame = 0

		next, cmd := m.onSpinnerTick()
		nm := next.(Model)
		if nm.spinnerFrame != 0 {
			t.Fatalf("spinnerFrame = %d, want unchanged 0 outside a spinner state", nm.spinnerFrame)
		}
		if cmd != nil {
			t.Fatal("onSpinnerTick should return nil cmd outside a spinner state")
		}
	})

	// Reliability fix: onSpinnerTick is the ONLY place the tick loop dies,
	// so it must clear m.spinning the instant the flow leaves the
	// spinner-state set — otherwise a stale spinning=true would wrongly
	// block the NEXT spinner-state entry from ever seeding a fresh loop.
	t.Run("leaving a spinner state clears the spinning single-flight guard", func(t *testing.T) {
		m := New(Deps{})
		m.state = StateMainMenu
		m.spinning = true

		next, _ := m.onSpinnerTick()
		nm := next.(Model)
		if nm.spinning {
			t.Fatal("onSpinnerTick outside a spinner state should reset m.spinning to false")
		}
	})
}

// TestOnBranchCreated_SpinnerSingleFlight_SkipsSecondSeed is the RED test for
// the spinner double-tick-loop reliability fix: keyPlanPreview already seeds
// a spinnerCmd loop entering StateBranchCreation, and onBranchCreated used to
// seed ANOTHER one unconditionally entering StateCherryPicking — two
// concurrent tea.Tick loops (~2x frame speed). With the single-flight guard,
// onBranchCreated must NOT seed a second spinner while one is already
// running (m.spinning == true): the returned command batches only
// cherryPickCmd + tickCmd, not a third spinnerCmd.
func TestOnBranchCreated_SpinnerSingleFlight_SkipsSecondSeed(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: validationConfig()})
	m.state = StateBranchCreation
	m.branchName = "deploy/PROJ-1-to-UAT"
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
	m.spinning = true // a spinner loop is already ticking (seeded on BranchCreation entry)

	next, cmd := m.onBranchCreated(branchCreatedMsg{})
	nm := next.(Model)
	if nm.State() != StateCherryPicking {
		t.Fatalf("expected CherryPicking, got %v (err=%v)", nm.State(), nm.Err())
	}
	if !nm.spinning {
		t.Fatal("spinning should stay true — the already-running loop keeps ticking")
	}
	if cmd == nil {
		t.Fatal("onBranchCreated should still fire cherryPickCmd + tickCmd")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg (cherryPickCmd + tickCmd), got %T", msg)
	}
	if len(batch) != 2 {
		t.Fatalf("tea.BatchMsg has %d cmds, want 2 (cherryPickCmd + tickCmd) — "+
			"a second spinnerCmd must NOT be seeded while m.spinning is already true", len(batch))
	}
}

// TestOnAISuggestDone_Success_SetsTitleAndDescription_NotAccepted is task
// 4.6 (RED): a successful generation sets aiTitle/aiDescription and clears
// aiPending, but does NOT set aiAccepted — a generated suggestion is
// PROPOSED, never auto-accepted (spec: "Explicit Accept Overrides Only The
// PR-Creation Title Source").
func TestOnAISuggestDone_Success_SetsTitleAndDescription_NotAccepted(t *testing.T) {
	m := New(Deps{})
	m.aiPending = true

	next, cmd := m.Update(aiSuggestDoneMsg{title: "PROJ-1 - AI drafted title", description: "AI drafted description"})
	if cmd != nil {
		t.Errorf("onAISuggestDone should return a nil cmd, got non-nil")
	}
	nm := next.(Model)

	if nm.aiPending {
		t.Error("aiPending should be false after the request lands")
	}
	if nm.aiTitle != "PROJ-1 - AI drafted title" {
		t.Errorf("aiTitle = %q, want %q", nm.aiTitle, "PROJ-1 - AI drafted title")
	}
	if nm.aiDescription != "AI drafted description" {
		t.Errorf("aiDescription = %q, want %q", nm.aiDescription, "AI drafted description")
	}
	if nm.aiAccepted {
		t.Error("aiAccepted must stay false — a generated suggestion is proposed, not accepted")
	}
}

// TestOnAISuggestDone_ErrOrEmptyTitle_NoSuggestion is task 4.6 (RED): an
// error and an empty-title result are treated identically — aiPending
// clears, aiErr is recorded, but NO title/description change occurs (spec:
// "Silent Graceful Degradation").
func TestOnAISuggestDone_ErrOrEmptyTitle_NoSuggestion(t *testing.T) {
	wantErr := errors.New("ai: request failed")
	tests := []struct {
		name string
		msg  aiSuggestDoneMsg
	}{
		{name: "request errored", msg: aiSuggestDoneMsg{err: wantErr}},
		{name: "empty title, no error", msg: aiSuggestDoneMsg{title: "", description: "some description"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(Deps{})
			m.aiPending = true

			next, cmd := m.Update(tt.msg)
			if cmd != nil {
				t.Errorf("onAISuggestDone should return a nil cmd, got non-nil")
			}
			nm := next.(Model)

			if nm.aiPending {
				t.Error("aiPending should be false after the request lands, even on failure")
			}
			if nm.aiTitle != "" {
				t.Errorf("aiTitle = %q, want empty (no suggestion surfaced on failure/empty title)", nm.aiTitle)
			}
			if nm.aiAccepted {
				t.Error("aiAccepted must stay false")
			}
		})
	}
}

// TestOnPrepDone_AutoFiresAISuggestion_WhenConfigured is Feature B's RED:
// once the screen settles on pushReady, onPrepDone proactively requests an
// ai-pr-summary suggestion — the user no longer has to press `a` first — but
// never auto-accepts (aiAccepted stays false; only the explicit second 'a'
// press, keyPushPreparation, ever sets it).
func TestOnPrepDone_AutoFiresAISuggestion_WhenConfigured(t *testing.T) {
	m := New(Deps{GenerateSummary: func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		return "PROJ-1 - AI drafted title", "AI drafted description", nil
	}})
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}

	next, cmd := m.Update(prepDoneMsg{})
	nm := next.(Model)

	if nm.pushPhase != pushReady {
		t.Fatalf("onPrepDone should settle pushPhase on pushReady, got %v", nm.pushPhase)
	}
	if !nm.aiPending {
		t.Error("onPrepDone should set aiPending when Deps.GenerateSummary is configured")
	}
	if cmd == nil {
		t.Fatal("onPrepDone should return the auto-fired aiSuggestCmd")
	}
	if nm.aiAccepted {
		t.Error("auto-firing must never set aiAccepted")
	}
}

// TestOnPrepDone_NoAutoFire_WhenGenerateSummaryNil is Feature B's
// nil-degrades companion: with no Deps.GenerateSummary configured, onPrepDone
// never sets aiPending and returns a nil cmd (same nil-degrades convention as
// every other optional Dep in this file).
func TestOnPrepDone_NoAutoFire_WhenGenerateSummaryNil(t *testing.T) {
	m := New(Deps{})
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}

	next, cmd := m.Update(prepDoneMsg{})
	nm := next.(Model)

	if nm.pushPhase != pushReady {
		t.Fatalf("onPrepDone should settle pushPhase on pushReady, got %v", nm.pushPhase)
	}
	if nm.aiPending {
		t.Error("onPrepDone should not set aiPending without Deps.GenerateSummary")
	}
	if cmd != nil {
		t.Error("onPrepDone should return a nil cmd without Deps.GenerateSummary")
	}
}

// TestOnPrepDone_NoAutoFire_WhenAlreadyPendingOrHeld proves the auto-fire
// guard (m.aiTitle == "" && !m.aiPending) prevents a duplicate request: a
// late/duplicate prepDoneMsg landing while a request is already in flight,
// or once a suggestion is already held, must not fire a second one.
func TestOnPrepDone_NoAutoFire_WhenAlreadyPendingOrHeld(t *testing.T) {
	dep := func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		return "PROJ-1 - AI drafted title", "AI drafted description", nil
	}

	t.Run("already pending", func(t *testing.T) {
		m := New(Deps{GenerateSummary: dep})
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
		m.aiPending = true

		_, cmd := m.Update(prepDoneMsg{})
		if cmd != nil {
			t.Error("onPrepDone must not fire a second request while one is already pending")
		}
	})

	t.Run("suggestion already held", func(t *testing.T) {
		m := New(Deps{GenerateSummary: dep})
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
		m.aiTitle = "PROJ-1 - AI drafted title"

		_, cmd := m.Update(prepDoneMsg{})
		if cmd != nil {
			t.Error("onPrepDone must not re-request once a suggestion is already held")
		}
	})
}
