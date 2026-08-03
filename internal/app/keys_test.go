package app

import (
	"context"
	"testing"

	"github.com/malavolta/DeployDeck/internal/git"
)

// TestKeyTicket_GuardedQ is task 2.5 (RED): keyTicket has no dedicated "q"
// case today, so "q" always falls into the default rune-append branch — even
// on an EMPTY buffer, where it should instead back out like "esc" (bug fix,
// design D3 "guarded-q"). The non-empty case already passes (documents the
// unchanged append behavior); the empty case is the RED assertion.
func TestKeyTicket_GuardedQ(t *testing.T) {
	t.Run("non-empty buffer appends q", func(t *testing.T) {
		m := New(Deps{})
		m.state = StateTicketInput
		m.ticket = "WEB-1"
		next, _ := m.Update(keyPress("q"))
		nm := next.(Model)
		if nm.ticket != "WEB-1q" {
			t.Fatalf("ticket = %q, want %q", nm.ticket, "WEB-1q")
		}
		if nm.state != StateTicketInput {
			t.Fatalf("state = %v, want StateTicketInput unchanged", nm.state)
		}
	})

	t.Run("empty buffer transitions back like esc", func(t *testing.T) {
		m := New(Deps{})
		m.state = StateTicketInput
		m.ticket = ""
		next, _ := m.Update(keyPress("q"))
		nm := next.(Model)
		if nm.state != StatePrereqCheck {
			t.Fatalf("state = %v, want StatePrereqCheck (mirrors esc)", nm.state)
		}
		if nm.ticket != "" {
			t.Fatalf("ticket = %q, want unchanged empty buffer", nm.ticket)
		}
	})
}

// TestKeyCancelConfirm_GuardedQ is task 2.6 (RED): keyCancelConfirm has no
// dedicated "q" case today, so "q" always falls into the default rune-append
// branch — even on an EMPTY buffer, where it should instead mirror "esc"
// (clear the buffer, back to StateValidationPolling, re-arm the poll loop).
func TestKeyCancelConfirm_GuardedQ(t *testing.T) {
	t.Run("non-empty buffer appends q", func(t *testing.T) {
		m := New(Deps{})
		m.state = StateCancelConfirm
		m.cancelInput = "CANCEL"
		next, _ := m.Update(keyPress("q"))
		nm := next.(Model)
		if nm.cancelInput != "CANCELq" {
			t.Fatalf("cancelInput = %q, want %q", nm.cancelInput, "CANCELq")
		}
		if nm.state != StateCancelConfirm {
			t.Fatalf("state = %v, want StateCancelConfirm unchanged", nm.state)
		}
	})

	t.Run("empty buffer mirrors esc", func(t *testing.T) {
		m := New(Deps{})
		m.state = StateCancelConfirm
		m.cancelInput = ""
		m.notice = "stale"
		next, cmd := m.Update(keyPress("q"))
		nm := next.(Model)
		if nm.state != StateValidationPolling {
			t.Fatalf("state = %v, want StateValidationPolling (mirrors esc)", nm.state)
		}
		if nm.cancelInput != "" {
			t.Fatalf("cancelInput = %q, want cleared", nm.cancelInput)
		}
		if nm.notice != "" {
			t.Fatalf("notice = %q, want cleared", nm.notice)
		}
		// Assert only non-nil (mirrors TestModel_CancelConfirm_EscReturnsToPolling):
		// invoking a tea.Tick-backed cmd() blocks for the real poll interval.
		if cmd == nil {
			t.Fatal("expected the re-arming pollTickCmd, got nil")
		}
	})
}

// TestKeyQuickDeploy_GuardedQ is task 2.7 (RED): keyQuickDeploy's
// `case "q", "esc":` is unconditional today, so "q" ALWAYS backs out — even
// with typed DESPLEGAR text in the buffer, where "q" should instead append
// like every other guarded free-text screen (bug fix). The empty-buffer case
// already passes (documents the unchanged back-out behavior).
func TestKeyQuickDeploy_GuardedQ(t *testing.T) {
	t.Run("non-empty buffer appends q and stays", func(t *testing.T) {
		m := New(Deps{})
		m.state = StateQuickDeploy
		m.quickConfirm = "DESPLEGA"
		next, _ := m.Update(keyPress("q"))
		nm := next.(Model)
		if nm.quickConfirm != "DESPLEGAq" {
			t.Fatalf("quickConfirm = %q, want %q", nm.quickConfirm, "DESPLEGAq")
		}
		if nm.state != StateQuickDeploy {
			t.Fatalf("state = %v, want StateQuickDeploy unchanged", nm.state)
		}
	})

	t.Run("empty buffer still backs out", func(t *testing.T) {
		m := New(Deps{})
		m.state = StateQuickDeploy
		m.quickConfirm = ""
		next, _ := m.Update(keyPress("q"))
		nm := next.(Model)
		if nm.state != StateRunHistory {
			t.Fatalf("state = %v, want StateRunHistory", nm.state)
		}
	})
}

// TestKeyPushPreparation_A_NoSuggestion_RequestsWhenDepPresent is task 4.8
// (RED): the FIRST 'a' press at pushReady, with no suggestion yet and a
// non-nil GenerateSummary dep, sets aiPending and fires aiSuggestCmd (spec:
// "Explicit keypress triggers generation").
func TestKeyPushPreparation_A_NoSuggestion_RequestsWhenDepPresent(t *testing.T) {
	m := New(Deps{GenerateSummary: func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		return "PROJ-1 - AI drafted title", "AI drafted description", nil
	}})
	m.state = StatePushPreparation
	m.pushPhase = pushReady
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: "deploy/PROJ-1-to-UAT"}

	next, cmd := m.Update(keyPress("a"))
	nm := next.(Model)

	if !nm.aiPending {
		t.Fatal("first 'a' with a suggestion dep present should set aiPending")
	}
	if nm.aiAccepted {
		t.Fatal("first 'a' must not accept anything yet")
	}
	if cmd == nil {
		t.Fatal("first 'a' should return the aiSuggestCmd command")
	}
	msg, ok := cmd().(aiSuggestDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want aiSuggestDoneMsg", msg)
	}
}

// TestKeyPushPreparation_A_UnacceptedSuggestion_Accepts is task 4.8 (RED):
// the SECOND 'a' press, once a suggestion has been generated but not yet
// accepted, sets aiAccepted — the distinct accept action (spec: "Accepting
// overrides the PR-creation title source").
func TestKeyPushPreparation_A_UnacceptedSuggestion_Accepts(t *testing.T) {
	m := New(Deps{GenerateSummary: func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		return "should not be called again", "", nil
	}})
	m.state = StatePushPreparation
	m.pushPhase = pushReady
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT", PromotionBranch: "deploy/PROJ-1-to-UAT"}
	m.aiTitle = "PROJ-1 - AI drafted title"
	m.aiDescription = "AI drafted description"

	next, cmd := m.Update(keyPress("a"))
	nm := next.(Model)

	if !nm.aiAccepted {
		t.Fatal("second 'a' on an unaccepted suggestion should accept it")
	}
	if cmd != nil {
		t.Fatal("accepting should not fire another aiSuggestCmd")
	}
}

// TestKeyPushPreparation_A_PendingOrAccepted_Inert is task 4.8 (RED): once
// aiPending or aiAccepted, further 'a' presses are a strict no-op — a
// suggestion can never be requested twice concurrently, nor re-requested
// after acceptance.
func TestKeyPushPreparation_A_PendingOrAccepted_Inert(t *testing.T) {
	dep := func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (string, string, error) {
		return "unexpected call", "", nil
	}

	t.Run("pending is inert", func(t *testing.T) {
		m := New(Deps{GenerateSummary: dep})
		m.state = StatePushPreparation
		m.pushPhase = pushReady
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
		m.aiPending = true

		next, cmd := m.Update(keyPress("a"))
		if cmd != nil {
			t.Fatal("'a' while aiPending should be inert (no new command)")
		}
		if next.(Model).aiAccepted {
			t.Fatal("'a' while aiPending must not accept anything")
		}
	})

	t.Run("accepted is inert", func(t *testing.T) {
		m := New(Deps{GenerateSummary: dep})
		m.state = StatePushPreparation
		m.pushPhase = pushReady
		m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}
		m.aiTitle = "PROJ-1 - AI drafted title"
		m.aiAccepted = true

		next, cmd := m.Update(keyPress("a"))
		if cmd != nil {
			t.Fatal("'a' once already accepted should be inert (no new command)")
		}
		if next.(Model).aiTitle != "PROJ-1 - AI drafted title" {
			t.Fatal("'a' once already accepted must not change the accepted title")
		}
	})
}

// TestKeyPushPreparation_A_NilDep_Inert is task 4.8 (RED): with no
// GenerateSummary dep configured, 'a' is inert (spec: "No ai config leaves
// pushReady unchanged").
func TestKeyPushPreparation_A_NilDep_Inert(t *testing.T) {
	m := New(Deps{})
	m.state = StatePushPreparation
	m.pushPhase = pushReady
	m.plan = git.DeploymentPlan{Ticket: "PROJ-1", TargetBranch: "UAT"}

	next, cmd := m.Update(keyPress("a"))
	if cmd != nil {
		t.Fatal("'a' with no GenerateSummary dep should be inert")
	}
	if next.(Model).aiPending {
		t.Fatal("'a' with no GenerateSummary dep must not set aiPending")
	}
}

// TestConfirmSelection_EmptyGuardNoticeIsSpanish is task 3.1 (RED, design
// D3): the empty-selection guard notice on StateCommitSelection must read in
// Spanish, matching the rest of the TUI's Spanish copy (spec: "Localized
// Empty-Selection Guard Notice Without Cross-Screen Bleed"). Fails against
// the current English string.
func TestConfirmSelection_EmptyGuardNoticeIsSpanish(t *testing.T) {
	m := New(Deps{})
	m.state = StateCommitSelection
	m.ticket = "PROJ-1"
	m.items = nil // zero selection

	next, _ := m.confirmSelection()
	nm := next.(Model)

	want := "selecciona al menos un commit para continuar"
	if nm.notice != want {
		t.Errorf("notice = %q, want %q", nm.notice, want)
	}
	if nm.state != StateCommitSelection {
		t.Errorf("state = %v, want StateCommitSelection unchanged (guard blocks the transition)", nm.state)
	}
}

// TestBackTransitions_ClearStaleNotice is task 3.2 (RED, design D3's "no
// bleed" rule): every audited back/navigation transition that leaves a
// notice-bearing screen must clear m.notice, so a guard notice never renders
// on the destination screen (spec: "Notice does not persist after going
// back" / "does not bleed onto an unrelated screen"). Fails on all 6 today
// (none currently clear m.notice).
func TestBackTransitions_ClearStaleNotice(t *testing.T) {
	tests := []struct {
		name      string
		state     State
		ticket    string
		key       string
		wantState State
	}{
		{name: "keySelection esc -> ticket", state: StateCommitSelection, key: "esc", wantState: StateTicketInput},
		{name: "keyTicket esc -> prereq", state: StateTicketInput, key: "esc", wantState: StatePrereqCheck},
		{name: "keyTicket empty-q -> prereq", state: StateTicketInput, ticket: "", key: "q", wantState: StatePrereqCheck},
		{name: "keyTarget esc -> selection", state: StateTargetSelection, key: "esc", wantState: StateCommitSelection},
		{name: "keyPlanPreview esc -> target", state: StatePlanPreview, key: "esc", wantState: StateTargetSelection},
		{name: "keySourceConfirm esc -> ticket", state: StateSourceConfirm, key: "esc", wantState: StateTicketInput},
		{name: "keyQueueReview esc -> packageReview", state: StateQueueReview, key: "esc", wantState: StatePackageReview},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(Deps{})
			m.state = tt.state
			m.ticket = tt.ticket
			m.notice = "stale notice from a prior screen"

			next, _ := m.Update(keyPress(tt.key))
			nm := next.(Model)

			if nm.state != tt.wantState {
				t.Fatalf("state = %v, want %v", nm.state, tt.wantState)
			}
			if nm.notice != "" {
				t.Errorf("notice = %q, want cleared (\"\") on this back-transition", nm.notice)
			}
		})
	}
}
